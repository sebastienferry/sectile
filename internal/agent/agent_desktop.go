package agent

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agenthttp"
	"tasks/internal/models"
	"tasks/internal/runner"
	"tasks/internal/version"
	"time"
)

// loopbackServer is the agent's private HTTP surface: the Electron companion
// and the agent's own subprocesses reach it over the loopback interface only,
// and it never leaves the machine. Its fields are set once at start and read
// afterwards, so they need no lock of their own.
type loopbackServer struct {
	server *http.Server
	port   int
	url    string
	// Proxied requests use serverLink.token upstream. Local MCP may omit a
	// client key only after explicit opt-in; the companion has its own token.
	// desktopToken authenticates the companion; desktopInfo is the handshake
	// file it reads to find this session.
	desktopToken string
	desktopInfo  string
	// echoConsoles mirrors console output on the agent's own stdout. Off by
	// default: it is a debugging aid, not a way to read runs.
	echoConsoles bool
}

type desktopRun struct {
	Branch          string    `json:"branch,omitempty"`
	Kind            string    `json:"kind,omitempty"`
	Provider        string    `json:"provider,omitempty"`
	Model           string    `json:"model,omitempty"`
	CancelRequested bool      `json:"cancelRequested,omitempty"`
	QueueSequence   uint64    `json:"queueSequence,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	StartedAt       time.Time `json:"startedAt,omitzero"`
	Prompt          string    `json:"prompt,omitempty"`
	ID              string    `json:"id"`
	TaskID          string    `json:"taskId"`
	TaskKey         string    `json:"taskKey"`
	// MacroKey is set on a macro skill run, which has no task. TaskKey then
	// carries the macro key too, for the label.
	MacroKey  string `json:"macroKey,omitempty"`
	ProjectID string `json:"projectId"`
	Skill     string `json:"skill"`
	SessionID string `json:"sessionId"`
	Directory string `json:"directory"`
	Status    string `json:"status"`
	// ExternalTerminal marks the terminal emulator currently attached to or running this session.
	ExternalTerminal string `json:"externalTerminal,omitempty"`
	// Headless marks a run that has no PTY on purpose. The desktop shows its
	// captured output read-only instead of reporting a missing console.
	Headless bool `json:"headless,omitempty"`
	// Trace marks a headless run whose engine is reporting what it does as it
	// does it. The desktop attaches to it read-only rather than answering the
	// selection with the notice it shows for a run that has nothing to watch.
	// An agent that cannot trace sends nothing here, and that notice is what its
	// runs keep showing.
	Trace bool `json:"trace,omitempty"`
}

func (d *agentDaemon) desktopHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if r.Header.Get("Origin") != "" || d.loopback.desktopToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(d.loopback.desktopToken)) != 1 {
		http.Error(w, "Unauthorized", 401)
		return
	}
	// The build the companion is talking to. It is its own route rather than a
	// field on /desktop/status because status is polled every few seconds and
	// the version never changes while the process lives.
	if r.URL.Path == "/desktop/version" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(version.Current())
		return
	}
	if r.URL.Path == "/desktop/mcp" {
		d.desktopMCP(w, r)
		return
	}
	if r.URL.Path == "/desktop/status" && r.Method == http.MethodGet {
		d.link.mu.Lock()
		connected := d.link.conn != nil
		d.link.mu.Unlock()
		settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		disconnected := []string{}
		for id, value := range settings.DisconnectedProjects {
			if value {
				disconnected = append(disconnected, id)
			}
		}
		sort.Strings(disconnected)
		// contractError separates a server that is merely unreachable from one
		// that cannot be talked to at all. Without it the desktop reports both
		// as a disconnection and the user has no reason to look at the build.
		_ = json.NewEncoder(w).Encode(map[string]any{"connected": connected, "server": d.link.serverURL, "contractError": d.contract.current(), "capabilities": []string{"git-diff", "create-task", "remove-project", "free-console", "transition-stage"}, "disconnectedProjects": disconnected})
		return
	}
	if (r.URL.Path == "/desktop/restart" || r.URL.Path == "/desktop/shutdown") && r.Method == http.MethodPost {
		if !d.prepareMu.TryLock() {
			http.Error(w, "Project preparation or deployment is in progress", 409)
			return
		}
		defer d.prepareMu.Unlock()
		d.queue.mu.Lock()
		defer d.queue.mu.Unlock()
		if d.restartAgent == nil || d.queue.shuttingDown {
			http.Error(w, "Restart unavailable", http.StatusConflict)
			return
		}
		for _, run := range d.queue.runs {
			select {
			case <-run.exited:
			default:
				http.Error(w, "Stop active executions before restarting", http.StatusConflict)
				return
			}
		}
		d.queue.shuttingDown = true
		d.queue.restartRequested = r.URL.Path == "/desktop/restart"
		w.WriteHeader(http.StatusNoContent)
		d.restartAgent()
		return
	}
	if r.URL.Path == "/desktop/git-diff" {
		d.desktopGitDiff(w, r)
		return
	}
	if r.URL.Path == "/desktop/consoles" {
		d.desktopConsole(w, r)
		return
	}
	if r.URL.Path == "/desktop/create-task" {
		d.desktopCreateTask(w, r)
		return
	}
	if r.URL.Path == "/desktop/run-result" && r.Method == http.MethodGet {
		d.desktopRunResult(w, r)
		return
	}
	if r.URL.Path == "/desktop/tasks/transition" {
		d.desktopTaskTransition(w, r)
		return
	}
	if r.URL.Path == "/desktop/tasks/terminal-external" {
		d.desktopTasksTerminalExternal(w, r)
		return
	}
	if r.URL.Path == "/desktop/terminal/detach" {
		d.desktopTerminalDetach(w, r)
		return
	}
	if r.URL.Path == "/desktop/tasks" {
		d.desktopTasks(w, r)
		return
	}
	if r.URL.Path == "/desktop/project" {
		d.desktopProject(w, r)
		return
	}
	if r.URL.Path == "/desktop/projects" {
		d.desktopProjects(w, r)
		return
	}
	if r.URL.Path == "/desktop/history" && r.Method == http.MethodDelete {
		d.queue.mu.Lock()
		removed := []string{}
		for id, run := range d.queue.runs {
			select {
			case <-run.exited:
				if d.terminal.manager != nil && run.desktop.SessionID != "" {
					_ = d.terminal.manager.CloseSession(run.desktop.SessionID)
				}
				delete(d.queue.runs, id)
				removed = append(removed, id)
			default:
			}
		}
		d.queue.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"removed": removed})
		return
	}
	if r.URL.Path != "/desktop/runs" && r.URL.Path != "/desktop/stop" && r.URL.Path != "/desktop/terminal" {
		http.Error(w, "Unknown local agent endpoint", 404)
		return
	}
	id := r.URL.Query().Get("id")
	d.queue.mu.Lock()
	if r.URL.Path == "/desktop/runs" && r.Method == http.MethodGet {
		runs := []desktopRun{}
		for key, run := range d.queue.runs {
			entry := run.desktop
			entry.ID = key
			entry.QueueSequence = run.sequence
			entry.CancelRequested = run.canceled && (entry.Status == "queued" || entry.Status == "preparing" || entry.Status == "running")
			if entry.Status != "" {
				runs = append(runs, entry)
			}
		}
		d.queue.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(runs)
		return
	}
	run := d.queue.runs[id]
	if run == nil {
		d.queue.mu.Unlock()
		http.Error(w, "Run not found", 404)
		return
	}
	entry := run.desktop
	trace := run.trace
	if r.URL.Path == "/desktop/stop" && r.Method == http.MethodPost {
		run.canceled = true
		d.queue.mu.Unlock()
		// A supervised PTY run normally closes exited through agent-exec. If the
		// terminal has already vanished, there is no process left that can send
		// that acknowledgement. Recover it here instead of making every Stop
		// retry wait twelve seconds and return 504 forever.
		if d.recoverOrphanedPTYRun(id, run) {
			_ = d.finishDesktopRun(context.Background(), entry.TaskID, id, stoppedStatus(entry.Skill), stoppedNote(entry.Skill, true))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		select {
		case <-run.exited:
			// The native client may have already reported completion via MCP.
			_ = d.finishDesktopRun(context.Background(), entry.TaskID, id, stoppedStatus(entry.Skill), stoppedNote(entry.Skill, false))
			w.WriteHeader(http.StatusNoContent)
		case <-time.After(12 * time.Second):
			if d.recoverOrphanedPTYRun(id, run) {
				_ = d.finishDesktopRun(context.Background(), entry.TaskID, id, stoppedStatus(entry.Skill), stoppedNote(entry.Skill, true))
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.Error(w, "Exit not confirmed", http.StatusGatewayTimeout)
		}
		return
	}
	d.queue.mu.Unlock()
	if r.URL.Path == "/desktop/terminal" && r.Method == http.MethodGet {
		exists := false
		for _, session := range d.terminal.manager.ListSessions() {
			if session.ID == entry.SessionID {
				exists = true
				break
			}
		}
		if !exists {
			// A run with no console can still have something to watch: an
			// autonomous run reports what it is doing, and the desktop attaches
			// to that trace over this same route rather than opening a second
			// kind of connection for it.
			if trace != nil {
				serveRunTrace(w, r, trace)
				return
			}
			http.Error(w, "Console is not ready", 409)
			return
		}

		d.terminal.manager.HandleWebSocket(w, r, entry.SessionID, entry.Directory, nil)
		return
	}
	http.Error(w, "Not found", 404)
}

// recoverOrphanedPTYRun closes the local record only when the agent itself can
// prove that the embedded terminal which owned it no longer exists. Headless
// runs have no terminal by design and must still confirm their process exit.
func (d *agentDaemon) recoverOrphanedPTYRun(id string, run *controlledRun) bool {
	if d.terminal.manager == nil {
		return false
	}
	d.queue.mu.Lock()
	if d.queue.runs[id] != run || run.desktop.Headless || run.desktop.Status != "running" || run.desktop.SessionID == "" {
		d.queue.mu.Unlock()
		return false
	}
	sessionID := run.desktop.SessionID
	d.queue.mu.Unlock()
	for _, session := range d.terminal.manager.ListSessions() {
		if session.ID == sessionID {
			return false
		}
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if d.queue.runs[id] != run {
		return false
	}
	select {
	case <-run.exited:
		return false
	default:
	}
	run.desktop.Status = stoppedStatus(run.desktop.Skill)
	run.once.Do(func() { close(run.exited) })
	return true
}

func (d *agentDaemon) writeDesktopInfo() error {
	if d.loopback.desktopInfo == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(d.loopback.desktopInfo), 0700); err != nil {
		return err
	}
	// Publish atomically so a companion never reads a partially written credential.
	raw, _ := json.Marshal(map[string]string{"url": d.loopback.url, "token": d.loopback.desktopToken})
	file, err := os.CreateTemp(filepath.Dir(d.loopback.desktopInfo), ".agent-connection-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), d.loopback.desktopInfo)
}

// Report process exit using the server's authenticated MCP endpoint.
// finishDesktopRun reports a finished run with the reason it ended. The console
// path always ends the same way; a headless run has a real result to carry,
// including the error that stopped it.
func (d *agentDaemon) finishDesktopRun(ctx context.Context, taskID, runID, status, note string) error {
	// A run without a task is either a free console, which reports nothing, or
	// a macro skill run, which reports under its macro.
	arguments := map[string]string{"taskKey": taskID}
	if taskID == "" {
		projectID, macroKey := d.macroOfRun(runID)
		if macroKey == "" {
			return nil
		}
		arguments = map[string]string{"projectId": projectID, "macroKey": macroKey}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "sectile-desktop-agent", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: d.link.serverURL + "/mcp", HTTPClient: agenthttp.Client(d.link.token)}, nil)
	if err != nil {
		return err
	}
	defer session.Close()
	if strings.TrimSpace(note) == "" {
		note = "Local console process exited"
	}
	arguments["runId"], arguments["status"], arguments["note"] = runID, status, note
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "finish_run", Arguments: arguments})
	if err != nil {
		return err
	}
	if result.IsError {
		return fmt.Errorf("remote run completion was rejected: %v", result.Content)
	}
	if taskID == "" {
		d.forgetMacroRun(runID)
	}
	return nil
}

func mustJSON(v any) string { raw, _ := json.Marshal(v); return string(raw) }

// desktopProjects keeps workstation paths in the local configuration only.
func (d *agentDaemon) desktopProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		d.disconnectProject(w, r)
		return
	}
	if r.Method == http.MethodGet {
		projects, err := d.discoverProjects(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		type entry struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Path         string `json:"path"`
			Configured   bool   `json:"configured"`
			Disconnected bool   `json:"disconnected"`
		}
		entries := []entry{}
		base := d.repoRoot
		if base == "" {
			base, _ = os.Getwd()
			base = findRepoRoot(base)
		}
		settings, err := agentconfig.ReadSettings(base)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, p := range projects.Projects {
			root, _, _ := d.localProjectRoot(r.Context(), agentconfig.Config{ProjectID: p.ID, GitRemoteURL: p.GitRemoteURL})
			mapped, configured := settings.Projects[p.ID]
			if root == "" {
				root = mapped
			}
			disconnected := settings.DisconnectedProjects[p.ID]
			if disconnected {
				root = ""
				configured = false
			}
			entries = append(entries, entry{p.ID, p.Name, root, configured || root != "", disconnected})
		}
		_ = json.NewEncoder(w).Encode(entries)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		ProjectID                   string  `json:"projectId"`
		Path                        string  `json:"path"`
		AIProvider                  *string `json:"aiProvider"`
		AIModel                     *string `json:"aiModel"`
		InheritAIProvider           bool    `json:"inheritAiProvider"`
		InheritAIModel              bool    `json:"inheritAiModel"`
		AICommandTemplate           *string `json:"aiCommandTemplate"`
		AICommandTemplateAutonomous *string `json:"aiCommandTemplateAutonomous"`
		InheritCommand              bool    `json:"inheritCommand"`
		InheritWorktrees            bool    `json:"inheritWorktrees"`
		Parallelism                 *int    `json:"parallelism"`
		UseWorktrees                *bool   `json:"useWorktrees"`
		Terminal                    *string `json:"terminal"`
		InheritTerminal             bool    `json:"inheritTerminal"`
		// SpecPath is the local specifications checkout; empty clears it, so
		// the project's own checkout carries the specifications again.
		SpecPath *string `json:"specPath"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || input.ProjectID == "" || !filepath.IsAbs(input.Path) {
		http.Error(w, "Project and absolute repository path required", 400)
		return
	}
	if _, err := d.fetchConfig(r.Context(), input.ProjectID, ""); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if _, err := gitLocal(r.Context(), input.Path, "rev-parse", "--show-toplevel"); err != nil {
		http.Error(w, "Select a local Git repository", 400)
		return
	}
	specPath := ""
	if input.SpecPath != nil {
		specPath = strings.TrimSpace(*input.SpecPath)
		if specPath != "" {
			if !filepath.IsAbs(specPath) {
				http.Error(w, "The specifications repository must be an absolute path", 400)
				return
			}
			top, err := gitLocal(r.Context(), specPath, "rev-parse", "--show-toplevel")
			if err != nil {
				http.Error(w, "Select a local Git repository for the specifications", 400)
				return
			}
			// A folder inside a repository names that repository: the macro
			// worktree is created at its root, where specs/ is looked for.
			specPath = filepath.Clean(top)
		}
	}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root := d.repoRoot
	if root == "" {
		root, _ = os.Getwd()
		root = findRepoRoot(root)
	}
	overrides, err := agentconfig.ReadSettings(root)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if input.AIProvider != nil && !input.InheritAIProvider {
		provider := strings.TrimSpace(*input.AIProvider)
		if err := agentconfig.ValidProvider(provider); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if provider == "custom" {
			cmd := ""
			if input.AICommandTemplate != nil && !input.InheritCommand {
				cmd = strings.TrimSpace(*input.AICommandTemplate)
			} else if !input.InheritCommand {
				cmd = overrides.Commands[input.ProjectID]
			}
			if !strings.Contains(cmd, "{prompt}") {
				http.Error(w, "Custom provider requires a command template containing {prompt}", 400)
				return
			}
		}
	}
	if input.AIModel != nil && !input.InheritAIModel {
		if err := agentconfig.ValidModel(*input.AIModel); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	if overrides.Projects == nil {
		overrides.Projects = map[string]string{}
	}
	if input.InheritAIProvider {
		delete(overrides.AIProviders, input.ProjectID)
	} else if input.AIProvider != nil {
		provider := strings.TrimSpace(*input.AIProvider)
		if provider == "" {
			delete(overrides.AIProviders, input.ProjectID)
		} else {
			if overrides.AIProviders == nil {
				overrides.AIProviders = map[string]string{}
			}
			overrides.AIProviders[input.ProjectID] = provider
		}
	}
	if input.InheritAIModel {
		delete(overrides.AIModels, input.ProjectID)
	} else if input.AIModel != nil {
		model := strings.TrimSpace(*input.AIModel)
		if model == "" {
			delete(overrides.AIModels, input.ProjectID)
		} else {
			if overrides.AIModels == nil {
				overrides.AIModels = map[string]string{}
			}
			overrides.AIModels[input.ProjectID] = model
		}
	}
	// The two commands are overridden together: a workstation that pins only the
	// interactive one would keep running the server's headless command beside it,
	// which is the opposite of what an override is for.
	if input.AICommandTemplate != nil || input.AICommandTemplateAutonomous != nil || input.InheritCommand {
		if overrides.Commands == nil {
			overrides.Commands = map[string]string{}
		}
		if overrides.CommandsAutonomous == nil {
			overrides.CommandsAutonomous = map[string]string{}
		}
		command, autonomous := "", ""
		if input.AICommandTemplate != nil {
			command = strings.TrimSpace(*input.AICommandTemplate)
		}
		if input.AICommandTemplateAutonomous != nil {
			autonomous = strings.TrimSpace(*input.AICommandTemplateAutonomous)
		}
		if len(command) > 4096 || len(autonomous) > 4096 {
			http.Error(w, "CLI command is too long", 400)
			return
		}
		if input.InheritCommand {
			command, autonomous = "", ""
		}
		overrides.Commands[input.ProjectID] = command
		overrides.CommandsAutonomous[input.ProjectID] = autonomous
	}
	if input.Parallelism != nil {
		if *input.Parallelism < 1 || *input.Parallelism > agentconfig.MaxParallelism {
			http.Error(w, fmt.Sprintf("Parallelism must be between 1 and %d", agentconfig.MaxParallelism), 400)
			return
		}
		if overrides.Parallelism == nil {
			overrides.Parallelism = map[string]int{}
		}
		overrides.Parallelism[input.ProjectID] = *input.Parallelism
	}
	overrides.Projects[input.ProjectID] = input.Path
	if input.SpecPath != nil {
		if specPath == "" {
			delete(overrides.SpecRepos, input.ProjectID)
		} else {
			if overrides.SpecRepos == nil {
				overrides.SpecRepos = map[string]string{}
			}
			overrides.SpecRepos[input.ProjectID] = specPath
		}
	}
	if input.UseWorktrees != nil {
		if overrides.Worktrees == nil {
			overrides.Worktrees = map[string]bool{}
		}
		overrides.Worktrees[input.ProjectID] = *input.UseWorktrees
	}
	if input.InheritWorktrees {
		delete(overrides.Worktrees, input.ProjectID)
	}
	if input.InheritTerminal {
		delete(overrides.Terminals, input.ProjectID)
	} else if input.Terminal != nil {
		termChoice := strings.TrimSpace(*input.Terminal)
		if termChoice == "" {
			delete(overrides.Terminals, input.ProjectID)
		} else {
			if overrides.Terminals == nil {
				overrides.Terminals = map[string]string{}
			}
			overrides.Terminals[input.ProjectID] = termChoice
		}
	}
	delete(overrides.DisconnectedProjects, input.ProjectID)
	if err := agentconfig.WriteSettings(overrides); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (d *agentDaemon) localSettingsRoot() string {
	root := d.repoRoot
	if root == "" {
		root, _ = os.Getwd()
		root = findRepoRoot(root)
	}
	return root
}

func (d *agentDaemon) disconnectProject(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "Project ID required", 400)
		return
	}
	if !d.prepareMu.TryLock() {
		http.Error(w, "Project preparation or configuration is in progress; retry removal", 409)
		return
	}
	defer d.prepareMu.Unlock()
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	for _, run := range d.queue.runs {
		if run.desktop.ProjectID != id {
			continue
		}
		select {
		case <-run.exited:
		default:
			http.Error(w, "Stop active executions and wait for them to exit before removing this project", 409)
			return
		}
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if settings.DisconnectedProjects == nil {
		settings.DisconnectedProjects = map[string]bool{}
	}
	settings.DisconnectedProjects[id] = true
	delete(settings.Projects, id)
	delete(settings.Worktrees, id)
	delete(settings.Parallelism, id)
	delete(settings.Commands, id)
	delete(settings.CommandsAutonomous, id)
	delete(settings.AIProviders, id)
	delete(settings.AIModels, id)
	if err := agentconfig.WriteSettings(settings); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Probe only loopback endpoints from the private discovery file.
func localAgentAvailable(file string) bool {
	raw, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	var connection struct {
		URL   string
		Token string
	}
	if json.Unmarshal(raw, &connection) != nil || connection.Token == "" {
		return false
	}
	endpoint, err := url.Parse(connection.URL)
	if err != nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.User != nil {
		return false
	}
	req, err := http.NewRequest(http.MethodGet, connection.URL+"/desktop/status", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+connection.Token)
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

// desktopProject exposes server metadata without allowing server configuration edits.
func (d *agentDaemon) desktopProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Project required", 400)
		return
	}
	config, err := d.fetchConfig(r.Context(), id, "")
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root, overrides, mappingErr := d.localProjectRoot(r.Context(), config)
	if r.Method == http.MethodGet {
		var project models.Project
		if err := d.readAPI(r.Context(), "/api/projects/"+url.PathEscape(id), &project); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		effective := agentconfig.ApplyOverrides(config, overrides)
		_, worktreeOverride := overrides.Worktrees[id]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"server":                      config,
			"monoRepo":                    project.MonoRepo,
			"path":                        root,
			"specPath":                    overrides.SpecRepos[id],
			"useWorktrees":                effective.UseWorktrees,
			"configured":                  mappingErr == nil,
			"aiCommandTemplate":           effective.AICommandTemplate,
			"aiCommandTemplateAutonomous": effective.AICommandTemplateAutonomous,
			"commandOverride":             overrides.Commands[id] != "" || overrides.CommandsAutonomous[id] != "",
			"worktreeOverride":            worktreeOverride,
			"parallelism":                 agentconfig.ExecutionLimit(id, effective.UseWorktrees, overrides),
			"aiProvider":                  effective.AIProvider,
			"aiModel":                     effective.AIModel,
			"aiProviderOverride":          overrides.AIProviders[id] != "",
			"aiModelOverride":             overrides.AIModels[id] != "",
			"terminal":                    effective.ExternalTerminalCommand,
			"terminalOverride":            overrides.Terminals[id] != "",
		})
		return
	}
	if mappingErr != nil {
		http.Error(w, mappingErr.Error(), 400)
		return
	}
	// Do not change scaffolding underneath active native clients.
	d.queue.mu.Lock()
	for _, run := range d.queue.runs {
		select {
		case <-run.exited:
		default:
			d.queue.mu.Unlock()
			http.Error(w, "Stop active executions before deploying project tooling", 409)
			return
		}
	}
	if d.queue.shuttingDown {
		d.queue.mu.Unlock()
		http.Error(w, "Agent is stopping", 409)
		return
	}
	d.queue.mu.Unlock()
	switch r.URL.Query().Get("action") {
	case "initialize":
		provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
		if _, err := agentconfig.ResolveLocations(provider); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Attempt failures are structured so the UI preserves partial success.
		result, _ := d.initializeProvider(root, config, provider)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	case "skills":
		_, err = agentconfig.Scaffold(root, config)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Server skills deployed"})
	case "framework":
		if config.SpecFramework != "openspec" && config.SpecFramework != "speckit" {
			http.Error(w, "No supported SDD framework configured", 400)
			return
		}
		result := runner.NewRunner().InstallSpecFramework(models.SpecFrameworkInstallRequest{Framework: config.SpecFramework, RepoPath: root, ProjectID: id, AIAgent: config.AIProvider})
		if result.Error != "" {
			http.Error(w, result.Error, 500)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	default:
		http.Error(w, "Unknown deployment action", 400)
	}
}

