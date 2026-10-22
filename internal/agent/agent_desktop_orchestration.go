package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"tasks/internal/agentconfig"
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
		root, settings, err := d.localProjectRoot(r.Context(), agentconfig.Config{ProjectID: id, GitRemoteURL: project.GitRemoteUrl})
		if err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		file := settings.Project(id).OrchestrationFile
		if file == "" {
			file = "AGENTS.md"
		}
		guidelines, fileErr := readOrchestrationGuidelines(root, file)
		view := map[string]any{"orchestratorEnabled": project.OrchestratorEnabled, "orchestratorMaxConcurrent": project.OrchestratorMaxConcurrent, "orchestrationFile": file, "guidelines": guidelines}
		if fileErr != nil {
			view["guidelinesError"] = fileErr.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(view)
	case http.MethodPut:
		var input struct {
			Enabled    *bool   `json:"orchestratorEnabled"`
			Concurrent *int    `json:"orchestratorMaxConcurrent"`
			File       *string `json:"orchestrationFile"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || input.Enabled == nil || input.Concurrent == nil || input.File == nil || *input.Concurrent < 1 || *input.Concurrent > 10 {
			http.Error(w, "Enablement, concurrency from 1 to 10, and a Markdown guidelines file are required", 400)
			return
		}
		var project models.Project
		if err := d.readAPI(r.Context(), path, &project); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		root, _, err := d.localProjectRoot(r.Context(), agentconfig.Config{ProjectID: id, GitRemoteURL: project.GitRemoteUrl})
		if err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		file := strings.TrimSpace(*input.File)
		if file == "" {
			file = "AGENTS.md"
		}
		if err := validateOrchestrationFile(file); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if *input.Enabled {
			if _, err := readOrchestrationGuidelines(root, file); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
		}
		// Save the workstation path first: a failed server write cannot enable
		// a project with a path that was never stored on this workstation.
		if _, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			local := settings.Project(id)
			local.OrchestrationFile = file
			settings.SetProject(id, local)
			return nil
		}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		d.relayServer(w, r, http.MethodPut, path, mustJSON(models.UpdateProjectRequest{OrchestratorEnabled: input.Enabled, OrchestratorMaxConcurrent: input.Concurrent}))
	case http.MethodPost:
		// No force or capacity override: Evaluate always respects saved settings.
		var input struct{}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			http.Error(w, "Evaluation accepts an empty JSON object", 400)
			return
		}
		var project models.Project
		if err := d.readAPI(r.Context(), path, &project); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		root, settings, err := d.localProjectRoot(r.Context(), agentconfig.Config{ProjectID: id, GitRemoteURL: project.GitRemoteUrl})
		if err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		file := settings.Project(id).OrchestrationFile
		if file == "" {
			file = "AGENTS.md"
		}
		guidelines, err := readOrchestrationGuidelines(root, file)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		d.relayServer(w, r, http.MethodPost, path+"/orchestrate", mustJSON(models.OrchestrateProjectRequest{Guidelines: &guidelines}))
	default:
		http.Error(w, "Method not allowed", 405)
	}
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
