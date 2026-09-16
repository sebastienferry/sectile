package agent

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
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
	"tasks/internal/agenthttp"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/runner"
	"tasks/internal/terminal"

	"github.com/gorilla/websocket"
)

const (
	// agentHeartbeatInterval is how often the agent sends its own application
	// heartbeat. It deliberately differs from the server read timeout of 45s
	// (handlers.defaultAgentReadTimeout): at one keepalive every 10s from each
	// side, four consecutive keepalives must be lost before the server drops
	// the connection. The previous 30s sat exactly on that timeout, so whenever
	// the pong path hiccupped the heartbeat arrived on the deadline and whether
	// the connection survived was a coin flip.
	agentHeartbeatInterval = 10 * time.Second
	// agentPongWriteTimeout is how long the pong sender waits for the
	// connection write mutex. It is generous on purpose: gorilla's default ping
	// handler gives up after one second and, because the resulting error is
	// declared temporary, discards the pong without a word. Waiting instead of
	// giving up is safe here because the pong is written off the read loop.
	agentPongWriteTimeout = 20 * time.Second
)

// agentDaemon runs the local Sectile agent that connects outward to a remote
// Sectile server and executes workflow steps locally inside Git worktrees.
type agentDaemon struct {
	operationMu sync.Mutex
	operations  map[string]context.CancelFunc
	// queue owns every live execution and the process lifecycle flags that
	// gate admission, behind its own mutex. Reach that state only via d.queue.
	queue        runQueue
	restartAgent context.CancelFunc
	// loopback is the private HTTP surface the Electron companion and the
	// agent's own subprocesses talk to, and the credentials that gate it.
	loopback         loopbackServer
	serverURL        string
	token            string
	projectID        string
	deviceID         string
	terminalApp      string
	terminalExplicit bool
	conn             *websocket.Conn
	connMu           sync.Mutex
	terminalMgr      *terminal.Manager
	repoRoot         string
	prepareMu        sync.Mutex
	done             chan struct{}
	contract         contractState
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

func resolveServerURL(flagURL string) string {
	if flagURL != "" {
		return flagURL
	}
	if envURL := os.Getenv("REMOTE_URL"); envURL != "" {
		return envURL
	}
	return ""
}

// validLoopbackRequest accepts a local caller that presents this session's
// secret. Comparison is constant time: the gateway answers unauthenticated
// callers, so a timing oracle would be reachable by any local process.
func (d *agentDaemon) validLoopbackRequest(r *http.Request) bool {
	if d.loopback.token == "" {
		return false
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	presented := strings.TrimPrefix(header, "Bearer ")
	return subtle.ConstantTimeCompare([]byte(presented), []byte(d.loopback.token)) == 1
}

// runningUnderTest reports whether this process is a test binary. Only the
// testing package registers this flag.
func runningUnderTest() bool {
	return flag.Lookup("test.v") != nil
}

// runAgentCommand is the entrypoint for "sectile-agent". It parses flags,
// connects to the remote server, and enters the main event loop.
// Run starts the workstation daemon and blocks until it stops.
func Run(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	serverURL := fs.String("url", "", "Remote Sectile server URL (e.g. https://sectile.example.com)")
	token := fs.String("token", "", "Authentication token for the remote server")
	projectID := fs.String("project", "all", "Project primary key, or all for multi-project operation")
	deviceID := fs.String("device", "", "Device identifier (defaults to hostname)")
	terminalApp := fs.String("terminal", "", "Deprecated compatibility option; executions use agent-owned consoles")
	repoRoot := fs.String("repo", "", "Local repository root (defaults to current Git checkout)")

	fs.Bool("desktop", false, "Deprecated compatibility flag; local consoles are always available")
	desktopInfo := fs.String("desktop-info", "", "Private local connection file (default: ~/.taskflow/agent-connection.json)")
	listProjects := fs.Bool("list-projects", false, "List server projects and exit")
	echoConsoles := fs.Bool("echo-consoles", false, "Mirror console output on this terminal (debugging; consoles are readable from the desktop)")
	_ = fs.Parse(args)

	resolvedURL := resolveServerURL(*serverURL)
	if resolvedURL == "" {
		fmt.Fprintln(os.Stderr, "Error: --url is required (or set REMOTE_URL)")
		fs.Usage()
		os.Exit(1)
	}
	*serverURL = resolvedURL

	if *token == "" {
		if envToken := os.Getenv("TOKEN"); envToken != "" {
			*token = envToken
		} else {
			fmt.Fprintln(os.Stderr, "Error: --token is required (or set TOKEN)")
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
		termChoice = os.Getenv("SECTILE_TERMINAL")
	}
	if termChoice == "" {
		termChoice = detectDefaultTerminal()
	}

	daemon := &agentDaemon{
		loopback: loopbackServer{
			desktopInfo:  *desktopInfo,
			desktopToken: os.Getenv("SECTILE_DESKTOP_TOKEN"),
			echoConsoles: *echoConsoles,
		},
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

	if daemon.loopback.desktopToken == "" {
		daemon.loopback.desktopToken = rand.Text()
	}
	daemon.loopback.token = rand.Text()
	if daemon.loopback.desktopInfo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Printf("[Agent] Cannot locate local configuration: %v", err)
			return
		}
		daemon.loopback.desktopInfo = filepath.Join(home, ".taskflow", "agent-connection.json")
	}
	if localAgentAvailable(daemon.loopback.desktopInfo) {
		log.Printf("[Agent] An agent is already available through %s; connect the companion to it or stop it first", daemon.loopback.desktopInfo)
		return
	}
	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemon.restartAgent = cancel
	// Run after console and gateway cleanup, preserving the original arguments.
	defer func() {
		if !daemon.queue.restartRequested {
			return
		}
		// Under `go test` the executable is the test binary and the arguments
		// are its own flags: restarting would detach a second test binary that
		// outlives the run and keeps holding the gateway port.
		if runningUnderTest() {
			log.Println("[Agent] Restart skipped: running under a test binary")
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

	log.Printf("🚀 Sectile Agent starting (server=%s, project=%s, device=%s)", daemon.serverURL, daemon.projectID, daemon.deviceID)

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
		if daemon.loopback.server != nil {
			_ = daemon.loopback.server.Shutdown(context.Background())
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
			if agentconfig.IsMismatch(err) {
				// Retrying is still right, since updating and restarting the
				// server is what clears this, but calling it a lost connection
				// sends the reader to the network. noteContract has already
				// spelled out the cause, so the retry line only has to say the
				// server has not been updated yet.
				log.Printf("[Agent] Server still does not serve agent contract v%d (attempt %d). Retrying in %s...", agentconfig.Version, attempt, backoff)
			} else {
				log.Printf("[Agent] Connection lost (attempt %d): %v. Reconnecting in %s...", attempt, err, backoff)
			}

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
// so that local skills, scripts, and tools can seamlessly interact with the Sectile API through
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
	d.loopback.port = addr.Port
	d.loopback.url = fmt.Sprintf("http://127.0.0.1:%d", d.loopback.port)
	log.Printf("🔌 [Agent Proxy] Local agent HTTP gateway listening on %s", d.loopback.url)

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
		// Loopback alone is not an authorization: every local process can
		// reach this port. Require the session secret before lending the
		// user's identity to the caller.
		if !d.validLoopbackRequest(r) {
			http.Error(w, "Valid agent session token required", http.StatusUnauthorized)
			return
		}
		if r.Host != fmt.Sprintf("127.0.0.1:%d", d.loopback.port) && r.Host != fmt.Sprintf("localhost:%d", d.loopback.port) {
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
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"ok","agent":"sectile-local","device":%q,"server":%q}`, d.deviceID, d.serverURL)))
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	d.loopback.server = server

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
	fmt.Printf("\n✅ [Agent] Connecté avec succès au serveur Sectile (%s)\n", d.serverURL)
	fmt.Printf("   Projet: [%s] | Machine: [%s]\n", d.projectID, d.deviceID)
	fmt.Printf("   Prêt ! Les compétences déclenchées sur l'interface web s'exécuteront ici.\n\n")

	// Start heartbeat sender.
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go d.heartbeatLoop(heartbeatCtx, conn)

	installKeepalive(heartbeatCtx, conn)

	// Message read loop.
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		_, msgData, err := conn.ReadMessage()
		if err != nil {
			logDisconnect(err)
			return fmt.Errorf("read error: %w", err)
		}

		var msg agentprotocol.Message
		if err := json.Unmarshal(msgData, &msg); err != nil {
			log.Printf("[Agent] Malformed message from server: %v", err)
			continue
		}

		messageCtx := ctx
		if strings.HasPrefix(msg.Type, "workspace_") {
			messageCtx = heartbeatCtx
		}
		d.handleMessage(messageCtx, conn, msg)
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
	ticker := time.NewTicker(agentHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msg := agentprotocol.Message{
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

// installKeepalive answers the server pings off the read loop. gorilla's default
// ping handler writes the pong itself with a one-second budget, and the write
// timeout it gets back is declared temporary, so the handler reports success and
// the pong is lost. Every operation result the agent writes holds the same
// connection write mutex, so a busy agent silently stops answering and the
// server drops it. Queueing the pong keeps the read loop free and lets the
// sender wait for the mutex instead of abandoning the reply.
func installKeepalive(ctx context.Context, conn *websocket.Conn) {
	pongs := make(chan []byte, 1)
	conn.SetPingHandler(func(payload string) error {
		select {
		case pongs <- []byte(payload):
		default:
			// A pong is already queued. Pongs are not cumulative: the queued
			// one answers this ping too.
		}
		return nil
	})
	go pongLoop(ctx, conn, pongs)
}

// pongLoop answers the server pings queued by the connection's ping handler.
// It is the only writer of pong frames, and it runs for the life of the
// connection so a reply is never dropped because the read loop had to move on.
func pongLoop(ctx context.Context, conn *websocket.Conn, pongs <-chan []byte) {
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-pongs:
			err := conn.WriteControl(websocket.PongMessage, payload, time.Now().Add(agentPongWriteTimeout))
			if err == nil || errors.Is(err, websocket.ErrCloseSent) {
				continue
			}
			// This is the failure that produced minutes of unexplained
			// flapping with no trace anywhere: say it out loud.
			log.Printf("[Agent] Could not answer the server keepalive after %s: %v. The server will drop this connection if it stays unanswered.", agentPongWriteTimeout, err)
			return
		}
	}
}

// logDisconnect reports a lost connection in terms a user reading the agent log
// can act on. A server that hung up on purpose sends a close code and a reason;
// without this the log only ever showed "close 1006 (abnormal closure)", which
// names neither.
func logDisconnect(err error) {
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) && closeErr.Text != "" {
		log.Printf("[Agent] Disconnected by the server: code %d, %s", closeErr.Code, closeErr.Text)
		fmt.Printf("\n⚠️  [Agent] Déconnecté par le serveur (code %d) : %s\n\n", closeErr.Code, closeErr.Text)
		return
	}
	log.Printf("[Agent] Connection closed without a reason from the server: %v", err)
}

// handleMessage processes a single message received from the remote server.
func (d *agentDaemon) handleMessage(ctx context.Context, conn *websocket.Conn, msg agentprotocol.Message) {
	switch msg.Type {
	case "workspace_cancel":
		d.operationMu.Lock()
		cancel := d.operations[msg.MsgID]
		d.operationMu.Unlock()
		if cancel != nil {
			cancel()
		}
	case "workspace_request":
		d.startOperation(ctx, conn, msg)
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

	case "pull_tasks":
		d.handlePullTasks(ctx, conn, msg)

	default:
		log.Printf("[Agent] Unknown message type: %s", msg.Type)
	}
}

// handlePullTasks returns all currently queued or running executions.
func (d *agentDaemon) handlePullTasks(ctx context.Context, conn *websocket.Conn, msg agentprotocol.Message) {
	d.queue.mu.Lock()
	tasks := make([]agentprotocol.RunningTask, 0)
	for key, run := range d.queue.runs {
		entry := run.desktop
		status := entry.Status
		select {
		case <-run.exited:
			continue
		default:
		}
		if status == "queued" || status == "running" {
			tasks = append(tasks, agentprotocol.RunningTask{
				ID:        key,
				TaskID:    entry.TaskID,
				TaskKey:   entry.TaskKey,
				ProjectID: entry.ProjectID,
				Skill:     entry.Skill,
				Status:    status,
				CreatedAt: entry.CreatedAt,
				StartedAt: entry.StartedAt,
				Branch:    entry.Branch,
				Directory: entry.Directory,
			})
		}
	}
	d.queue.mu.Unlock()

	raw, err := json.Marshal(tasks)
	if err != nil {
		log.Printf("[Agent] Failed to marshal running tasks: %v", err)
		return
	}

	resp := agentprotocol.Message{
		MsgID:   msg.MsgID,
		Type:    "running_tasks",
		Payload: raw,
	}

	d.connMu.Lock()
	_ = conn.WriteJSON(resp)
	d.connMu.Unlock()
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
func (d *agentDaemon) handleDispatchStep(ctx context.Context, conn *websocket.Conn, msg agentprotocol.Message) {
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
	// launchFailure carries why the console never started. Without it the run
	// closed with "Local console process exited", which is false when nothing
	// ever ran, and left the real cause only in the agent's terminal.
	var launchFailure error
	defer func() {
		if !launched {
			d.queue.mu.Lock()
			status := "failed"
			if run.canceled {
				status = "canceled"
			}
			run.desktop.Status = status
			run.once.Do(func() { close(run.exited) })
			d.queue.mu.Unlock()
			note := ""
			if launchFailure != nil {
				note = "Execution never started: " + launchFailure.Error()
			}
			_ = d.finishDesktopRun(context.Background(), taskRef, payload.RunID, status, note)
		}
	}()
	if err := d.awaitRunSlot(ctx, run); err != nil {
		return
	}
	config, workDir, branch, task, err := d.prepareDispatch(ctx, taskRef, run.isolated)
	if err != nil {
		launchFailure = err
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
			resp, err := agenthttp.Client(d.token).Do(req)
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
		payload.Prompt += fmt.Sprintf("\nRemote execution runId: %s. Reuse this ID with start_run and finish it using finish_run when the entire skill ends.", payload.RunID)
	}
	payload.Mode = liveSessionMode(payload.SkillID, payload.Action, payload.Mode)
	fullLine, err := dispatchCommand(config, taskRef, payload.SkillID, payload.Action, payload.Prompt, payload.Command, payload.Mode, agentCommandContext{Task: task, Branch: branch, Directory: workDir, Tracker: config.IssueTracker, Repo: config.GithubRepo})
	if err != nil {
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
		return
	}

	autonomous := models.NormalizeSkillMode(payload.Mode) == models.SkillModeAutonomous
	if payload.RunID != "" && !autonomous {
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
		"SECTILE_TASK_KEY":      payload.TaskKey,
		"SECTILE_TASK_BRANCH":   branch,
		"SECTILE_TASK_WORKTREE": workDir,
		"SECTILE_TASK_ID":       taskRef,
		"SECTILE_RUN_ID":        payload.RunID,
		"SECTILE_REMOTE_MODE":   "true",
		"SECTILE_AGENT_URL":     d.loopback.url,
		"SECTILE_SERVER_URL":    d.serverURL,
		"SECTILE_AGENT_TOKEN":   d.loopback.token,
	}
	if payload.ProjectID != "" {
		envVars["SECTILE_PROJECT_ID"] = payload.ProjectID
	}

	// An autonomous run forks here, before any terminal exists: no PTY session,
	// no foreground process group, no window. The desktop still lists the run and
	// shows what the CLI printed, but read-only: the output is captured from the
	// process pipes and posted onto the run activity.
	if autonomous {
		if err := d.startHeadlessRun(taskRef, payload, config, workDir, branch, envVars, fullLine); err != nil {
			d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
			return
		}
		launched = true
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "completed", fmt.Sprintf("Step %s launched headless", payload.Action))
		return
	}

	if payload.RunID != "" {
		if _, err := d.terminalMgr.GetOrCreateSession(sessionID, workDir, envVars); err != nil {
			d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", err.Error())
			return
		}
		d.queue.read(payload.RunID, func(run *controlledRun) {
			run.desktop = desktopRun{CreatedAt: run.desktop.CreatedAt, Prompt: run.desktop.Prompt, ID: payload.RunID, TaskID: taskRef, TaskKey: payload.TaskKey, ProjectID: config.ProjectID, Skill: payload.SkillID, SessionID: sessionID, Directory: workDir, Branch: branch, Status: "running"}
		})
	}
	// The agent owns consoles independently of any attached companion.
	if err := d.runInPty(sessionID, workDir, envVars, fullLine); err != nil {
		launchFailure = err
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

	// The console output already reaches the desktop over the WebSocket and is
	// kept in the session history. Echoing it here as well buries the agent's
	// own messages under whatever the model writes, which makes the terminal
	// the agent runs in unusable exactly when something needs diagnosing.
	if d.loopback.echoConsoles {
		sess.AddOutputListener(func(chunk []byte) {
			_, _ = os.Stdout.Write(chunk)
		})
	}

	time.Sleep(350 * time.Millisecond)
	// The full line carries the prompt, which can run to thousands of
	// characters. What identifies a launch is the session and where it runs.
	log.Printf("⚡ [Agent] Launching skill command in local PTY terminal (session: %s, workdir: %s)", sessionID, workDir)
	startedAt := time.Now().UTC()
	if err := d.terminalMgr.SendInput(sessionID, fullLine+"\n"); err != nil {
		return err
	}
	// Controlled executions use their run ID as the session ID.
	d.queue.read(sessionID, func(run *controlledRun) {
		if run.desktop.StartedAt.IsZero() {
			run.desktop.StartedAt = startedAt
		}
	})
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

	msg := agentprotocol.Message{
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