// desktopTasks browses server tasks and reuses the web dispatch path.
func (d *agentDaemon) desktopTasks(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		http.Error(w, "Project required", 400)
		return
	}
	config, err := d.fetchConfig(r.Context(), projectID, "")
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if r.Method == http.MethodGet {
		var tasks []models.Task
		q := url.Values{"projectId": {projectID}, "q": {r.URL.Query().Get("q")}}
		if err := d.readAPI(r.Context(), "/api/tasks?"+q.Encode(), &tasks); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		if tasks == nil {
			tasks = []models.Task{}
		}
		if r.URL.Query().Get("launchable") == "true" {
			filtered := []models.Task{}
			for _, task := range tasks {
				if !desktopTaskFinished(task) {
					filtered = append(filtered, task)
				}
			}
			tasks = filtered
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tasks)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		TaskID  string
		SkillID string
		Prompt  string
		// Mode is the one-off execution mode the user chose in the Launch or
		// Relaunch dialog. Empty means no override: the server's precedence
		// still applies. The agent does not interpret it, it passes it on.
		Mode string
		// Force asks the server to skip its duplicate-launch refusal. As with
		// Mode, the agent does not interpret it, it passes it on.
		Force bool
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || input.TaskID == "" {
		http.Error(w, "Task and skill required", 400)
		return
	}
	var task models.Task
	if err := d.readAPI(r.Context(), "/api/tasks/"+url.PathEscape(input.TaskID), &task); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if desktopTaskFinished(task) {
		http.Error(w, "This task is finished. Reopen it on the server before launching an execution.", 409)
		return
	}
	if task.ProjectID != projectID {
		http.Error(w, "Task does not belong to project", 400)
		return
	}
	if !launchableSkill(config, input.SkillID, input.Prompt) {
		http.Error(w, "Unknown project skill", 400)
		return
	}
	if _, _, err := d.localProjectRoot(r.Context(), config); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if !models.ValidSkillMode(input.Mode) {
		http.Error(w, "Unknown execution mode", 400)
		return
	}
	body := mustJSON(map[string]any{"skillId": input.SkillID, "prompt": input.Prompt, "mode": input.Mode, "force": input.Force})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, d.link.serverURL+"/api/tasks/"+url.PathEscape(task.ID)+"/run-skill", strings.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}

