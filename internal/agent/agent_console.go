package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"tasks/internal/agentconfig"
)

// consoleRunKind marks a free console run, which holds no background worker capacity.
const consoleRunKind = "console"

// consoleCommand opens a built-in provider without an initial prompt.
func consoleCommand(provider, model string) (string, error) {
	switch provider {
	case "codex", "claude", "agy", "gemini", "vibe":
		return strings.TrimRight("exec "+provider+" "+strings.Join(agentconfig.ModelArgs(provider, model), " "), " "), nil
	case "cursor":
		return strings.TrimSpace("exec cursor agent " + strings.Join(agentconfig.ModelArgs(provider, model), " ")), nil
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
	command, err := consoleCommand(provider, agentconfig.ResolveModel(config, ""))
	if input.EngineID != "" && config.AICommandTemplate != "" {
		command, err = expandConfiguredTemplate(config.AICommandTemplate, config.AIModel, "", false, agentCommandContext{Directory: root})
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	go d.launchConsole(run, command)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(entry)
}

func (d *agentDaemon) launchConsole(run *controlledRun, command string) {
	err := d.awaitRunSlot(context.Background(), run)
	if err == nil && d.terminal.manager == nil {
		err = fmt.Errorf("terminal manager unavailable")
	}
	if err == nil {
		var wrapped string
		wrapped, err = d.wrapRun("", run.desktop.ID, command)
		if err == nil {
			// A free console carries no task, but it does have a run: it is the
			// one Sectile-launched kind that would otherwise be unable to report
			// that it is waiting for the user.
			env := map[string]string{
				"SECTILE_TASK_KEY": "", "SECTILE_TASK_ID": "", "SECTILE_RUN_ID": run.desktop.ID,
				"SECTILE_TASK_BRANCH": "", "SECTILE_TASK_WORKTREE": "", "SECTILE_REMOTE_MODE": "",
				"SECTILE_PROJECT_ID": run.desktop.ProjectID,
				"SECTILE_AGENT_URL":  d.link.serverURL, "SECTILE_SERVER_URL": d.link.serverURL,
				"SECTILE_AGENT_TOKEN":  d.link.token,
				"SECTILE_LOOPBACK_URL": d.loopback.url,
			}
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
