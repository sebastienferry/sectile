package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
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

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/runner"
	"tasks/internal/terminal"

	"github.com/gorilla/websocket"
)

// agentDaemon runs the local TaskFlow agent that connects outward to a remote
// TaskFlow server and executes workflow steps locally inside Git worktrees.
type agentDaemon struct {
	serverURL        string
	token            string
	projectID        string
	deviceID         string
	terminalApp      string
	terminalExplicit bool
	conn             *websocket.Conn
	connMu           sync.Mutex
	terminalMgr      *terminal.Manager
	database         *db.DB
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
	projectID := fs.String("project", "default", "Project ID to register with")
	deviceID := fs.String("device", "", "Device identifier (defaults to hostname)")
	terminalApp := fs.String("terminal", "", "Terminal application to launch for interactive jobs (ghostty, iterm, terminal, warp, pty)")
	dbPath := fs.String("db", "", "Local database path (optional, for task metadata)")

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

	// Optional local database for task metadata caching.
	var database *db.DB
	if *dbPath != "" {
		var err error
		database, err = db.NewDB(*dbPath)
		if err != nil {
			log.Printf("[Agent] Warning: could not open local database: %v", err)
		} else {
			defer database.Close()
		}
	}

	daemon := &agentDaemon{
		serverURL:        strings.TrimRight(*serverURL, "/"),
		token:            *token,
		projectID:        *projectID,
		deviceID:         *deviceID,
		terminalApp:      termChoice,
		terminalExplicit: termExplicit,
		terminalMgr:      terminal.NewManager(),
		database:         database,
		done:             make(chan struct{}),
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[Agent] Shutting down...")
		cancel()
		close(daemon.done)
	}()

	log.Printf("🚀 TaskFlow Agent starting (server=%s, project=%s, device=%s)", daemon.serverURL, daemon.projectID, daemon.deviceID)

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

// connect establishes a single WebSocket connection and runs the message loop
// until the connection is lost or the context is cancelled.
func (d *agentDaemon) connect(ctx context.Context) error {
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

// readProjectTerminalConfig checks for terminal preference in .taskflow/config.json
func readProjectTerminalConfig(dir string) string {
	configPath := filepath.Join(dir, ".taskflow", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var cfg struct {
		Terminal                string `json:"terminal"`
		TerminalApp             string `json:"terminalApp"`
		ExternalTerminalCommand string `json:"externalTerminalCommand"`
	}
	if err := json.Unmarshal(data, &cfg); err == nil {
		if cfg.Terminal != "" {
			return cfg.Terminal
		}
		if cfg.TerminalApp != "" {
			return cfg.TerminalApp
		}
		if cfg.ExternalTerminalCommand != "" {
			return cfg.ExternalTerminalCommand
		}
	}
	return ""
}

// resolveTaskWorktreeDir looks for the worktree directory under .tasks/worktrees/
func resolveTaskWorktreeDir(root, taskKey string) string {
	cleanKey := strings.TrimSpace(taskKey)
	candidates := []string{
		filepath.Join(root, ".tasks", "worktrees", cleanKey),
	}
	if !strings.HasPrefix(cleanKey, "#") {
		candidates = append(candidates, filepath.Join(root, ".tasks", "worktrees", "#"+cleanKey))
	}
	if strings.HasPrefix(cleanKey, "gh-") {
		num := strings.TrimPrefix(cleanKey, "gh-")
		candidates = append(candidates, filepath.Join(root, ".tasks", "worktrees", "#"+num))
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return root
}

// handleDispatchStep executes a workflow step locally inside a Git worktree.
func (d *agentDaemon) handleDispatchStep(ctx context.Context, conn *websocket.Conn, msg handlers.AgentMessage) {
	var payload struct {
		TaskKey                 string `json:"taskKey"`
		TaskID                  string `json:"taskId"`
		SkillID                 string `json:"skillId"`
		Action                  string `json:"action"` // clarify, specify, code, etc.
		WorkDir                 string `json:"workDir"`
		ProjectID               string `json:"projectId"`
		Provider                string `json:"provider"`
		Prompt                  string `json:"prompt"`
		ExternalTerminalCommand string `json:"externalTerminalCommand"`
		TTYMode                 string `json:"ttyMode"`
		AICommandTemplate       string `json:"aiCommandTemplate"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Agent] Invalid dispatch_step payload: %v", err)
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "Invalid payload")
		return
	}

	log.Printf("🚀 [Agent] Received job dispatch for task %s (id=%s): action=%s skill=%s", payload.TaskKey, msg.TaskID, payload.Action, payload.SkillID)
	fmt.Printf("\n⚡ ========================================================\n")
	fmt.Printf("🚀 [Agent] Received job dispatch for task %s\n", payload.TaskKey)
	fmt.Printf("   Action: %s | Skill: %s\n", payload.Action, payload.SkillID)
	fmt.Printf("========================================================\n\n")

	d.sendStatus(conn, msg.MsgID, msg.TaskID, "running", fmt.Sprintf("Executing %s", payload.Action))

	// Resolve target worktree directory
	cwd, _ := os.Getwd()
	root := findRepoRoot(cwd)
	workDir := payload.WorkDir
	if workDir == "" || workDir == "." {
		workDir = resolveTaskWorktreeDir(root, payload.TaskKey)
	}

	// Determine skill command name
	skillCmd := "/" + payload.Action
	switch strings.ToLower(payload.SkillID) {
	case "clarify", "clarify-issue", "clarify_issue":
		skillCmd = "/clarify-issue"
	case "specify", "specify-issue", "specify_issue":
		skillCmd = "/specify-issue"
	case "code", "code-issue", "code_issue", "implement":
		skillCmd = "/code-issue"
	case "create_pr", "create-pr", "createpr":
		skillCmd = "/create-pr"
	case "handoff", "handoff-issue", "handoff_issue":
		skillCmd = "/handoff-issue"
	default:
		if !strings.HasPrefix(skillCmd, "/") {
			skillCmd = "/" + payload.SkillID
		}
	}

	// Resolve AI provider CLI tool
	provider := payload.Provider
	if provider == "" {
		if _, err := exec.LookPath("agy"); err == nil {
			provider = "agy"
		} else if _, err := exec.LookPath("claude"); err == nil {
			provider = "claude"
		} else if _, err := exec.LookPath("codex"); err == nil {
			provider = "codex"
		} else {
			provider = "agy"
		}
	}

	promptArg := skillCmd
	trimmedPrompt := strings.TrimSpace(payload.Prompt)
	if trimmedPrompt != "" && strings.HasPrefix(trimmedPrompt, "/") {
		promptArg = trimmedPrompt
	} else if payload.TaskKey != "" && !strings.Contains(promptArg, payload.TaskKey) {
		promptArg = fmt.Sprintf("%s %s", promptArg, payload.TaskKey)
	}

	var fullLine string
	switch strings.ToLower(provider) {
	case "agy":
		fullLine = fmt.Sprintf("agy -i %q", promptArg)
	case "claude":
		fullLine = fmt.Sprintf("claude %q", promptArg)
	case "codex":
		fullLine = fmt.Sprintf("codex %q", promptArg)
	case "vibe":
		fullLine = fmt.Sprintf("vibe -p %q", promptArg)
	default:
		fullLine = fmt.Sprintf("%s -i %q", provider, promptArg)
	}

	// Create or reuse a PTY session for this task.
	sessionID := "task-" + payload.TaskKey
	envVars := map[string]string{
		"TASKFLOW_TASK_KEY":    payload.TaskKey,
		"TASKFLOW_TASK_ID":     msg.TaskID,
		"TASKFLOW_REMOTE_MODE": "true",
	}

	// Determine terminal application:
	// 1. Explicit CLI flag --terminal
	// 2. Project/server external terminal configuration from payload
	// 3. Local project .taskflow/config.json in workDir or root
	// 4. Configured daemon default (TASKFLOW_TERMINAL or detectDefaultTerminal())
	termApp := ""
	if d.terminalExplicit {
		termApp = d.terminalApp
	} else if payload.ExternalTerminalCommand != "" {
		termApp = payload.ExternalTerminalCommand
	} else if localTerm := readProjectTerminalConfig(workDir); localTerm != "" {
		termApp = localTerm
	} else if rootTerm := readProjectTerminalConfig(root); rootTerm != "" {
		termApp = rootTerm
	} else if d.terminalApp != "" {
		termApp = d.terminalApp
	} else {
		termApp = detectDefaultTerminal()
	}

	if termApp != "" && termApp != "none" && termApp != "pty" {
		// Launch in configured external desktop terminal (Ghostty, iTerm, etc.)
		r := runner.NewRunner()
		log.Printf("🚀 [Agent] Launching external terminal (%s) for task %s: %s (workdir: %s)", termApp, payload.TaskKey, fullLine, workDir)
		fmt.Printf("\n🖥️  [Agent] Ouverture du terminal externe (%s) pour %s...\n", strings.ToUpper(termApp), payload.TaskKey)
		fmt.Printf("   Dossier : %s\n", workDir)
		fmt.Printf("   Commande: %s\n\n", fullLine)

		if err := r.OpenExternalTerminal(termApp, workDir, fullLine, envVars); err != nil {
			log.Printf("[Agent] Failed to open external terminal %s: %v, falling back to background PTY", termApp, err)
			d.runInPty(sessionID, workDir, envVars, fullLine)
		} else {
			d.sendStatus(conn, msg.MsgID, msg.TaskID, "completed", fmt.Sprintf("Terminal externe (%s) ouvert pour %s", termApp, payload.TaskKey))
			return
		}
	} else {
		d.runInPty(sessionID, workDir, envVars, fullLine)
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "completed", fmt.Sprintf("Step %s launched in local PTY", payload.Action))
	}
}

// runInPty starts or reuses an embedded PTY session and injects the command line.
func (d *agentDaemon) runInPty(sessionID, workDir string, envVars map[string]string, fullLine string) {
	sess, err := d.terminalMgr.GetOrCreateSession(sessionID, workDir, envVars)
	if err != nil {
		log.Printf("[Agent] Failed to create PTY session: %v", err)
		return
	}

	sess.AddOutputListener(func(chunk []byte) {
		_, _ = os.Stdout.Write(chunk)
	})

	time.Sleep(350 * time.Millisecond)
	log.Printf("⚡ [Agent] Launching skill command in local PTY terminal: %s (workdir: %s)", fullLine, workDir)
	_ = d.terminalMgr.SendInput(sessionID, fullLine+"\n")
}

// sendStatus sends a step_status message back to the remote server.
func (d *agentDaemon) sendStatus(conn *websocket.Conn, msgID, taskID, status, summary string) {
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
