package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// Saved board views on the desktop (#429). The views themselves are the
// server's; what this workstation adds is a folder per view, which launches
// made from the view run in, and which never leaves the workstation.

// viewRootMap remembers, per ticket, the folder of the view it was last
// launched from here, so that a relaunch, a next step or a stage the server
// chains keeps running where the launch from the view started. It lives as
// long as the agent: after a restart, the desktop sends the view again.
type viewRootMap struct {
	mu    sync.Mutex
	roots map[string]string
}

func (m *viewRootMap) get(taskID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.roots[strings.TrimSpace(taskID)]
}

// set records root for the ticket, or forgets it when root is empty, and
// returns what it replaced so that a refused launch can put it back.
func (m *viewRootMap) set(taskID, root string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	taskID = strings.TrimSpace(taskID)
	previous := m.roots[taskID]
	if root == "" {
		delete(m.roots, taskID)
		return previous
	}
	if m.roots == nil {
		m.roots = map[string]string{}
	}
	m.roots[taskID] = root
	return previous
}

// desktopView is a server view with this workstation's folder for it.
type desktopView struct {
	models.BoardView
	Directory string `json:"directory"`
}

// desktopViews serves the signed-in user's saved views.
//
// GET lists them with their folder here. POST {viewId, path} sets the folder
// of one view ("" clears it) once it is confirmed to be a Git checkout, and
// answers a warning, not a refusal, when its origin is not the repository the
// view declares.
func (d *agentDaemon) desktopViews(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var views []models.BoardView
		if err := d.readAPI(r.Context(), "/api/me/board-views", &views); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		overrides, err := agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		list := []desktopView{}
		for _, view := range views {
			list = append(list, desktopView{BoardView: view, Directory: overrides.ViewDirectories[view.ID]})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		var input struct {
			ViewID string `json:"viewId"`
			Path   string `json:"path"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || strings.TrimSpace(input.ViewID) == "" {
			http.Error(w, "View required", 400)
			return
		}
		view, err := d.readView(r.Context(), input.ViewID)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		path, warning, status, err := viewDirectory(r.Context(), view, strings.TrimSpace(input.Path))
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}
		if err := d.saveViewDirectory(view.ID, path); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"directory": path, "warning": warning})
	default:
		http.Error(w, "Method not allowed", 405)
	}
}

func (d *agentDaemon) readView(ctx context.Context, viewID string) (models.BoardView, error) {
	var view models.BoardView
	err := d.readAPI(ctx, "/api/me/board-views/"+url.PathEscape(strings.TrimSpace(viewID)), &view)
	return view, err
}

// viewDirectory checks the folder chosen for a view and returns the top of its
// checkout. A checkout of another repository than the view's is kept, with a
// warning naming both: the user may know better, and says so by choosing it.
func viewDirectory(ctx context.Context, view models.BoardView, path string) (string, string, int, error) {
	if path == "" {
		return "", "", 0, nil
	}
	if !filepath.IsAbs(path) {
		return "", "", 400, fmt.Errorf("The folder of the view %s must be an absolute path", view.Name)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", "", 400, fmt.Errorf("The folder %s does not exist", path)
	}
	top, err := gitLocal(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", 400, fmt.Errorf("%s is not a Git checkout", path)
	}
	top = filepath.Clean(top)
	warning := ""
	if declared := strings.TrimSpace(view.Repository); declared != "" {
		remote, err := gitLocal(ctx, top, "remote", "get-url", "origin")
		if err != nil || models.RepositoryIdentity(remote) != models.RepositoryIdentity(declared) {
			if strings.TrimSpace(remote) == "" {
				remote = "no origin"
			}
			warning = fmt.Sprintf("%s is a checkout of %s, while the view %s declares %s", top, strings.TrimSpace(remote), view.Name, models.RepositoryIdentity(declared))
		}
	}
	return top, warning, 0, nil
}

func (d *agentDaemon) saveViewDirectory(viewID, path string) error {
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	overrides, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return err
	}
	if path == "" {
		delete(overrides.ViewDirectories, viewID)
	} else {
		if overrides.ViewDirectories == nil {
			overrides.ViewDirectories = map[string]string{}
		}
		overrides.ViewDirectories[viewID] = path
	}
	return agentconfig.WriteSettings(overrides)
}

// viewLaunchRoot is the folder a launch from the view runs in: the view's
// folder here, else this workstation's checkout of the view's repository
// (#456 mappings), else "" for the project's own resolution.
func (d *agentDaemon) viewLaunchRoot(ctx context.Context, viewID string) (string, error) {
	overrides, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return "", err
	}
	if folder := strings.TrimSpace(overrides.ViewDirectories[viewID]); folder != "" {
		if _, err := gitLocal(ctx, folder, "rev-parse", "--show-toplevel"); err != nil {
			return "", fmt.Errorf("the folder %s of this view is no longer a Git checkout; choose it again", folder)
		}
		return folder, nil
	}
	view, err := d.readView(ctx, viewID)
	if err != nil {
		return "", err
	}
	if declared := strings.TrimSpace(view.Repository); declared != "" {
		if folder := strings.TrimSpace(overrides.Repositories[models.RepositoryIdentity(declared)]); folder != "" {
			return folder, nil
		}
	}
	return "", nil
}

// desktopViewTasks lists the tasks a view selects, across its projects.
func (d *agentDaemon) desktopViewTasks(w http.ResponseWriter, r *http.Request, viewID string) {
	var tasks []models.Task
	q := url.Values{"viewId": {viewID}, "q": {r.URL.Query().Get("q")}}
	if err := d.readAPI(r.Context(), "/api/tasks?"+q.Encode(), &tasks); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	filtered := []models.Task{}
	for _, task := range tasks {
		if r.URL.Query().Get("launchable") != "true" || !desktopTaskFinished(task) {
			filtered = append(filtered, task)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(filtered)
}
