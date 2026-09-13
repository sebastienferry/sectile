package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/handlers"
	"tasks/internal/models"
	"tasks/internal/runner"
	"tasks/internal/terminal"

	"github.com/gorilla/websocket"
)

// agentDaemon runs the local TaskFlow agent that connects outward to a remote
// TaskFlow server and executes workflow steps locally inside Git worktrees.
type agentDaemon struct {
	queueSequence    uint64
	restartRequested bool
	shuttingDown     bool
	restartAgent     context.CancelFunc
	desktopToken     string
	desktopInfo      string
	runsMu           sync.Mutex
	runs             map[string]*controlledRun
	serverURL        string
	token            string
	projectID        string
	deviceID         string
	terminalApp      string
	terminalExplicit bool
	agentPort        int
	agentURL         string
	httpServer       *http.Server
	conn             *websocket.Conn
	connMu           sync.Mutex
	terminalMgr      *terminal.Manager
	repoRoot         string
	prepareMu        sync.Mutex
	done             chan struct{}
}

// detectDefaultTerminal detects installed terminal apps on macOS (Ghostty, iTerm, Terminal.app)
func detectDefaultTerminal() string {
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/Applications/Ghostty.app"); err == nil {
			return "ghostty"
		}
		if _, err := os.Stat("/Applications/iTerm.app"); err == nil {
			return "iterm"
		}
		return "terminal"
	}
	return "pty"
}

// runAgentCommand is the entrypoint for "taskflow agent". It parses flags,
// connects to the remote server, and enters the main event loop.
func runAgentCommand(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	serverURL := fs.String("url", "", "Remote TaskFlow server URL (e.g. https://taskflow.example.com)")
	token := fs.String("token", "", "Authentication token for the remote server")
	projectID := fs.String("project", "all", "Project primary key, or all for multi-project operation")
	deviceID := fs.String("device", "", "Device identifier (defaults to hostname)")
	terminalApp := fs.String("terminal", "", "Deprecated compatibility option; executions use agent-owned consoles")
	repoRoot := fs.String("repo", "", "Local repository root (defaults to current Git checkout)")

	fs.Bool("desktop", false, "Deprecated compatibility flag; local consoles are always available")
	desktopInfo := fs.String("desktop-info", "", "Private local connection file (default: ~/.taskflow/agent-connection.json)")
	listProjects := fs.Bool("list-projects", false, "List server projects and exit")
	_ = fs.Parse(args)

	if *serverURL == "" {
		if envURL := os.Getenv("TASKFLOW_REMOTE_URL"); envURL != "" {
			*serverURL = envURL
		} else {
			fmt.Fprintln(os.Stderr, "Error: --url is required (or set TASKFLOW_REMOTE_URL)")
			fs.Usage()
			os.Exit(1)
		}
	}

	if *token == "" {
		if envToken := os.Getenv("TASKFLOW_AGENT_TOKEN"); envToken != "" {
			*token = envToken
		} else {
			fmt.Fprintln(os.Stderr, "Error: --token is required (or set TASKFLOW_AGENT_TOKEN)")
			fs.Usage()
			os.Exit(1)
		}
	}

	if *deviceID == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "unknown"
		}
		*deviceID = hostname
	}

	termExplicit := false
	termChoice := strings.TrimSpace(*terminalApp)
	if termChoice != "" {
		termExplicit = true
	} else {
		termChoice = os.Getenv("TASKFLOW_TERMINAL")
	}
	if termChoice == "" {
		termChoice = detectDefaultTerminal()
	}

	daemon := &agentDaemon{
		desktopInfo: *desktopInfo, desktopToken: os.Getenv("TASKFLOW_DESKTOP_TOKEN"),
		serverURL:        strings.TrimRight(*serverURL, "/"),
		token:            *token,
		projectID:        *projectID,
		deviceID:         *deviceID,
		terminalApp:      termChoice,
		terminalExplicit: termExplicit,
		terminalMgr:      terminal.NewManager(),
		repoRoot:         *repoRoot,
		done:             make(chan struct{}),
	}

	if *listProjects {
		projects, err := daemon.discoverProjects(context.Background())
		if err != nil {
			log.Printf("[Agent] Project discovery failed: %v", err)
			return
		}
		for _, p := range projects.Projects {
			fmt.Printf("%s\t%s\t%s\n", p.ID, p.Name, p.GitRemoteURL)
		}
		return
	}

	if daemon.desktopToken == "" {
		daemon.desktopToken = rand.Text()
	}
	if daemon.desktopInfo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Printf("[Agent] Cannot locate local configuration: %v", err)
			return
		}
		daemon.desktopInfo = filepath.Join(home, ".taskflow", "agent-connection.json")
	}
	if localAgentAvailable(daemon.desktopInfo) {
		log.Printf("[Agent] An agent is already available through %s; connect the companion to it or stop it first", daemon.desktopInfo)
		return
	}
	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemon.restartAgent = cancel
	// Run after console and gateway cleanup, preserving the original arguments.
	defer func() {
		if !daemon.restartRequested {
			return
		}
		binary, err := os.Executable()
		if err != nil {
			log.Printf("[Agent] Restart failed: %v", err)
			return
		}
		child := exec.Command(binary, os.Args[1:]...)
		child.Env = os.Environ()
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			log.Printf("[Agent] Restart failed: %v", err)
			return
		}
		_ = child.Process.Release()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[Agent] Shutting down...")
		cancel()
		close(daemon.done)
	}()

	log.Printf("🚀 TaskFlow Agent starting (server=%s, project=%s, device=%s)", daemon.serverURL, daemon.projectID, daemon.deviceID)

	// Start local agent HTTP reverse proxy gateway
	if err := daemon.startLocalProxy(ctx); err != nil {
		log.Printf("[Agent] Cannot bootstrap MCP without the local gateway: %v", err)
		return
	}
	if err := daemon.writeDesktopInfo(); err != nil {
		log.Printf("Desktop connection: %v", err)
		return
	}
	defer func() {
		if daemon.httpServer != nil {
			_ = daemon.httpServer.Shutdown(context.Background())
		}
	}()

	defer func() {
		for _, session := range daemon.terminalMgr.ListSessions() {
			_ = daemon.terminalMgr.CloseSession(session.ID)
		}
	}()
	daemon.connectLoop(ctx)
}

