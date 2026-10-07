package agent

import (
	"encoding/json"
	"net/http"
	"tasks/internal/agentconfig"
)

const codexSettingsCapability = "codex-settings"

func (d *agentDaemon) workstationCodexReviewer() string {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err == nil && settings.Defaults.CodexApprovalsReviewer == "auto_review" {
		return "auto_review"
	}
	return "user"
}

// desktopCodexSettings stores Sectile's conversation overrides, leaving the
// user's native Codex configuration untouched.
func (d *agentDaemon) desktopCodexSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", 405)
		return
	}
	reviewer := d.workstationCodexReviewer()
	if r.Method == http.MethodPut {
		var input struct {
			ApprovalsReviewer string `json:"approvalsReviewer"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil || (input.ApprovalsReviewer != "user" && input.ApprovalsReviewer != "auto_review") {
			http.Error(w, "Invalid Codex approvals reviewer", 400)
			return
		}
		d.prepareMu.Lock()
		_, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			settings.Defaults.CodexApprovalsReviewer = ""
			if input.ApprovalsReviewer == "auto_review" {
				settings.Defaults.CodexApprovalsReviewer = input.ApprovalsReviewer
			}
			return nil
		})
		d.prepareMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		reviewer = input.ApprovalsReviewer
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"approvalsReviewer": reviewer})
}
