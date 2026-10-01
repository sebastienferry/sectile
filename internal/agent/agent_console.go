package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// consoleRunKind marks a free console run, which holds no background worker capacity.
const consoleRunKind = "console"

// consoleCommand opens a built-in provider without an initial prompt.
func consoleCommand(provider, model string) (string, error) {
	switch provider {
	case "codex", "claude", "agy":
		return strings.TrimRight("exec "+provider+" "+strings.Join(agentconfig.ModelArgs(provider, model), " "), " "), nil
	default:
		return "", fmt.Errorf("select a supported AI engine")
	}
}

// desktopConsole admits a taskless, prompt-free CLI in the mapped checkout.
func (d *agentDaemon) desktopConsole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		ProjectID string `json:"projectId"`
		Provider  string `json:"provider"`
		EngineID  string `json:"engineId"`
		// View "conversation" asks for Claude's structured view instead of a
		// PTY. It is a preference: an engine it cannot honour gets the PTY.
		View string `json:"view"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || strings.TrimSpace(input.ProjectID) == "" {
		http.Error(w, "Project and AI engine required", http.StatusBadRequest)
		return
	}
	// The provider is checked before anything is fetched; the command itself is
	// built once the overrides are applied and the model is known.
	if _, err := consoleCommand(input.Provider, ""); input.EngineID == "" && err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	config, err := d.fetchConfig(r.Context(), input.ProjectID, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root, overrides, err := d.localProjectRoot(r.Context(), config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	id := uuid.NewString()
	var engine agentconfig.Engine
	if input.EngineID != "" {
		var found bool
		engine, found = overrides.Engine(input.EngineID)
		if !found {
			http.Error(w, "This engine is no longer in the catalogue", http.StatusNotFound)
			return
		}
		// This choice applies to this launch only.
		overrides.SetProjectEngine(input.ProjectID, engine.ID)
	}
	config = agentconfig.Resolve(config, overrides)
	provider := input.Provider
	if input.EngineID != "" {
		provider = config.AIProvider
	}
	// The console is given the project's folders at launch (#676); a
	// conversation reads them again at each turn instead.
	folders := buildFolderMap(r.Context(), config, overrides, root, codeIdentity(config), root, models.Task{})
	template := ""
	if input.EngineID != "" {
		template = config.AICommandTemplate
	}
	command, err := consoleLaunch(provider, agentconfig.ResolveModel(config, ""), template, config.AIModel, root, folderMapDirs(folders))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// A Claude engine with a launch template still converses: the template is
	// not run, only its model is kept, and the first notice says so.
	if input.View == "conversation" && provider == "claude" {
		d.queue.mu.Lock()
		run, err := d.newConversationLocked(input.ProjectID, root, conversationModel(config), conversationOrigin(config, "It runs in this project's local repository."))
		if err != nil {
			d.queue.mu.Unlock()
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		run.desktop.EngineID, run.desktop.EngineName = engine.ID, engine.Name
		entry := run.desktop
		d.queue.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(entry)
		return
	}
	d.queue.mu.Lock()
	run, err := d.enqueueRunLocked("", agentconfig.Dispatch{RunID: id}, input.ProjectID, root, agentconfig.ExecutionLimit(input.ProjectID, config.UseWorktrees, overrides), false)
	if err != nil {
		d.queue.mu.Unlock()
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	run.desktop.Kind, run.desktop.Provider = consoleRunKind, provider
	run.desktop.EngineID, run.desktop.EngineName = engine.ID, engine.Name
	run.desktop.Model = config.AIModel
	entry := run.desktop
	d.queue.mu.Unlock()
	// The daemon owns the execution after admission, independently of the request.
	go d.launchConsole(run, command, folders)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(entry)
}

// consoleLaunch is the command line of a free console: a built-in engine
// with its model, or the engine's own template, given the project's folders
// through the engine's attested option, or the template's {addDirs} slot.
func consoleLaunch(provider, model, template, templateModel, root string, dirs []string) (string, error) {
	if template != "" {
		return expandConfiguredTemplate(template, templateModel, "", false, agentCommandContext{Directory: root, AddDirs: dirs})
	}
	command, err := consoleCommand(provider, model)
	if err != nil {
		return "", err
	}
	return words(command, addDirArgs(provider, dirs)), nil
}

// consoleEnv is the environment of a free console. It carries no task, but it
// does have a run: it is the one Sectile-launched kind that would otherwise be
// unable to report that it is waiting for the user. It carries the project's
// folder map, as a skill run does.
func (d *agentDaemon) consoleEnv(run *controlledRun, folders []models.FolderMapEntry) map[string]string {
	env := map[string]string{
		"SECTILE_TASK_KEY": "", "SECTILE_TASK_ID": "", "SECTILE_RUN_ID": run.desktop.ID,
		"SECTILE_TASK_BRANCH": "", "SECTILE_TASK_WORKTREE": "", "SECTILE_REMOTE_MODE": "",
		"SECTILE_PROJECT_ID": run.desktop.ProjectID,
		"SECTILE_AGENT_URL":  d.link.serverURL, "SECTILE_SERVER_URL": d.link.serverURL,
		"SECTILE_AGENT_TOKEN":  d.link.token,
		"SECTILE_LOOPBACK_URL": d.loopback.url,
	}
	if raw, err := json.Marshal(folders); err == nil && len(folders) > 0 {
		env["SECTILE_REPOSITORIES"] = string(raw)
	}
	return env
}

func (d *agentDaemon) launchConsole(run *controlledRun, command string, folders []models.FolderMapEntry) {
	err := d.awaitRunSlot(context.Background(), run)
	if err == nil && d.terminal.manager == nil {
		err = fmt.Errorf("terminal manager unavailable")
	}
	if err == nil {
		var wrapped string
		wrapped, err = d.wrapRun("", run.desktop.ID, command)
		if err == nil {
			env := d.consoleEnv(run, folders)
			_, err = d.terminal.manager.GetOrCreateSession(run.desktop.ID, run.root, env)
			if err == nil {
				d.queue.mu.Lock()
				run.desktop.SessionID = run.desktop.ID
				d.queue.mu.Unlock()
				err = d.runInPty(run.desktop.ID, run.root, env, wrapped)
			}
			if err == nil {
				d.queue.mu.Lock()
				// A fast command may have already reported its exit.
				if run.desktop.Status == "preparing" {
					run.desktop.Status = "running"
				}
				d.queue.mu.Unlock()
				return
			}
		}
	}
	if d.terminal.manager != nil {
		_ = d.terminal.manager.CloseSession(run.desktop.ID)
	}
	d.queue.mu.Lock()
	run.desktop.SessionID = ""
	run.desktop.Status = "failed"
	if run.canceled || d.queue.shuttingDown {
		run.desktop.Status = "canceled"
	}
	run.once.Do(func() { close(run.exited) })
	d.queue.mu.Unlock()
}