func (d *agentDaemon) desktopCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		ProjectID   string
		Title       string
		Description string
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&input) != nil || input.ProjectID == "" || strings.TrimSpace(input.Title) == "" {
		http.Error(w, "Project and title required", 400)
		return
	}
	if _, err := d.fetchConfig(r.Context(), input.ProjectID, ""); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	body := mustJSON(models.CreateTaskRequest{ProjectID: input.ProjectID, Title: strings.TrimSpace(input.Title), Description: input.Description, RequireRemoteCreation: true})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, d.link.serverURL+"/api/tasks", strings.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}

func (d *agentDaemon) desktopTaskTransition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		ProjectID string `json:"projectId"`
		TaskID    string `json:"taskId"`
		Stage     string `json:"stage"`
		Note      string `json:"note"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&input) != nil {
		http.Error(w, "Invalid request body", 400)
		return
	}
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		projectID = input.ProjectID
	}
	if projectID == "" || strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.Stage) == "" {
		http.Error(w, "Project, task, and stage required", 400)
		return
	}
	if _, err := d.fetchConfig(r.Context(), projectID, ""); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	body := mustJSON(map[string]string{
		"stage": strings.TrimSpace(input.Stage),
		"note":  input.Note,
	})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, d.link.serverURL+"/api/tasks/"+url.PathEscape(input.TaskID)+"/stage", strings.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}

// launchableSkill admits a project skill or one of the reserved identifiers.
// A discussion carries nothing, unlike custom instructions.
func launchableSkill(config agentconfig.Config, skillID, prompt string) bool {
	if skillID == "discuss" {
		return true
	}
	if skillID == "custom" {
		return strings.TrimSpace(prompt) != ""
	}
	for _, skill := range config.Skills {
		if skill.ID == skillID {
			return true
		}
	}
	return false
}

func desktopTaskFinished(task models.Task) bool {
	if task.Status == models.StatusFinished || task.Status == models.StatusDone {
		return true
	}
	for _, label := range task.Labels {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(label), "#"), "finished") {
			return true
		}
	}
	return false
}

// Resolve the selected execution against server activity, independently of PTY exit.
func (d *agentDaemon) desktopRunResult(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	var taskID, projectID string
	if !d.queue.read(id, func(run *controlledRun) { taskID, projectID = run.taskID, run.desktop.ProjectID }) {
		http.Error(w, "Run not found", 404)
		return
	}
	if taskID == "" {
		http.Error(w, "Free consoles have no task result", http.StatusNotFound)
		return
	}
	var task models.Task
	if err := d.readAPI(r.Context(), "/api/tasks/"+url.PathEscape(taskID), &task); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if task.ID != taskID || task.ProjectID != projectID {
		http.Error(w, "Task does not match execution", 409)
		return
	}
	var activities []models.TaskActivity
	if err := d.readAPI(r.Context(), "/api/tasks/"+url.PathEscape(taskID)+"/activities", &activities); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	var activity any
	for _, item := range activities {
		if item.ID == id && item.TaskID == taskID {
			// A remote run stores the record kind in SkillID ("remote_run") and the
			// launched skill in SkillName. The desktop matches the launched skill.
			activity = map[string]string{"id": item.ID, "taskId": item.TaskID, "skillId": item.SkillName, "status": item.Status}
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"activity": activity, "task": map[string]any{"status": task.Status, "labels": task.Labels}})
}

// resolveTerminalForProject determines the terminal emulator based on precedence:
// 1. Explicit call override
// 2. Project local override (overrides.Terminals[projectID])
// 3. Workstation setting (overrides.Terminal)
// 4. Server project configuration (config.ExternalTerminalCommand)
// 5. Agent daemon flag (d.terminal.app)
// 6. System auto-detection (detectDefaultTerminal())
func (d *agentDaemon) resolveTerminalForProject(ctx context.Context, projectID, override string) string {
	override = strings.TrimSpace(override)
	if override != "" {
		return override
	}
	base := d.repoRoot
	if base == "" {
		base, _ = os.Getwd()
		base = findRepoRoot(base)
	}
	overrides, err := agentconfig.ReadSettings(base)
	if err == nil {
		if projectID != "" && overrides.Terminals[projectID] != "" {
			return overrides.Terminals[projectID]
		}
		if overrides.Terminal != "" {
			return overrides.Terminal
		}
	}
	if projectID != "" {
		if config, err := d.fetchConfig(ctx, projectID, ""); err == nil && config.ExternalTerminalCommand != "" {
			return config.ExternalTerminalCommand
		}
	}
	if d.terminal.app != "" {
		return d.terminal.app
	}
	return detectDefaultTerminal()
}

func (d *agentDaemon) desktopTerminalDetach(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		RunID    string `json:"runId"`
		Terminal string `json:"terminal"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input); err != nil || strings.TrimSpace(input.RunID) == "" {
		http.Error(w, "Run ID required", http.StatusBadRequest)
		return
	}

	d.queue.mu.Lock()
	run := d.queue.runs[input.RunID]
	if run == nil {
		d.queue.mu.Unlock()
		http.Error(w, "Run not found", http.StatusNotFound)
		return
	}

	select {
	case <-run.exited:
		d.queue.mu.Unlock()
		http.Error(w, "Run already exited", http.StatusConflict)
		return
	default:
	}

	sessionID := run.desktop.SessionID
	if sessionID == "" {
		sessionID = run.desktop.ID
	}
	projectID := run.desktop.ProjectID
	d.queue.mu.Unlock()

	termChoice := d.resolveTerminalForProject(r.Context(), projectID, input.Terminal)

	if err := d.launchExternalTerminal(termChoice, sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	d.queue.mu.Lock()
	run.desktop.ExternalTerminal = termChoice
	d.queue.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"detached": true,
		"terminal": termChoice,
	})
}