// connectLoop establishes and maintains the WebSocket connection to the remote
// server with exponential backoff on disconnection.
func (d *agentDaemon) connectLoop(ctx context.Context) {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := d.connect(ctx)
		if err != nil {
			attempt++
			backoff := time.Duration(math.Min(float64(time.Second)*math.Pow(2, float64(attempt)), float64(60*time.Second)))
			log.Printf("[Agent] Connection lost (attempt %d): %v. Reconnecting in %s...", attempt, err, backoff)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
		} else {
			attempt = 0
		}
	}
}

// startLocalProxy starts an embedded HTTP reverse proxy on 127.0.0.1 (default port 8091 or dynamic)
// so that local skills, scripts, and tools can seamlessly interact with the TaskFlow API through
// the local agent without needing to know the remote server's URL or auth tokens.
func (d *agentDaemon) startLocalProxy(ctx context.Context) error {
	var ln net.Listener
	var err error
	address := "127.0.0.1:8091"
	ln, err = net.Listen("tcp", address)
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("failed to start local agent proxy listener: %w", err)
		}
	}

	addr := ln.Addr().(*net.TCPAddr)
	d.agentPort = addr.Port
	d.agentURL = fmt.Sprintf("http://127.0.0.1:%d", d.agentPort)
	log.Printf("🔌 [Agent Proxy] Local agent HTTP gateway listening on %s", d.agentURL)

	mux := http.NewServeMux()

	upstream, err := url.Parse(d.serverURL)
	if err != nil || (upstream.Scheme != "http" && upstream.Scheme != "https") || upstream.Host == "" {
		_ = ln.Close()
		return fmt.Errorf("invalid upstream server URL")
	}
	proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(upstream)
		pr.Out.Header.Set("Authorization", "Bearer "+d.token)
	}}
	forward := func(w http.ResponseWriter, r *http.Request) {
		// Reject browser requests before attaching the daemon's credential.
		if r.Header.Get("Origin") != "" {
			http.Error(w, "Browser origins are not allowed", http.StatusForbidden)
			return
		}
		if r.Host != fmt.Sprintf("127.0.0.1:%d", d.agentPort) && r.Host != fmt.Sprintf("localhost:%d", d.agentPort) {
			http.Error(w, "Invalid gateway host", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, r)
	}
	mux.HandleFunc("/api/", forward)
	mux.HandleFunc("/mcp", forward)
	mux.HandleFunc("/control/runs/", d.handleRunControl)
	mux.HandleFunc("/desktop/", d.desktopHandler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"ok","agent":"taskflow-local","device":%q,"server":%q}`, d.deviceID, d.serverURL)))
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	d.httpServer = server

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[Agent Proxy] Server error: %v", err)
		}
	}()

	return nil
}

// connect establishes a single WebSocket connection and runs the message loop
// until the connection is lost or the context is cancelled.
func (d *agentDaemon) connect(ctx context.Context) error {
	if d.projectID == "all" {
		projects, err := d.discoverProjects(ctx)
		if err != nil {
			return fmt.Errorf("project discovery: %w", err)
		}
		for _, p := range projects.Projects {
			_, _, err := d.localProjectRoot(ctx, agentconfig.Config{ProjectID: p.ID, GitRemoteURL: p.GitRemoteURL})
			if err != nil {
				log.Printf("[Agent] Project %s (%s) requires a local mapping: %v", p.Name, p.ID, err)
			} else {
				log.Printf("[Agent] Local project available: %s (%s)", p.Name, p.ID)
			}
		}
	}
	if d.projectID != "" && d.projectID != "default" && d.projectID != "all" {
		config, err := d.fetchConfig(ctx, d.projectID, "")
		if err != nil {
			return fmt.Errorf("configuration sync: %w", err)
		}
		if err := d.syncLocalProject(ctx, config); err != nil {
			return err
		}
	}
	wsURL, err := d.buildWSURL()
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+d.token)

	log.Printf("[Agent] Connecting to %s...", wsURL)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	d.connMu.Lock()
	d.conn = conn
	d.connMu.Unlock()

	defer func() {
		d.connMu.Lock()
		d.conn = nil
		d.connMu.Unlock()
		_ = conn.Close()
	}()

	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	log.Printf("[Agent] Connected to remote server")
	fmt.Printf("\n✅ [Agent] Connecté avec succès au serveur TaskFlow (%s)\n", d.serverURL)
	fmt.Printf("   Projet: [%s] | Machine: [%s]\n", d.projectID, d.deviceID)
	fmt.Printf("   Prêt ! Les compétences déclenchées sur l'interface web s'exécuteront ici.\n\n")

	// Start heartbeat sender.
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go d.heartbeatLoop(heartbeatCtx, conn)

	// Message read loop.
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		_, msgData, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		var msg handlers.AgentMessage
		if err := json.Unmarshal(msgData, &msg); err != nil {
			log.Printf("[Agent] Malformed message from server: %v", err)
			continue
		}

		d.handleMessage(ctx, conn, msg)
	}
}

// buildWSURL converts the HTTP server URL to a WebSocket URL with query params.
func (d *agentDaemon) buildWSURL() (string, error) {
	u, err := url.Parse(d.serverURL)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}

	u.Path = "/ws/agent-connect"
	q := u.Query()
	q.Set("projectId", d.projectID)
	q.Set("deviceId", d.deviceID)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// heartbeatLoop sends periodic heartbeats to keep the connection alive and
// detect stale connections.
func (d *agentDaemon) heartbeatLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msg := handlers.AgentMessage{
				Type: "heartbeat",
			}
			d.connMu.Lock()
			err := conn.WriteJSON(msg)
			d.connMu.Unlock()
			if err != nil {
				log.Printf("[Agent] Heartbeat send failed: %v", err)
				return
			}
		}
	}
}

// handleMessage processes a single message received from the remote server.
func (d *agentDaemon) handleMessage(ctx context.Context, conn *websocket.Conn, msg handlers.AgentMessage) {
	switch msg.Type {
	case "heartbeat":
		// Server heartbeat response; nothing to do.
		return

	case "dispatch_step":
		go d.handleDispatchStep(ctx, conn, msg)

	case "pty_input":
		// Forward keyboard input to the local PTY session.
		var payload struct {
			SessionID string `json:"sessionId"`
			Data      string `json:"data"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Agent] Invalid pty_input payload: %v", err)
			return
		}
		if err := d.terminalMgr.SendInput(payload.SessionID, payload.Data); err != nil {
			log.Printf("[Agent] Failed to send PTY input: %v", err)
		}

	case "pty_resize":
		// Resize is handled within the terminal session by the existing
		// WsMessage protocol when a browser connects. For remote relay,
		// the resize command would be forwarded to the session.
		log.Printf("[Agent] Received pty_resize for task %s (not yet wired)", msg.TaskID)

	default:
		log.Printf("[Agent] Unknown message type: %s", msg.Type)
	}
}

