package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// desktopRepository is one repository of a project as the desktop shows it:
// the remote, and the folder that holds its checkout on this workstation.
type desktopRepository struct {
	URL      string `json:"url"`
	Identity string `json:"identity"`
	Path     string `json:"path"`
	Code     bool   `json:"code"`
}

// desktopRepositories serves the repository mappings of a project (#456).
//
// GET ?projectId= lists the project's repositories with their folder here.
// POST {projectId, repository, path, taskId} maps a repository to a folder,
// once the folder's origin is confirmed to be that repository ("" clears the
// mapping), and pins a ticket to it when taskId is given, which is how a
// launch waiting for its repository is resumed from the desktop.
func (d *agentDaemon) desktopRepositories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
		config, err := d.fetchConfig(r.Context(), projectID, "")
		if err != nil || projectID == "" {
			http.Error(w, "Project required", 400)
			return
		}
		root, overrides, _ := d.localProjectRoot(r.Context(), config)
		code := codeIdentity(config)
		list := []desktopRepository{}
		for _, repository := range projectRepositories(config) {
			path, _ := repositoryRoot(overrides, root, code, repository.Identity)
			list = append(list, desktopRepository{URL: repository.URL, Identity: repository.Identity, Path: path, Code: repository.Identity == code})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var input struct {
			ProjectID  string  `json:"projectId"`
			Repository string  `json:"repository"`
			Path       *string `json:"path"`
			TaskID     string  `json:"taskId"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || strings.TrimSpace(input.ProjectID) == "" {
			http.Error(w, "Project and repository required", 400)
			return
		}
		config, err := d.fetchConfig(r.Context(), input.ProjectID, "")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		repository, ok := models.FindProjectRepository(projectRepositories(config), input.Repository)
		if !ok {
			http.Error(w, fmt.Sprintf("%s is not one of the project's repositories", strings.TrimSpace(input.Repository)), 400)
			return
		}
		if input.Path != nil {
			if status, err := d.mapRepository(r, repository, strings.TrimSpace(*input.Path)); err != nil {
				http.Error(w, err.Error(), status)
				return
			}
		}
		if taskID := strings.TrimSpace(input.TaskID); taskID != "" {
			if err := d.patchTask(r.Context(), taskID, map[string]string{"repository": repository.Identity}); err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "Method not allowed", 405)
	}
}

// mapRepository records, or clears when path is empty, the folder of one
// repository on this workstation. A folder whose checkout names another
// remote is refused, naming both: a wrong mapping would run a ticket in the
// wrong repository without a word.
func (d *agentDaemon) mapRepository(r *http.Request, repository models.ProjectRepository, path string) (int, error) {
	if path != "" {
		if !filepath.IsAbs(path) {
			return 400, fmt.Errorf("The folder of %s must be an absolute path", repository.Identity)
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			return 400, fmt.Errorf("The folder %s does not exist", path)
		}
		top, err := gitLocal(r.Context(), path, "rev-parse", "--show-toplevel")
		if err != nil {
			return 400, fmt.Errorf("%s is not a Git checkout", path)
		}
		remote, err := gitLocal(r.Context(), top, "remote", "get-url", "origin")
		if err != nil || models.RepositoryIdentity(remote) != repository.Identity {
			if strings.TrimSpace(remote) == "" {
				remote = "no origin"
			}
			return 400, fmt.Errorf("%s is a checkout of %s, not of %s", path, strings.TrimSpace(remote), repository.Identity)
		}
		path = filepath.Clean(top)
	}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	overrides, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return 500, err
	}
	if path == "" {
		delete(overrides.Repositories, repository.Identity)
	} else {
		if overrides.Repositories == nil {
			overrides.Repositories = map[string]string{}
		}
		overrides.Repositories[repository.Identity] = path
	}
	if err := agentconfig.WriteSettings(overrides); err != nil {
		return 500, err
	}
	return 0, nil
}
