package agent

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
)

// projectTerminalCapability is what /desktop/status announces once the agent
// opens a terminal on a project's local repository (#761).
const projectTerminalCapability = "project-terminal"

// desktopProjectTerminal opens a plain terminal window on a project's local
// repository, running the user's own shell, in the terminal the project is set
// to use. The request names the project only: the folder is the one Project
// prompt opens in, resolved here. The window is visible on purpose, the user
// asked for it from the project's menu.
//
// POST {projectId} answers {opened, terminal, directory}, or the reason of a
// refusal.
func (d *agentDaemon) desktopProjectTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		ProjectID string `json:"projectId"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || strings.TrimSpace(input.ProjectID) == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}
	config, err := d.fetchConfig(r.Context(), input.ProjectID, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	directory, _, err := d.localProjectRoot(r.Context(), config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	terminal := d.resolveTerminalForProject(r.Context(), input.ProjectID, "")
	if err := d.openDirectoryTerminal(runtime.GOOS, terminal, directory); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"opened": true, "terminal": terminal, "directory": directory})
}