// findRepoRoot finds the repository root containing .tasks and all worktrees
func findRepoRoot(startDir string) string {
	// 1. Try git rev-parse --git-common-dir (works inside any git worktree)
	cmd := exec.Command("git", "-C", startDir, "rev-parse", "--git-common-dir")
	if out, err := cmd.Output(); err == nil {
		gitCommon := strings.TrimSpace(string(out))
		if gitCommon != "" {
			if !filepath.IsAbs(gitCommon) {
				gitCommon = filepath.Join(startDir, gitCommon)
			}
			if realGitDir, err := filepath.Abs(gitCommon); err == nil {
				candidate := filepath.Dir(realGitDir)
				if fi, err := os.Stat(filepath.Join(candidate, ".tasks")); err == nil && fi.IsDir() {
					return candidate
				}
				return candidate
			}
		}
	}

	// 2. Walk upwards looking for .tasks directory
	dir := startDir
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".tasks")); err == nil && fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 3. Walk upwards looking for real .git directory (not a worktree file)
	dir = startDir
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return startDir
}

// handleDispatchStep executes a workflow step locally inside a Git worktree.
func (d *agentDaemon) handleDispatchStep(ctx context.Context, conn *websocket.Conn, msg handlers.AgentMessage) {
	var payload agentconfig.Dispatch
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Agent] Invalid dispatch_step payload: %v", err)
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "Invalid payload")
		return
	}

	if payload.SchemaVersion != 0 && payload.SchemaVersion != agentconfig.Version {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "Unsupported dispatch schemaVersion")
		return
	}

	if payload.Action == "cancel_run" {
		d.cancelRun(ctx, conn, msg, payload)
		return
	}
	log.Printf("🚀 [Agent] Received job dispatch for task %s (id=%s): action=%s skill=%s", payload.TaskKey, msg.TaskID, payload.Action, payload.SkillID)
	fmt.Printf("\n⚡ ========================================================\n")
	fmt.Printf("🚀 [Agent] Received job dispatch for task %s\n", payload.TaskKey)
	fmt.Printf("   Action: %s | Skill: %s\n", payload.Action, payload.SkillID)
	fmt.Printf("========================================================\n\n")

	d.sendStatus(conn, msg.MsgID, msg.TaskID, "running", fmt.Sprintf("Executing %s", payload.Action))

	if msg.TaskID != "" && payload.TaskID != "" && msg.TaskID != payload.TaskID {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "Conflicting task primary keys")
		return
	}
	taskRef := msg.TaskID
	if taskRef == "" {
		taskRef = payload.TaskID
	}
	if taskRef == "" {
		taskRef = payload.TaskKey
	}
	if payload.RunID == "" {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "A run ID is required for supervised execution")
		return
	}
	queueConfig, err := d.fetchConfig(ctx, "", taskRef)
	if err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}
	if payload.ProjectID != "" && payload.ProjectID != queueConfig.ProjectID {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "Dispatch project does not match task")
		return
	}
	run, err := d.admitProjectRun(ctx, taskRef, payload, queueConfig)
	if err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}
	// Admission is acknowledged promptly; process completion remains MCP-owned.
	d.sendStatus(conn, msg.MsgID, msg.TaskID, "completed", "Execution accepted into the local queue")
	launched := false
	defer func() {
		if !launched {
			d.runsMu.Lock()
			status := "failed"
			if run.canceled {
				status = "canceled"
			}
			run.desktop.Status = status
			run.once.Do(func() { close(run.exited) })
			d.runsMu.Unlock()
			_ = d.finishDesktopRun(context.Background(), taskRef, payload.RunID, status)
		}
	}()
	if err := d.awaitRunSlot(ctx, run); err != nil {
		return
	}
	config, workDir, branch, task, err := d.prepareDispatch(ctx, taskRef, run.isolated)
	if err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}
	if payload.ProjectID != "" && payload.ProjectID != config.ProjectID {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "Dispatch project does not match task")
		return
	}
	payload.ProjectID = config.ProjectID
	payload.SkillID = models.NormalizeSkillID(payload.SkillID)
	if payload.SkillID == "" && models.NormalizeSkillID(payload.Action) == "adjust" {
		payload.SkillID = "adjust"
	}
	if payload.SkillID == "adjust" {
		pr, verifyErr := runner.NewRunner().BranchPullRequest(workDir, branch)
		if verifyErr != nil {
			d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", verifyErr.Error())
			return
		}
		var task models.Task
		if verifyErr = d.readAPI(ctx, "/api/tasks/"+url.PathEscape(taskRef), &task); verifyErr != nil {
			d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", verifyErr.Error())
			return
		}
		if task.PrURL != nil && *task.PrURL != "" && *task.PrURL != pr.URL {
			d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "recorded PR does not match task branch")
			return
		}
		if task.PrURL == nil || *task.PrURL == "" {
			raw, _ := json.Marshal(map[string]string{"prUrl": pr.URL})
			req, err := http.NewRequestWithContext(ctx, http.MethodPatch, d.serverURL+"/api/tasks/"+url.PathEscape(taskRef), strings.NewReader(string(raw)))
			if err != nil {
				d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := agentHTTPClient(d.token).Do(req)
			if err != nil {
				d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
				return
			}
			resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "could not persist existing PR identity")
				return
			}
		}
		payload.Prompt += "\nExisting PR identity: " + pr.URL + ". Update this same PR; never create or replace it."
	}
	if payload.SkillID == "specify" || payload.SkillID == "implement" {
		payload.Prompt += "\nPreserve accepted artifacts and code on retry. If this is PR recovery, retain the attained task stage and complete the configured creation owner checks without advancing to reviewed."
	}

	if payload.RunID != "" {
		payload.Prompt += fmt.Sprintf("\nRemote execution runId: %s. Reuse this ID with taskflow_start_run and finish it using taskflow_finish_run when the entire skill ends.", payload.RunID)
	}
	fullLine, err := dispatchCommand(config, taskRef, payload.SkillID, payload.Action, payload.Prompt, payload.Command, agentCommandContext{Task: task, Branch: branch, Directory: workDir, Tracker: config.IssueTracker, Repo: config.GithubRepo})
	if err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}

	if payload.RunID != "" {
		fullLine, err = d.wrapRun(taskRef, payload.RunID, fullLine)
		if err != nil {
			d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
			return
		}
	}
	// Create or reuse a PTY session for this task.
	sessionID := "task-" + config.ProjectID + "-" + taskRef
	if payload.RunID != "" {
		sessionID = payload.RunID
	}

	envVars := map[string]string{
		"TASKFLOW_TASK_KEY":      payload.TaskKey,
		"TASKFLOW_TASK_BRANCH":   branch,
		"TASKFLOW_TASK_WORKTREE": workDir,
		"TASKFLOW_TASK_ID":       taskRef,
		"TASKFLOW_RUN_ID":        payload.RunID,
		"TASKFLOW_REMOTE_MODE":   "true",
		"TASKFLOW_AGENT_URL":     d.agentURL,
		"TASKFLOW_SERVER_URL":    d.serverURL,
		"TASKFLOW_AGENT_TOKEN":   d.token,
	}
	if payload.ProjectID != "" {
		envVars["TASKFLOW_PROJECT_ID"] = payload.ProjectID
	}

	if payload.RunID != "" {
		if _, err := d.terminalMgr.GetOrCreateSession(sessionID, workDir, envVars); err != nil {
			d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
			return
		}
		d.runsMu.Lock()
		if run := d.runs[payload.RunID]; run != nil {
			run.desktop = desktopRun{CreatedAt: run.desktop.CreatedAt, Prompt: run.desktop.Prompt, ID: payload.RunID, TaskID: taskRef, TaskKey: payload.TaskKey, ProjectID: config.ProjectID, Skill: payload.SkillID, SessionID: sessionID, Directory: workDir, Branch: branch, Status: "running"}
		}
		d.runsMu.Unlock()
	}
	// The agent owns consoles independently of any attached companion.
	if err := d.runInPty(sessionID, workDir, envVars, fullLine); err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}
	launched = true
	d.sendStatus(conn, msg.MsgID, msg.TaskID, "completed", fmt.Sprintf("Step %s launched in local PTY", payload.Action))
}

