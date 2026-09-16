package main

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

func consoleCommand(provider string) (string, error) {
	switch provider {
	case "codex", "claude":
		return "exec " + provider, nil
	default:
		return "", fmt.Errorf("select Codex or Claude")
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
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || strings.TrimSpace(input.ProjectID) == "" {
		http.Error(w, "Project and provider required", http.StatusBadRequest)
		return
	}
	command, err := consoleCommand(input.Provider)
	if err != nil {
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
	config = agentconfig.ApplyOverrides(config, overrides)
	d.runsMu.Lock()
	run, err := d.enqueueRunLocked("", agentconfig.Dispatch{RunID: id}, input.ProjectID, root, agentconfig.ExecutionLimit(input.ProjectID, config.UseWorktrees, overrides), false)
	if err != nil {
		d.runsMu.Unlock()
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	run.desktop.Kind, run.desktop.Provider = consoleRunKind, input.Provider
	entry := run.desktop
	d.runsMu.Unlock()
	// The daemon owns the execution after admission, independently of the request.
	go d.launchConsole(run, command)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(entry)
}

func (d *agentDaemon) launchConsole(run *controlledRun, command string) {
	err := d.awaitRunSlot(context.Background(), run)
	if err == nil && d.terminalMgr == nil {
		err = fmt.Errorf("terminal manager unavailable")
	}
	if err == nil {
		var wrapped string
		wrapped, err = d.wrapRun("", run.desktop.ID, command)
		if err == nil {
			env := map[string]string{
				"SECTILE_TASK_KEY": "", "SECTILE_TASK_ID": "", "SECTILE_RUN_ID": "",
				"SECTILE_TASK_BRANCH": "", "SECTILE_TASK_WORKTREE": "", "SECTILE_REMOTE_MODE": "",
				"SECTILE_PROJECT_ID": run.desktop.ProjectID,
				"SECTILE_AGENT_URL":  d.agentURL, "SECTILE_SERVER_URL": d.serverURL,
				"SECTILE_AGENT_TOKEN": d.loopbackToken,
			}
			_, err = d.terminalMgr.GetOrCreateSession(run.desktop.ID, run.root, env)
			if err == nil {
				d.runsMu.Lock()
				run.desktop.SessionID = run.desktop.ID
				d.runsMu.Unlock()
				err = d.runInPty(run.desktop.ID, run.root, env, wrapped)
			}
			if err == nil {
				d.runsMu.Lock()
				// A fast command may have already reported its exit.
				if run.desktop.Status == "preparing" {
					run.desktop.Status = "running"
				}
				d.runsMu.Unlock()
				return
			}
		}
	}
	if d.terminalMgr != nil {
		_ = d.terminalMgr.CloseSession(run.desktop.ID)
	}
	d.runsMu.Lock()
	run.desktop.SessionID = ""
	run.desktop.Status = "failed"
	if run.canceled || d.shuttingDown {
		run.desktop.Status = "canceled"
	}
	run.once.Do(func() { close(run.exited) })
	d.runsMu.Unlock()
}