func (d *agentDaemon) desktopTasksTerminalExternal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		ProjectID string `json:"projectId"`
		TaskID    string `json:"taskId"`
		SkillID   string `json:"skillId"`
		Terminal  string `json:"terminal"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input); err != nil || strings.TrimSpace(input.ProjectID) == "" || strings.TrimSpace(input.TaskID) == "" {
		http.Error(w, "Project and task required", http.StatusBadRequest)
		return
	}
	if input.SkillID == "" {
		input.SkillID = "discuss"
	}

	config, err := d.fetchConfig(r.Context(), input.ProjectID, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	var task models.Task
	if err := d.readAPI(r.Context(), "/api/tasks/"+url.PathEscape(input.TaskID), &task); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if desktopTaskFinished(task) {
		http.Error(w, "This task is finished. Reopen it on the server before launching an execution.", http.StatusConflict)
		return
	}
	if task.ProjectID != input.ProjectID {
		http.Error(w, "Task does not belong to project", http.StatusBadRequest)
		return
	}

	d.prepareMu.Lock()
	root, overrides, err := d.localProjectRoot(r.Context(), config)
	d.prepareMu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	workDir := root
	branch := "main"
	if task.BranchName != nil && *task.BranchName != "" {
		branch = *task.BranchName
	}
	if config.UseWorktrees {
		worktreeDir := filepath.Join(root, ".tasks", "worktrees", task.Key)
		if info, err := os.Stat(worktreeDir); err == nil && info.IsDir() {
			workDir = worktreeDir
		}
	}

	runID := uuid.NewString()

	d.queue.mu.Lock()
	limit := agentconfig.ExecutionLimit(input.ProjectID, config.UseWorktrees, overrides)
	run, err := d.enqueueRunLocked(task.ID, agentconfig.Dispatch{RunID: runID}, input.ProjectID, workDir, limit, false)
	if err != nil {
		d.queue.mu.Unlock()
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	termChoice := d.resolveTerminalForProject(r.Context(), input.ProjectID, input.Terminal)

	run.desktop = desktopRun{
		ID:               runID,
		TaskID:           task.ID,
		TaskKey:          task.Key,
		ProjectID:        input.ProjectID,
		Skill:            input.SkillID,
		SessionID:        runID,
		Directory:        workDir,
		Branch:           branch,
		Status:           "running",
		CreatedAt:        time.Now().UTC(),
		StartedAt:        time.Now().UTC(),
		ExternalTerminal: termChoice,
	}
	d.queue.mu.Unlock()

	envVars := map[string]string{
		"SECTILE_TASK_KEY":      task.Key,
		"SECTILE_TASK_BRANCH":   branch,
		"SECTILE_TASK_WORKTREE": workDir,
		"SECTILE_TASK_ID":       task.ID,
		"SECTILE_RUN_ID":        runID,
		"SECTILE_REMOTE_MODE":   "true",
		"SECTILE_AGENT_URL":     d.link.serverURL,
		"SECTILE_SERVER_URL":    d.link.serverURL,
		"SECTILE_AGENT_TOKEN":   d.link.token,
		"SECTILE_LOOPBACK_URL":  d.loopback.url,
		"SECTILE_PROJECT_ID":    input.ProjectID,
	}

	if d.terminal.manager != nil {
		if _, err := d.terminal.manager.GetOrCreateSession(runID, workDir, envVars); err != nil {
			d.queue.mu.Lock()
			delete(d.queue.runs, runID)
			d.queue.mu.Unlock()
			http.Error(w, fmt.Sprintf("Failed to initialize PTY session: %v", err), http.StatusInternalServerError)
			return
		}
	}

	if err := d.launchExternalTerminal(termChoice, runID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to launch external terminal: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"runId":    runID,
		"terminal": termChoice,
	})
}