// runInPty starts or reuses an embedded PTY session and injects the command line.
func (d *agentDaemon) runInPty(sessionID, workDir string, envVars map[string]string, fullLine string) error {
	sess, err := d.terminalMgr.GetOrCreateSession(sessionID, workDir, envVars)
	if err != nil {
		log.Printf("[Agent] Failed to create PTY session: %v", err)
		return err
	}

	sess.AddOutputListener(func(chunk []byte) {
		_, _ = os.Stdout.Write(chunk)
	})

	time.Sleep(350 * time.Millisecond)
	log.Printf("⚡ [Agent] Launching skill command in local PTY terminal: %s (workdir: %s)", fullLine, workDir)
	startedAt := time.Now().UTC()
	if err := d.terminalMgr.SendInput(sessionID, fullLine+"\n"); err != nil {
		return err
	}
	// Controlled executions use their run ID as the session ID.
	d.runsMu.Lock()
	if run := d.runs[sessionID]; run != nil && run.desktop.StartedAt.IsZero() {
		run.desktop.StartedAt = startedAt
	}
	d.runsMu.Unlock()
	return nil
}

// sendStatus sends a step_status message back to the remote server.
func (d *agentDaemon) sendStatus(conn *websocket.Conn, msgID, taskID, status, summary string) {
	if status == "failed" {
		log.Printf("[Agent] Task %s failed: %s", taskID, summary)
	}
	payload, _ := json.Marshal(map[string]string{
		"status":  status,
		"summary": summary,
	})

	msg := handlers.AgentMessage{
		MsgID:   msgID,
		Type:    "step_status",
		TaskID:  taskID,
		Payload: payload,
	}

	d.connMu.Lock()
	_ = conn.WriteJSON(msg)
	d.connMu.Unlock()
}

func (d *agentDaemon) dispatchTerminal(config agentconfig.Config, override string) string {
	if d.terminalExplicit {
		return d.terminalApp
	}
	if override != "" {
		return override
	}
	if config.ExternalTerminalCommand != "" {
		return config.ExternalTerminalCommand
	}
	if d.terminalApp != "" {
		return d.terminalApp
	}
	return detectDefaultTerminal()
}
