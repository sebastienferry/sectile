package agent

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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
	// binarySha256 fingerprints the executable this agent was started from,
	// hashed at start: by the time the companion asks, the file on disk may
	// already be a newer build. Empty when the executable could not be read.
	binarySha256 string
}

// desktopVersion answers /desktop/version: the build, plus the fingerprint that
// lets the companion tell a same-version rebuild from the binary it bundles.
type desktopVersion struct {
	version.Info
	BinarySha256 string `json:"binarySha256,omitempty"`
}

// executableSha256 hashes the running executable's content, "" when it cannot
// be read. The version and the commit do not change on a rebuild with
// uncommitted changes; the content does.
func executableSha256() string {
	binary, err := os.Executable()
	if err != nil {
		return ""
	}
	return fileSha256(binary)
}

func fileSha256(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type desktopRun struct {
	Conversation    bool      `json:"conversation,omitempty"`
	EngineID        string    `json:"engineId,omitempty"`
	EngineName      string    `json:"engineName,omitempty"`
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
	// Folders are every folder of the run, Directory first, when it has more
	// than one (#762): the desktop offers to copy each. A folder prepared or
	// attached while the run goes is added to it. Absent for a run launched
	// without a folder map, and from an older agent.
	Folders []runFolder `json:"folders,omitempty"`
	Status  string      `json:"status"`
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
	// WaitingSince is set while the session is blocked on the user. The desktop
	// reads it to raise its notification and to mark the run in its list. The
	// server sends it, as the session declares it over MCP (#318).
	WaitingSince time.Time `json:"waitingSince,omitzero"`
	// Restored marks a run the agent loaded from its run store at start
	// (#588). It has no session: the desktop attaches to it read-only and is
	// replayed what its console showed when the previous agent last wrote it.
	Restored bool `json:"restored,omitempty"`
}

func (d *agentDaemon) desktopHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if r.Header.Get("Origin") != "" || d.loopback.desktopToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(d.loopback.desktopToken)) != 1 {
		http.Error(w, "Unauthorized", 401)
		return
	}
	if r.URL.Path == "/desktop/conversation" {
		d.desktopConversation(w, r)
		return
	}
	if r.URL.Path == "/desktop/conversation-terminal" {
		d.desktopConversationTerminal(w, r)
		return
	}
	if r.URL.Path == "/desktop/project-terminal" {
		d.desktopProjectTerminal(w, r)
		return
	}
	// The build the companion is talking to. It is its own route rather than a
	// field on /desktop/status because status is polled every few seconds and
	// the version never changes while the process lives.
	if r.URL.Path == "/desktop/version" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(desktopVersion{Info: version.Current(), BinarySha256: d.loopback.binarySha256})
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
		capabilities := []string{"git-diff", markdownDocumentsCapability, markdownImagesCapability, "create-task", "remove-project", "free-console", "transition-stage", "repositories", attachedFoldersCapability, "git-init", taskEnginesCapability, openEditorCapability, "claude-conversation", "codex-conversation", codexSettingsCapability, conversationControlsCapability, conversationQueueCapability, runFoldersCapability, runFoldersTerminalsCapability, consoleViewCapability, conversationModeCapability, projectTerminalCapability, archiveWorkspaceCapability}
		if d.store != nil {
			capabilities = append(capabilities, runStoreCapability)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"connected": connected, "server": d.link.serverURL, "contractError": d.contract.current(), "capabilities": capabilities, "disconnectedProjects": disconnected})
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
	if r.URL.Path == "/desktop/tasks/archive-workspace" {
		d.desktopArchiveWorkspace(w, r)
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
	if r.URL.Path == "/desktop/open-editor" {
		d.desktopOpenEditor(w, r)
		return
	}
	if r.URL.Path == "/desktop/tasks" {
		d.desktopTasks(w, r)
		return
	}
	if r.URL.Path == "/desktop/engines" {
		d.desktopEngines(w, r)
		return
	}
	if r.URL.Path == "/desktop/console-view" {
		d.desktopConsoleView(w, r)
		return
	}
	if r.URL.Path == "/desktop/codex-settings" {
		d.desktopCodexSettings(w, r)
		return
	}
	if r.URL.Path == "/desktop/conversation-mode" {
		d.desktopConversationMode(w, r)
		return
	}
	if r.URL.Path == "/desktop/task-engines" {
		d.desktopTaskEngines(w, r)
		return
	}
	if r.URL.Path == "/desktop/workstation" {
		d.desktopWorkstation(w, r)
		return
	}
	if r.URL.Path == "/desktop/workstation/sandbox" {
		d.desktopWorkstationSandbox(w, r)
		return
	}
	if r.URL.Path == "/desktop/project" {
		d.desktopProject(w, r)
		return
	}
	if r.URL.Path == "/desktop/project/sandbox/promote" {
		d.desktopPromoteSandboxRule(w, r)
		return
	}
	if r.URL.Path == "/desktop/projects" {
		d.desktopProjects(w, r)
		return
	}
	if r.URL.Path == "/desktop/repositories" {
		d.desktopRepositories(w, r)
		return
	}
	if r.URL.Path == "/desktop/run-folder" {
		d.desktopRunFolder(w, r)
		return
	}
	if r.URL.Path == "/desktop/folders" {
		d.desktopFolders(w, r)
		return
	}
	if r.URL.Path == "/desktop/git-init" {
		d.desktopGitInit(w, r)
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
		d.store.remove(removed...)
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
		if run.conversation != nil {
			if !run.conversation.busy && run.conversation.codex == nil {
				run.desktop.Status = conversationStoppedStatus(run)
				run.once.Do(func() { close(run.exited) })
				run.trace.close()
			}
			d.queue.mu.Unlock()
			select {
			case <-run.exited:
				// A ticket discussion is a server run: stopping it ends it there too.
				if entry.TaskID != "" {
					_ = d.finishDesktopRun(context.Background(), entry.TaskID, id, stoppedStatus(entry.Skill), stoppedNote(entry.Skill, false))
				}
				w.WriteHeader(http.StatusNoContent)
			case <-time.After(12 * time.Second):
				http.Error(w, "Exit not confirmed", http.StatusGatewayTimeout)
			}
			return
		}
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
			mapped := settings.ProjectPath(p.ID)
			configured := mapped != ""
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
	var input projectSettingsInput
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&input) != nil || input.ProjectID == "" || !filepath.IsAbs(input.Path) {
		http.Error(w, "Project and absolute repository path required", 400)
		return
	}
	config, err := d.fetchConfig(r.Context(), input.ProjectID, "")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if _, err := gitLocal(r.Context(), input.Path, "rev-parse", "--show-toplevel"); err != nil {
		http.Error(w, "Select a local Git repository", 400)
		return
	}
	if input.SpecArtifacts != nil && !input.InheritSpecArtifacts {
		if value := *input.SpecArtifacts; value != models.SpecArtifactsKeep && value != models.SpecArtifactsDrop {
			http.Error(w, "Specifications must be keep or drop", 400)
			return
		}
	}
	specPath, issueSpecPath := "", ""
	for _, folder := range []struct {
		input   *string
		output  *string
		setting string
	}{{input.SpecPath, &specPath, macroSpecSetting}, {input.IssueSpecPath, &issueSpecPath, issueSpecSetting}} {
		if folder.input == nil {
			continue
		}
		var err error
		if *folder.output, err = normalizeSpecFolder(r.Context(), *folder.input, folder.setting); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	clonesPath := ""
	if input.ClonesPath != nil {
		if clonesPath = strings.TrimSpace(*input.ClonesPath); clonesPath != "" {
			if !filepath.IsAbs(clonesPath) {
				http.Error(w, "Clones folder must be an absolute path", 400)
				return
			}
			clonesPath = filepath.Clean(clonesPath)
		}
	}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	unlock := agentconfig.LockSettings()
	defer unlock()
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if input.statesEngine() {
		http.Error(w, agentconfig.ErrEngineFields.Error(), 400)
		return
	}
	project := input.apply(settings.Project(input.ProjectID))
	project.Path = input.Path
	if input.SpecPath != nil {
		project.MacroSpecPath = specPath
	}
	if input.IssueSpecPath != nil {
		project.IssueSpecPath = issueSpecPath
	}
	if input.AnyRepository != nil {
		// Off is the default, so it is stored as nothing.
		project.AnyRepository = nil
		if *input.AnyRepository {
			on := true
			project.AnyRepository = &on
		}
	}
	if input.ClonesPath != nil {
		project.ClonesPath = clonesPath
	}
	if err := agentconfig.ValidateProject(project); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := input.applyEngine(&settings); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	settings.SetProject(input.ProjectID, project)
	delete(settings.DisconnectedProjects, input.ProjectID)
	if err := agentconfig.WriteSettings(settings); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d.reportCapabilitiesLater()
	if !agentconfig.Resolve(config, settings).DropsSpecArtifacts() {
		clearSpecExclusions(r.Context(), config, settings, input.Path)
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
	unlock := agentconfig.LockSettings()
	defer unlock()
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if settings.DisconnectedProjects == nil {
		settings.DisconnectedProjects = map[string]bool{}
	}
	settings.DisconnectedProjects[id] = true
	// The whole section goes with the project, the specifications folder
	// included: a project added again starts from the inherited values, not
	// from what was chosen before. Its seed marker stays, so the server values
	// are not taken a second time.
	delete(settings.ProjectSettings, id)
	// It leaves the workstation Sandbox whitelist too (#730); emptied, the
	// whitelist covers every project again.
	var whitelist []string
	for _, listed := range settings.Defaults.ClaudeSandboxProjects {
		if listed != id {
			whitelist = append(whitelist, listed)
		}
	}
	settings.Defaults.ClaudeSandboxProjects = whitelist
	if err := agentconfig.WriteSettings(settings); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// The settings file generated from its sandbox values goes too (#700).
	if _, err := agentconfig.ClaudeSettingsFile(id, nil); err != nil {
		log.Printf("[Agent] Could not remove the Claude settings of project %s: %v", id, err)
	}
	d.reportCapabilitiesLater()
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

// normalizeSpecFolder validates a specifications folder typed or chosen in the
// desktop settings, the Macro or the Issue one, which setting names in a
// refusal. It must be an absolute path to an existing directory. A folder
// inside a Git checkout names that checkout: the macro or task worktree is
// created at its root, where specs/ is looked for. A folder outside any
// checkout is kept as it is. Empty clears the override.
func normalizeSpecFolder(ctx context.Context, raw, setting string) (string, error) {
	folder := strings.TrimSpace(raw)
	if folder == "" {
		return "", nil
	}
	if !filepath.IsAbs(folder) {
		return "", fmt.Errorf("The %s must be an absolute path", setting)
	}
	info, err := os.Stat(folder)
	if err != nil {
		return "", fmt.Errorf("The %s %s does not exist", setting, folder)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("The %s %s is not a directory", setting, folder)
	}
	if top, err := gitLocal(ctx, folder, "rev-parse", "--show-toplevel"); err == nil {
		return filepath.Clean(top), nil
	}
	return filepath.Clean(folder), nil
}

// specFolderKind says what the effective specifications folder is, for the
// desktop settings to show next to the field: "git" inside a Git checkout,
// "folder" for a plain directory, "missing" when it no longer exists, and
// "unset" when the project folder itself is not mapped here.
func specFolderKind(ctx context.Context, folder string) string {
	if strings.TrimSpace(folder) == "" {
		return "unset"
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return "missing"
	}
	if _, err := gitLocal(ctx, folder, "rev-parse", "--show-toplevel"); err == nil {
		return "git"
	}
	return "folder"
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
		effective := agentconfig.Resolve(config, overrides)
		section := overrides.Project(id)
		// The inherited folder follows the code checkout, on every project
		// (#484): only an override is stored, so a later change of the local
		// repository carries it along.
		specDefault := ""
		if mappingErr == nil {
			specDefault = root
		}
		specEffective := section.MacroSpecPath
		if strings.TrimSpace(specEffective) == "" {
			specEffective = specDefault
		}
		issueSpecEffective := section.IssueSpecPath
		if strings.TrimSpace(issueSpecEffective) == "" {
			issueSpecEffective = specDefault
		}
		fields := executionFields(config, overrides)
		sandboxCovered, sandboxGlobal := projectSandboxInheritance(overrides, id)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"server":                      withoutExecution(config),
			"path":                        root,
			"specPath":                    section.MacroSpecPath,
			"specDefault":                 specDefault,
			"specKind":                    specFolderKind(r.Context(), specEffective),
			"issueSpecPath":               section.IssueSpecPath,
			"issueSpecDefault":            specDefault,
			"issueSpecKind":               specFolderKind(r.Context(), issueSpecEffective),
			"anyRepository":               overrides.AnyRepository(id),
			"clonesPath":                  section.ClonesPath,
			"clonesDefault":               overrides.ClonesPath(id, specDefault),
			"useWorktrees":                effective.UseWorktrees,
			"configured":                  mappingErr == nil,
			"aiCommandTemplate":           effective.AICommandTemplate,
			"aiCommandTemplateAutonomous": effective.AICommandTemplateAutonomous,
			"defaultEngine":               summaryOf(overrides.ProjectEngine(id)),
			"worktreeOverride":            section.UseWorktrees != nil,
			"specArtifacts":               models.NormalizeSpecArtifacts(effective.SpecArtifacts),
			"specArtifactsOverride":       section.SpecArtifacts != "",
			"specArtifactsTracked":        trackedSpecArtifacts(r.Context(), root, mappingErr),
			"parallelism":                 agentconfig.ExecutionLimit(id, effective.UseWorktrees, overrides),
			"aiProvider":                  effective.AIProvider,
			"aiModel":                     effective.AIModel,
			"terminal":                    effective.ExternalTerminalCommand,
			"terminalOverride":            section.Terminal != "",
			"claudeSandbox":               claudeSandboxPayload(section.ClaudeSandbox),
			"claudeSandboxGlobal":         sandboxGlobal,
			"claudeSandboxCovered":        sandboxCovered,
			// Claude Code's sandbox does not run on Windows: only the rules apply.
			"platformSandbox":    runtime.GOOS != "windows",
			"claudeSettingsPath": claudeSettingsPathOf(id),
			"fields":             fields,
			"skills":             skillNames(config),
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
	switch action := r.URL.Query().Get("action"); action {
	case "initialize", "provider-skills":
		provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
		if provider == "" {
			settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			provider = settings.Defaults.InitializationProvider
			if provider == "" {
				provider = agentconfig.DefaultProvider
			}
		}
		if _, err := agentconfig.ResolveLocations(provider); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Attempt failures are structured so the UI preserves partial success.
		// provider-skills installs the skills alone, the MCP being registered
		// from the MCP connection settings.
		var result initializationResult
		if action == "provider-skills" {
			result, _ = installProviderSkills(root, config, provider)
		} else {
			result, _ = d.initializeProvider(root, config, provider)
		}
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
		// View "conversation" explicitly asks for an interactive launch in
		// Claude's structured view. The server never sees it: the agent keeps
		// it until the dispatch comes back. Without it, the dispatch follows
		// the workstation console view (#711). An engine it cannot honour, or
		// an autonomous launch, gets what it would have had without it.
		View string
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
	if input.View == "conversation" {
		d.conversationViews.mark(task.ID)
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
	var exited bool
	var exitedAt time.Time
	held := map[string]bool{}
	d.queue.mu.Lock()
	run := d.queue.runs[id]
	if run != nil {
		taskID, projectID = run.taskID, run.desktop.ProjectID
		exited = run.restored
		select {
		case <-run.exited:
			exited = true
		default:
		}
		// finishedAt is when the run was first seen to have exited, and this
		// read may be the first to see it.
		if exited && run.finishedAt.IsZero() && !run.restored {
			run.finishedAt = time.Now().UTC()
		}
		exitedAt = run.finishedAt
		for heldID := range d.queue.runs {
			held[heldID] = true
		}
	}
	d.queue.mu.Unlock()
	if run == nil {
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
	var activity, successor any
	for _, item := range activities {
		if item.ID == id && item.TaskID == taskID {
			// A remote run stores the record kind in SkillID ("remote_run") and the
			// launched skill in SkillName. The desktop matches the launched skill.
			activity = map[string]string{"id": item.ID, "taskId": item.TaskID, "skillId": item.SkillName, "status": item.Status}
			if next := skillSuccessor(item, activities, held, exited, exitedAt); next != nil {
				entry := map[string]any{"id": next.ID, "taskId": next.TaskID, "skillId": next.SkillName, "status": next.Status}
				if next.WaitingSince != nil {
					entry["waitingSince"] = next.WaitingSince
				}
				successor = entry
			}
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"activity": activity, "successor": successor, "task": map[string]any{"status": task.Status, "labels": task.Labels}})
}

// skillSuccessor finds the skill a console went on to run once the one it was
// launched for ended (#586). That skill's start_run cannot reuse the console's
// SECTILE_RUN_ID, which the server refuses once ended, so it records a new
// remote run carrying no link to the console. It is recognised by task and
// time: the newest remote run of the task created after the console's own
// activity ended and while its process was alive, and which is no execution
// the agent launched itself. A parallel session on the same task can be taken
// for it; the owner accepted that heuristic. own is the console's activity,
// held the ids of the executions the agent holds, exitedAt when the process
// was first seen to have exited.
func skillSuccessor(own models.TaskActivity, activities []models.TaskActivity, held map[string]bool, exited bool, exitedAt time.Time) *models.TaskActivity {
	if own.Status != "completed" && own.Status != "failed" && own.Status != "canceled" {
		return nil
	}
	// An exit whose time is unknown leaves nothing to tell a skill started in
	// the console from one started after it.
	if exited && exitedAt.IsZero() {
		return nil
	}
	var newest *models.TaskActivity
	for i := range activities {
		item := &activities[i]
		if item.SkillID != "remote_run" || item.TaskID != own.TaskID || held[item.ID] || !item.CreatedAt.After(own.CreatedAt) {
			continue
		}
		if exited && !item.CreatedAt.Before(exitedAt) {
			continue
		}
		if newest == nil || item.CreatedAt.After(newest.CreatedAt) {
			newest = item
		}
	}
	return newest
}

// resolveTerminalForProject determines the terminal emulator based on precedence:
// 1. Explicit call override (the terminal picked for this very action)
// 2. The agent's explicit --terminal flag
// 3. Project section of the workstation settings
// 4. Workstation defaults
// 5. Agent daemon default (d.terminal.app)
// 6. System auto-detection (detectDefaultTerminal())
// The server holds no terminal any more (#305).
func (d *agentDaemon) resolveTerminalForProject(ctx context.Context, projectID, override string) string {
	override = strings.TrimSpace(override)
	if override != "" {
		return override
	}
	if d.terminal.explicit && d.terminal.app != "" {
		return d.terminal.app
	}
	if settings, err := agentconfig.ReadSettings(d.localSettingsRoot()); err == nil {
		if terminal := settings.Terminal(projectID); terminal != "" {
			return terminal
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
	// The engine is the task's, as for the discussion the app opens (#690).
	config = agentconfig.ResolveTask(config, overrides, task.ID)

	root, _, err = primaryRoot(r.Context(), config, overrides, root, task)
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
		worktreeDir, pathErr := localTaskPath(r.Context(), root, task, config.BranchNameFormat)
		if pathErr != nil {
			http.Error(w, pathErr.Error(), http.StatusConflict)
			return
		}
		branch, _ = taskWorktreeBranch(task, config.BranchNameFormat)
		if info, err := os.Stat(worktreeDir); err == nil && info.IsDir() {
			workDir = worktreeDir
		}
	}

	// The same line the app's discussion runs: the task's engine, given the
	// project's folders and its Claude settings (#690). It is built before the
	// run is registered, so a refusal leaves nothing to release.
	specWorkspace := d.knownTaskSpecWorkspace(r.Context(), config, task, workDir, branch)
	folders := d.taskFolderMap(r.Context(), config, task, workDir, specWorkspace, false)
	claudeSettings, err := d.launchClaudeSettings(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	line, err := dispatchCommand(config, task.ID, input.SkillID, "", "", "", "", "", agentCommandContext{Task: task, Branch: branch, Directory: workDir, Tracker: config.IssueTracker, Repo: config.GithubRepo, AddDirs: folderMapDirs(folders), ClaudeSettings: claudeSettings})
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
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

	// Preparing until the line is typed: wrapRun only takes a run in that state.
	run.desktop = desktopRun{
		ID:               runID,
		TaskID:           task.ID,
		TaskKey:          task.Key,
		ProjectID:        input.ProjectID,
		Skill:            input.SkillID,
		SessionID:        runID,
		Directory:        workDir,
		Folders:          runFolders(workDir, folders),
		Branch:           branch,
		Status:           "preparing",
		CreatedAt:        time.Now().UTC(),
		ExternalTerminal: termChoice,
	}
	run.interactiveProvider = discussionProvider(config, input.SkillID)
	d.queue.mu.Unlock()
	release := func() {
		if d.terminal.manager != nil {
			_ = d.terminal.manager.CloseSession(runID)
		}
		d.queue.mu.Lock()
		// Closed so the run store's exit watcher lets go of it.
		run.once.Do(func() { close(run.exited) })
		delete(d.queue.runs, runID)
		d.queue.mu.Unlock()
		d.store.remove(runID)
	}

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
	if raw, err := json.Marshal(folders); err == nil && len(folders) > 0 {
		envVars["SECTILE_REPOSITORIES"] = string(raw)
	}
	for name, value := range taskSpecEnvironment(specWorkspace) {
		envVars[name] = value
	}

	if d.terminal.manager == nil {
		release()
		http.Error(w, "Failed to initialize PTY session: terminal manager unavailable", http.StatusInternalServerError)
		return
	}
	wrapped, err := d.wrapRun(task.ID, runID, line)
	if err != nil {
		release()
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err := d.runInPty(runID, workDir, envVars, wrapped); err != nil {
		release()
		http.Error(w, fmt.Sprintf("Failed to initialize PTY session: %v", err), http.StatusInternalServerError)
		return
	}
	d.queue.mu.Lock()
	// A fast engine may have already reported its exit.
	if run.desktop.Status == "preparing" {
		run.desktop.Status = "running"
	}
	d.queue.mu.Unlock()

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
