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
// POST {projectId, repository, path} maps a repository to a folder, once the
// folder's origin is confirmed to be that repository ("" clears the mapping).
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
	unlock := agentconfig.LockSettings()
	defer unlock()
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

// desktopFolder is a folder attached to a project on this workstation (#484),
// as the desktop settings list it. Duplicate names the project repository
// whose own folder here wins over this entry, since the folder's remote became
// that repository after it was attached.
type desktopFolder struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Remote    string `json:"remote"`
	Identity  string `json:"identity"`
	Duplicate string `json:"duplicate,omitempty"`
}

// desktopFolders serves the folders attached to a project (#484). They live in
// this workstation's settings only: no request to the server carries a path.
//
// GET ?projectId= lists them, each read from the disk.
// POST {projectId, path} attaches one, or maps it when it is a checkout of one
// of the project's repositories, which the answer says with {mappedAs}.
// DELETE ?projectId=&path= detaches one, whether or not it still exists.
func (d *agentDaemon) desktopFolders(w http.ResponseWriter, r *http.Request) {
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
		list := []desktopFolder{}
		for _, folder := range attachedFolders(r.Context(), overrides, projectID) {
			entry := desktopFolder{Path: folder.Stored, Kind: folder.Kind, Remote: folder.Remote, Identity: folder.Identity}
			if _, ok := models.FindProjectRepository(projectRepositories(config), folder.Identity); ok {
				if mapped, found := repositoryRoot(overrides, root, code, folder.Identity); found && !sameDirectory(mapped, folder.Path) {
					entry.Duplicate = folder.Identity
				}
			}
			list = append(list, entry)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var input struct {
			ProjectID string `json:"projectId"`
			Path      string `json:"path"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || strings.TrimSpace(input.ProjectID) == "" {
			http.Error(w, "Project and folder required", 400)
			return
		}
		config, err := d.fetchConfig(r.Context(), input.ProjectID, "")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		mappedAs, status, err := d.attachFolder(r, config, strings.TrimSpace(input.Path))
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}
		if mappedAs == "" {
			w.WriteHeader(204)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"mappedAs": mappedAs})
	case http.MethodDelete:
		projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
		path := strings.TrimSpace(r.URL.Query().Get("path"))
		if projectID == "" || path == "" {
			http.Error(w, "Project and folder required", 400)
			return
		}
		if err := d.editFolders(projectID, func(folders []string) []string {
			kept := []string{}
			for _, folder := range folders {
				if filepath.Clean(folder) != filepath.Clean(path) {
					kept = append(kept, folder)
				}
			}
			return kept
		}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "Method not allowed", 405)
	}
}

// attachFolder checks a folder against what the project already has on this
// workstation, then attaches it, or maps it when it is a checkout of one of the
// project's repositories and returns that repository. A folder already known
// is refused naming what it already is.
func (d *agentDaemon) attachFolder(r *http.Request, config agentconfig.Config, path string) (string, int, error) {
	if !filepath.IsAbs(path) {
		return "", 400, fmt.Errorf("The folder must be an absolute path")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", 400, fmt.Errorf("The folder %s does not exist", path)
	}
	path = filepath.Clean(path)
	folder := describeFolder(r.Context(), path)
	d.prepareMu.Lock()
	root, overrides, _ := d.localProjectRoot(r.Context(), config)
	d.prepareMu.Unlock()
	same := func(other string) bool {
		other = strings.TrimSpace(other)
		return other != "" && (filepath.Clean(other) == path || sameDirectory(other, path) || sameDirectory(other, folder.Path))
	}
	code := codeIdentity(config)
	switch {
	case same(root):
		return "", 409, fmt.Errorf("%s is already the project's local repository", path)
	case same(overrides.SpecPath(config.ProjectID)):
		return "", 409, fmt.Errorf("%s is already the project's specifications folder", path)
	case folder.Identity != "" && folder.Identity == code:
		return "", 409, fmt.Errorf("%s is a checkout of the project's own repository %s: set it as the local repository instead", path, code)
	}
	for _, repository := range projectRepositories(config) {
		if mapped, ok := overrides.Repositories[repository.Identity]; ok && same(mapped) {
			return "", 409, fmt.Errorf("%s is already the folder of %s", path, repository.Identity)
		}
	}
	for _, attached := range attachedFolders(r.Context(), overrides, config.ProjectID) {
		if same(attached.Stored) || (attached.Kind == folderKindGit && same(attached.Path)) {
			return "", 409, fmt.Errorf("%s is already attached", path)
		}
		if folder.Identity != "" && attached.Identity == folder.Identity {
			return "", 409, fmt.Errorf("%s is already attached for %s", attached.Stored, folder.Identity)
		}
	}
	if repository, ok := models.FindProjectRepository(projectRepositories(config), folder.Identity); ok {
		if mapped := strings.TrimSpace(overrides.Repositories[repository.Identity]); mapped != "" {
			return "", 409, fmt.Errorf("%s is already mapped to %s", repository.Identity, mapped)
		}
		if status, err := d.mapRepository(r, repository, folder.Path); err != nil {
			return "", status, err
		}
		return repository.Identity, 0, nil
	}
	if err := d.editFolders(config.ProjectID, func(folders []string) []string { return append(folders, path) }); err != nil {
		return "", 500, err
	}
	return "", 0, nil
}

// editFolders rewrites the list of folders attached to a project, under the
// settings lock. An emptied list leaves the settings file.
func (d *agentDaemon) editFolders(projectID string, edit func([]string) []string) error {
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	unlock := agentconfig.LockSettings()
	defer unlock()
	overrides, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return err
	}
	section := overrides.Project(projectID)
	section.Folders = edit(section.Folders)
	if len(section.Folders) == 0 {
		section.Folders = nil
	}
	overrides.SetProject(projectID, section)
	return agentconfig.WriteSettings(overrides)
}
