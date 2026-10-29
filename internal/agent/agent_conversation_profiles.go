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
		removable := e.ID != settings.Engines.Default
		for project, id := range settings.Engines.Projects {
			if project != projectID && id == e.ID {
				removable = false
			}
		}
		for _, id := range settings.Engines.Tasks {
			if id == e.ID {
				removable = false
			}
		}
		profiles = append(profiles, conversationProfile{ID: e.ID, Name: e.Name, Provider: e.Provider, Model: e.Model, Mode: e.ConversationMode, CanDelete: removable})
	}
	selected := settings.ProjectEngine(projectID)
	defaultID := ""
	if simpleConversationEngine(selected) {
		defaultID = selected.ID
	}
	return map[string]any{"profiles": profiles, "defaultId": defaultID, "inherited": settings.Engines.Projects[projectID] == ""}
}

func (d *agentDaemon) desktopConversationProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		http.Error(w, "Projet requis", 400)
		return
	}
	if _, err := d.fetchConfig(r.Context(), projectID, ""); err != nil {
		http.Error(w, "Projet indisponible", http.StatusBadGateway)
		return
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
			http.Error(w, "Demande de profil invalide", 400)
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
						return fmt.Errorf("profil indisponible")
					}
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
					return fmt.Errorf("moteur non pris en charge")
				}
				if input.Mode != "" && input.Mode != "plan" && input.Mode != "execute" {
					return fmt.Errorf("mode de conversation non pris en charge")
				}
				index := -1
				if input.ID != "" {
					for i, e := range list {
						if e.ID == input.ID {
							if !simpleConversationEngine(e) {
								return fmt.Errorf("ce profil doit être modifié dans l’éditeur complet des moteurs")
							}
							index = i
						}
					}
					if index < 0 {
						return fmt.Errorf("ce profil n’existe plus")
					}
				}
				if index < 0 {
					list = append(list, agentconfig.Engine{})
					index = len(list) - 1
				}
				list[index].Name, list[index].Provider, list[index].Model, list[index].ConversationMode = input.Name, input.Provider, input.Model, input.Mode
				defaultID := settings.Engines.Default
				if defaultID == "" {
					defaultID = input.Name
				}
				if err := settings.ReplaceCatalogue(list, defaultID); err != nil {
					return err
				}
				if input.MakeDefault {
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
					return fmt.Errorf("profil indisponible ou encore utilisé par un autre projet, une tâche ou le poste")
				}
				kept := []agentconfig.Engine{}
				for _, e := range list {
					if e.ID != input.ID {
						kept = append(kept, e)
					}
				}
				return settings.ReplaceCatalogue(kept, settings.Engines.Default)
			default:
				return fmt.Errorf("action de profil inconnue")
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
		http.Error(w, "Impossible de lire les profils locaux", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(conversationProfilesView(settings, projectID))
}
