package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

const orchestrationControlsCapability = "orchestration-controls"
const orchestrationAgentsCapability = "orchestration-agents"

// orchestrationSessionsCapability says the agent starts and stops orchestration
// as an agent over the backlog (ADR 0064): POST names the agent to start,
// DELETE stops it, and GET carries the server's orchestration state.
const orchestrationSessionsCapability = "orchestration-sessions"

// desktopOrchestration keeps named agents on this workstation. Every agent
// launches through the same paired identity and server-owned project capacity.
func (d *agentDaemon) desktopOrchestration(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if id == "" {
		http.Error(w, "Project required", 400)
		return
	}
	var input struct {
		Enabled    *bool                             `json:"orchestratorEnabled"`
		Concurrent *int                              `json:"orchestratorMaxConcurrent"`
		Agents     *[]agentconfig.OrchestrationAgent `json:"agents"`
	}
	var launch struct {
		AgentID string `json:"agentId"`
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPut:
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || input.Enabled == nil || input.Concurrent == nil || input.Agents == nil || *input.Concurrent < 1 || *input.Concurrent > 10 {
			http.Error(w, "Enablement, concurrency from 1 to 10, and an agent list are required", 400)
			return
		}
	case http.MethodPost:
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&launch) != nil {
			http.Error(w, "Start accepts only an agent ID", 400)
			return
		}
	case http.MethodDelete:
		// Stop needs nothing local: the server disarms the project.
		d.relayServer(w, r, http.MethodPost, "/api/projects/"+url.PathEscape(id)+"/orchestrate/stop", "")
		return
	default:
		http.Error(w, "Method not allowed", 405)
		return
	}
	path := "/api/projects/" + url.PathEscape(id)
	var project models.Project
	if err := d.readAPI(r.Context(), path, &project); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if project.OrchestratorMaxConcurrent < 1 {
		http.Error(w, "Update the Sectile server to configure orchestration from desktop", 501)
		return
	}
	root, settings, err := d.localProjectRoot(r.Context(), agentconfig.Config{ProjectID: id, GitRemoteURL: project.GitRemoteUrl})
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	agents := settings.Project(id).OrchestrationAgentDefinitions()
	switch r.Method {
	case http.MethodGet:
		type agentView struct {
			agentconfig.OrchestrationAgent
			Guidelines      string `json:"guidelines"`
			GuidelinesError string `json:"guidelinesError,omitempty"`
		}
		views := make([]agentView, 0, len(agents))
		for _, agent := range agents {
			view := agentView{OrchestrationAgent: agent}
			view.Guidelines, _, err = readOrchestrationAgentGuidelines(root, agent)
			if err != nil {
				view.GuidelinesError = err.Error()
			}
			views = append(views, view)
		}
		// The server's state says which agent is started, its session and its
		// decisions. A server that predates ADR 0064 has none; the agents stay
		// configurable.
		var state *models.OrchestrationState
		if project.OrchestratorEnabled {
			var read models.OrchestrationState
			if d.readAPI(r.Context(), path+"/orchestrate", &read) == nil {
				state = &read
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"orchestratorEnabled": project.OrchestratorEnabled, "orchestratorMaxConcurrent": project.OrchestratorMaxConcurrent, "agents": views, "maxAgents": agentconfig.MaxOrchestrationAgents, "state": state})
	case http.MethodPut:
		agents, err = agentconfig.NormalizeOrchestrationAgents(*input.Agents)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if *input.Enabled && len(agents) == 0 {
			http.Error(w, "Add an agent before enabling orchestration", 400)
			return
		}
		for _, agent := range agents {
			if err := validateOrchestrationFile(agent.File); err != nil {
				http.Error(w, agent.Name+": "+err.Error(), 400)
				return
			}
			if *input.Enabled {
				if _, _, err := readOrchestrationAgentGuidelines(root, agent); err != nil {
					http.Error(w, agent.Name+": "+err.Error(), 400)
					return
				}
			}
		}
		// Save local definitions before enabling their server-owned project.
		if _, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			local := settings.Project(id)
			local.OrchestrationAgents = agents
			local.OrchestrationFile = ""
			settings.SetProject(id, local)
			return nil
		}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		// Removing the agent this workstation started stops orchestration: its
		// next session would have no policy to read (ADR 0064).
		var state models.OrchestrationState
		if d.readAPI(r.Context(), path+"/orchestrate", &state) == nil && state.Armed != nil && state.Armed.DeviceID == d.link.deviceID && state.Armed.ProfileID != "" &&
			!slices.ContainsFunc(agents, func(agent agentconfig.OrchestrationAgent) bool { return agent.ID == state.Armed.ProfileID }) {
			if err := d.postAPI(r.Context(), path+"/orchestrate/stop", struct{}{}, nil); err != nil {
				http.Error(w, "Stop the removed agent first: "+err.Error(), 502)
				return
			}
		}
		d.relayServer(w, r, http.MethodPut, path, mustJSON(models.UpdateProjectRequest{OrchestratorEnabled: input.Enabled, OrchestratorMaxConcurrent: input.Concurrent}))
	case http.MethodPost:
		// An omitted ID is unambiguous only for the former single-agent setup.
		if launch.AgentID == "" && len(agents) == 1 {
			launch.AgentID = agents[0].ID
		}
		var selected *agentconfig.OrchestrationAgent
		for i := range agents {
			if agents[i].ID == launch.AgentID {
				selected = &agents[i]
				break
			}
		}
		if selected == nil {
			http.Error(w, "Choose a saved orchestration agent", 400)
			return
		}
		// The file is read again at every session launch; reading it now
		// refuses to start an agent whose policy is already unreadable.
		if _, _, err := readOrchestrationAgentGuidelines(root, *selected); err != nil {
			http.Error(w, selected.Name+": "+err.Error(), 400)
			return
		}
		d.relayServer(w, r, http.MethodPost, path+"/orchestrate", mustJSON(models.OrchestrateProjectRequest{ProfileID: selected.ID, ProfileName: selected.Name}))
	}
}

