package agent

import (
	"encoding/json"
	"net/http"

	"tasks/internal/agentconfig"
)

// conversationModeCapability tells Desktop this agent keeps a workstation
// default permission mode for Claude conversations, so that the first turn of
// a new conversation (a skill launch, a project prompt or a Claude chat) runs
// in it. Desktop only offers the setting to an agent that announces it.
const conversationModeCapability = "conversation-mode-default"

// desktopConversationMode reads and replaces the workstation default
// permission mode of Claude conversations. Desktop gives it its setting on
// each connection and on each change; the agent keeps it in its own settings
// so that it outlives a restart and applies to launches the web app started.
func (d *agentDaemon) desktopConversationMode(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"mode": d.workstationConversationMode()})
	case http.MethodPut:
		var input struct {
			Mode string `json:"mode"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			http.Error(w, "Invalid permission mode", http.StatusBadRequest)
			return
		}
		// The composer's own list, never bypassPermissions.
		if !conversationModes[input.Mode] {
			http.Error(w, "Unknown permission mode", http.StatusBadRequest)
			return
		}
		d.prepareMu.Lock()
		settings, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			// acceptEdits is the default: an absent key says it.
			settings.Defaults.ConversationMode = ""
			if input.Mode != conversationMode("") {
				settings.Defaults.ConversationMode = input.Mode
			}
			return nil
		})
		d.prepareMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"mode": conversationMode(settings.Defaults.ConversationMode)})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// workstationConversationMode is the permission mode a new conversation's
// first turn runs in. It is read when the conversation is created, so a change
// applies to the next one. A value the agent does not accept (bypassPermissions
// included, should someone write it by hand) and an unreadable settings file
// both mean acceptEdits.
func (d *agentDaemon) workstationConversationMode() string {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return conversationMode("")
	}
	return conversationMode(settings.Defaults.ConversationMode)
}
