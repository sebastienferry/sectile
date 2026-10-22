package agent

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"tasks/internal/models"
)

const orchestrationControlsCapability = "orchestration-controls"

// desktopOrchestration exposes only orchestration settings and the explicit
// evaluate action. The paired credential owns the upstream request; desktop
// payloads cannot select another actor or bypass the configured concurrency.
func (d *agentDaemon) desktopOrchestration(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if id == "" {
		http.Error(w, "Project required", 400)
		return
	}
	path := "/api/projects/" + url.PathEscape(id)
	switch r.Method {
	case http.MethodGet:
		var project models.Project
		if err := d.readAPI(r.Context(), path, &project); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		if project.OrchestratorMaxConcurrent < 1 {
			http.Error(w, "Update the Sectile server to configure orchestration from desktop", 501)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"orchestratorEnabled": project.OrchestratorEnabled, "orchestratorMaxConcurrent": project.OrchestratorMaxConcurrent, "orchestratorPrompt": project.OrchestratorPrompt})
	case http.MethodPut:
		var input struct {
			Enabled    *bool   `json:"orchestratorEnabled"`
			Concurrent *int    `json:"orchestratorMaxConcurrent"`
			Prompt     *string `json:"orchestratorPrompt"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || input.Enabled == nil || input.Concurrent == nil || input.Prompt == nil || *input.Concurrent < 1 || *input.Concurrent > 10 {
			http.Error(w, "Enablement, concurrency from 1 to 10, and guidelines are required", 400)
			return
		}
		d.relayServer(w, r, http.MethodPut, path, mustJSON(models.UpdateProjectRequest{OrchestratorEnabled: input.Enabled, OrchestratorMaxConcurrent: input.Concurrent, OrchestratorPrompt: input.Prompt}))
	case http.MethodPost:
		// No force or capacity override: Evaluate always respects saved settings.
		var input struct{}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			http.Error(w, "Evaluation accepts an empty JSON object", 400)
			return
		}
		d.relayServer(w, r, http.MethodPost, path+"/orchestrate", "{}")
	default:
		http.Error(w, "Method not allowed", 405)
	}
}
