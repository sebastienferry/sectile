package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tasks/internal/agentconfig"
)

type conversationProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Model     string `json:"model,omitempty"`
	Mode      string `json:"mode,omitempty"`
	CanDelete bool   `json:"canDelete"`
}

func simpleConversationEngine(e agentconfig.Engine) bool {
	return (e.Provider == "claude" || e.Provider == "codex") && e.Command == "" && e.CommandAutonomous == ""
}

func conversationProfilesView(settings agentconfig.Settings, projectID string) map[string]any {
	profiles := []conversationProfile{}
	for _, e := range settings.Engines.Catalogue {
		if !simpleConversationEngine(e) {
			continue
		}
		removable := projectID == "" || e.ID != settings.Engines.Default
		if projectID != "" {
			for project, id := range settings.Engines.Projects {
				if project != projectID && !settings.DisconnectedProjects[project] && id == e.ID {
					removable = false
				}
			}
			for _, id := range settings.Engines.Tasks {
				if id == e.ID {
					removable = false
				}
			}
		}
		profiles = append(profiles, conversationProfile{ID: e.ID, Name: e.Name, Provider: e.Provider, Model: e.Model, Mode: e.ConversationMode, CanDelete: removable})
	}
	selected := settings.DefaultEngine()
	inherited := false
	if projectID != "" {
		selected = settings.ProjectEngine(projectID)
		inherited = settings.Engines.Projects[projectID] == ""
	}
	defaultID := ""
	if simpleConversationEngine(selected) {
		defaultID = selected.ID
	}
	return map[string]any{"profiles": profiles, "defaultId": defaultID, "inherited": inherited}
}

func (d *agentDaemon) desktopConversationProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID != "" {
		if _, err := d.fetchConfig(r.Context(), projectID, ""); err != nil {
			http.Error(w, "Project unavailable", http.StatusBadGateway)
			return
		}
	}
	var input struct {
		Action      string `json:"action"`
		ID          string `json:"id"`
		Name        string `json:"name"`
		Provider    string `json:"provider"`
		Model       string `json:"model"`
		Mode        string `json:"mode"`
		MakeDefault bool   `json:"makeDefault"`
	}
	if r.Method == http.MethodPost {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			http.Error(w, "Invalid profile request", 400)
			return
		}
		d.prepareMu.Lock()
		_, err := d.updateSettings(func(settings *agentconfig.Settings) error {
			list := append([]agentconfig.Engine(nil), settings.Engines.Catalogue...)
			switch input.Action {
			case "select":
				if input.ID != "" {
					e, ok := settings.Engine(input.ID)
					if !ok || !simpleConversationEngine(e) {
						return fmt.Errorf("profile unavailable")
					}
				}
				if projectID == "" {
					if input.ID == "" {
						return fmt.Errorf("workstation default profile required")
					}
					return settings.ReplaceCatalogue(list, input.ID)
				}
				settings.SetProjectEngine(projectID, input.ID)
				return nil
			case "save":
				// The first project profile must not replace an implicit workstation default.
				if len(list) == 0 {
					existing := settings.DefaultEngine()
					if err := settings.ReplaceCatalogue([]agentconfig.Engine{existing}, existing.Name); err != nil {
						return err
					}
					list = append([]agentconfig.Engine(nil), settings.Engines.Catalogue...)
				}
				if input.Provider != "claude" && input.Provider != "codex" {
					return fmt.Errorf("unsupported engine")
				}
				if !agentconfig.ValidConversationMode(input.Mode) {
					return fmt.Errorf("unsupported conversation mode")
				}
				index := -1
				if input.ID != "" {
					for i, e := range list {
						if e.ID == input.ID {
							if !simpleConversationEngine(e) {
								return fmt.Errorf("this profile must be edited in the full engine editor")
							}
							index = i
						}
					}
					if index < 0 {
						return fmt.Errorf("this profile no longer exists")
					}
				}
				if index < 0 {
					list = append(list, agentconfig.Engine{})
					index = len(list) - 1
				}
				list[index].Name, list[index].Provider, list[index].Model, list[index].ConversationMode = input.Name, input.Provider, input.Model, input.Mode
				defaultID := settings.Engines.Default
				if defaultID == "" || (projectID == "" && input.MakeDefault) {
					defaultID = input.Name
				}
				if err := settings.ReplaceCatalogue(list, defaultID); err != nil {
					return err
				}
				if projectID != "" && input.MakeDefault {
					settings.SetProjectEngine(projectID, settings.Engines.Catalogue[index].ID)
				}
				return nil
			case "delete":
				found := false
				for _, p := range conversationProfilesView(*settings, projectID)["profiles"].([]conversationProfile) {
					if p.ID == input.ID {
						found = p.CanDelete
					}
				}
				if !found {
					return fmt.Errorf("profile unavailable or still used by another project, a task, or the workstation")
				}
				kept := []agentconfig.Engine{}
				for _, e := range list {
					if e.ID != input.ID {
						kept = append(kept, e)
					}
				}
				if len(kept) == 0 {
					settings.Engines = agentconfig.Engines{}
					settings.Seeded.DefaultEngine = ""
					return nil
				}
				defaultID := settings.Engines.Default
				if defaultID == input.ID || defaultID == "" {
					defaultID = kept[0].ID
				}
				return settings.ReplaceCatalogue(kept, defaultID)
			default:
				return fmt.Errorf("unknown profile action")
			}
		})
		d.prepareMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		d.reportCapabilitiesLater()
	}
	settings, err := d.readSettings()
	if err != nil {
		http.Error(w, "Could not read local profiles", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(conversationProfilesView(settings, projectID))
}
