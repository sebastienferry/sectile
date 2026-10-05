package agent

import (
	"encoding/json"
	"net/http"

	"tasks/internal/agentconfig"
)

// consoleViewCapability tells Desktop this agent keeps a workstation console
// view (#711), so an interactive launch the web app started follows Desktop's
// Claude consoles setting too.
const consoleViewCapability = "console-view-default"

// desktopConsoleView reads and replaces the workstation console view. Desktop
// gives it its setting on each connection and on each change; the agent keeps
// it in its own settings so that it outlives a restart and a closed Desktop.
func (d *agentDaemon) desktopConsoleView(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"view": settings.Defaults.ConsoleViewOrDefault()})
	case http.MethodPut:
		var input struct {
			View string `json:"view"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			http.Error(w, "Invalid console view", 400)
			return
		}
		if input.View != agentconfig.ConsoleViewTerminal && input.View != agentconfig.ConsoleViewConversation {
			http.Error(w, "Unknown console view", 400)
			return
		}
		d.prepareMu.Lock()
		settings, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			// The terminal is the default: an absent key says it.
			settings.Defaults.ConsoleView = ""
			if input.View == agentconfig.ConsoleViewConversation {
				settings.Defaults.ConsoleView = input.View
			}
			return nil
		})
		d.prepareMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"view": settings.Defaults.ConsoleViewOrDefault()})
	default:
		http.Error(w, "Method not allowed", 405)
	}
}

// workstationConsoleView is the console view a dispatch without an explicit
// request opens in. It is read at each dispatch, so a change applies to the
// next one; an unreadable settings file means the terminal.
func (d *agentDaemon) workstationConsoleView() string {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return agentconfig.ConsoleViewTerminal
	}
	return settings.Defaults.ConsoleViewOrDefault()
}

// opensConversation decides whether an interactive dispatch runs in Claude's
// conversation view: Desktop marked the launch for it, or nothing marked it and
// the workstation console view is the conversation. An autonomous launch, an
// open_terminal and an engine the conversation cannot honour keep what they had.
func opensConversation(autonomous bool, action string, marked bool, view string, config agentconfig.Config) bool {
	if autonomous || action == "open_terminal" {
		return false
	}
	return (marked || view == agentconfig.ConsoleViewConversation) && conversationDiscussionEngine(config)
}