// The preview, enabling and run paths all validate the same named snapshot.
func readOrchestrationAgentGuidelines(root string, agent agentconfig.OrchestrationAgent) (content, snapshot string, err error) {
	content, err = readOrchestrationGuidelines(root, agent.File)
	if err != nil {
		return content, "", err
	}
	snapshot = fmt.Sprintf("Orchestration agent: %s [%s]\n\n%s", agent.Name, agent.ID, content)
	if len(snapshot) > models.OrchestratorGuidelinesMaxBytes {
		return content, "", fmt.Errorf("agent name and guidelines exceed 32 KiB")
	}
	return content, snapshot, nil
}

// validateOrchestrationFile keeps guidelines inside the mapped checkout.
func validateOrchestrationFile(file string) error {
	if !filepath.IsLocal(file) || strings.ToLower(filepath.Ext(file)) != ".md" {
		return fmt.Errorf("Choose a Markdown (.md) file inside this project's local folder")
	}
	return nil
}

func readOrchestrationGuidelines(root, file string) (string, error) {
	if err := validateOrchestrationFile(file); err != nil {
		return "", err
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return "", fmt.Errorf("Open local project: %w", err)
	}
	defer directory.Close()
	info, err := directory.Stat(file)
	if err != nil {
		return "", fmt.Errorf("Read guidelines %s: %w", file, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Guidelines %s must be a regular Markdown file", file)
	}
	// Root.Open refuses symlinks that escape the checkout as well as traversal.
	source, err := directory.Open(file)
	if err != nil {
		return "", fmt.Errorf("Read guidelines %s: %w", file, err)
	}
	defer source.Close()
	info, err = source.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Guidelines %s must be a regular Markdown file", file)
	}
	content, err := io.ReadAll(io.LimitReader(source, models.OrchestratorGuidelinesMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("Read guidelines %s: %w", file, err)
	}
	if len(content) > models.OrchestratorGuidelinesMaxBytes {
		return "", fmt.Errorf("Guidelines %s exceed 32 KiB", file)
	}
	if !utf8.Valid(content) || strings.TrimSpace(string(content)) == "" {
		return "", fmt.Errorf("Guidelines %s must contain UTF-8 Markdown text", file)
	}
	return string(content), nil
}
