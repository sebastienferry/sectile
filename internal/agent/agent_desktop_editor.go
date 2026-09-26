package agent

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/runner"
)

// openEditorCapability is what /desktop/status announces once the agent opens
// an execution's worktree in the workstation editor (#535). A desktop
// connected to an agent without it hides the button.
const openEditorCapability = "open-editor"

// desktopOpenEditor opens a run's directory in the workstation editor. The
// desktop names the run, never the path: the directory comes from the run
// registry, so the renderer cannot point the editor at an arbitrary folder.
// Unlike the server's open_editor operation, it never falls back to `code`:
// the desktop only offers it once an editor is chosen.
func (d *agentDaemon) desktopOpenEditor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		RunID string `json:"runId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input); err != nil || strings.TrimSpace(input.RunID) == "" {
		http.Error(w, "Run ID required", http.StatusBadRequest)
		return
	}

	d.queue.mu.Lock()
	run := d.queue.runs[input.RunID]
	directory := ""
	if run != nil {
		directory = strings.TrimSpace(run.desktop.Directory)
	}
	d.queue.mu.Unlock()
	if run == nil {
		http.Error(w, "Run not found", http.StatusNotFound)
		return
	}
	if directory == "" {
		http.Error(w, "This execution has no worktree", http.StatusConflict)
		return
	}

	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	editor := settings.Editor()
	if editor == "" {
		http.Error(w, "No editor is configured in Settings", http.StatusConflict)
		return
	}
	// An exited run may outlive its worktree, removed at handoff: say so
	// rather than start an editor on nothing.
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		http.Error(w, "The worktree no longer exists: "+directory, http.StatusGone)
		return
	}

	open := d.openEditorFn
	if open == nil {
		open = runner.NewRunner().OpenInEditor
	}
	if err := open(editor, directory); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"editor": editor, "directory": directory})
}
