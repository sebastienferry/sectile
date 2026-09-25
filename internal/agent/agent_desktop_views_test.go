package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// viewServer is a server with one mono-repo project "p", its task "t1", the
// user's view "v1" on github.com/o/view, and a run-skill route that records
// what it was asked.
type viewServer struct {
	mu       sync.Mutex
	launches []map[string]any
	queries  []string
	refuse   bool
}

func (s *viewServer) handler(t *testing.T, root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/config"):
			config := agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: "p", ProjectName: "Mono", GitRemoteURL: "git@github.com:o/project.git"}
			_ = json.NewEncoder(w).Encode(config)
		case r.URL.Path == "/api/me/board-views":
			_ = json.NewEncoder(w).Encode([]models.BoardView{{ID: "v1", Name: "Platform", ProjectIDs: []string{"p", "q"}, Repository: "git@github.com:o/view.git"}})
		case r.URL.Path == "/api/me/board-views/v1":
			_ = json.NewEncoder(w).Encode(models.BoardView{ID: "v1", Name: "Platform", ProjectIDs: []string{"p", "q"}, Repository: "git@github.com:o/view.git"})
		case r.URL.Path == "/api/tasks" && r.Method == http.MethodGet:
			s.mu.Lock()
			s.queries = append(s.queries, r.URL.RawQuery)
			s.mu.Unlock()
			_ = json.NewEncoder(w).Encode([]models.Task{{ID: "t1", ProjectID: "p", Key: "#1"}, {ID: "t2", ProjectID: "q", Key: "#2", Status: models.StatusFinished}})
		case r.URL.Path == "/api/tasks/t1" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(models.Task{ID: "t1", ProjectID: "p", Key: "#1"})
		case r.URL.Path == "/api/tasks/t1/run-skill":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			s.mu.Lock()
			s.launches = append(s.launches, body)
			refuse := s.refuse
			s.mu.Unlock()
			if refuse {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":"busy"}`))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	})
}

func viewDaemon(t *testing.T, root string) (*agentDaemon, *viewServer, func(method, path string, body any) *httptest.ResponseRecorder) {
	t.Helper()
	server := &viewServer{}
	srv := httptest.NewServer(server.handler(t, root))
	t.Cleanup(srv.Close)
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, projectID: "other"}}
	return d, server, func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		return w
	}
}

// US2: a view's folder is saved on the workstation once it is a checkout, with
// a warning when it is a checkout of another repository, and can be cleared.
func TestDesktopViewDirectory(t *testing.T) {
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/project.git")
	view := checkoutOf(t, "https://github.com/o/view")
	other := checkoutOf(t, "git@github.com:o/other.git")
	_, _, do := viewDaemon(t, root)

	if w := do("POST", "/desktop/views", map[string]any{"viewId": "v1", "path": t.TempDir()}); w.Code != 400 || !strings.Contains(w.Body.String(), "not a Git checkout") {
		t.Errorf("a plain folder: %d %s", w.Code, w.Body.String())
	}
	var answer struct{ Directory, Warning string }
	w := do("POST", "/desktop/views", map[string]any{"viewId": "v1", "path": other})
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &answer) != nil || !samePath(t, answer.Directory, other) ||
		!strings.Contains(answer.Warning, "git@github.com:o/other.git") || !strings.Contains(answer.Warning, "github.com/o/view") {
		t.Fatalf("another repository's checkout: %d %s", w.Code, w.Body.String())
	}
	w = do("POST", "/desktop/views", map[string]any{"viewId": "v1", "path": view})
	answer.Warning = ""
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &answer) != nil || answer.Warning != "" {
		t.Fatalf("the view repository's checkout: %d %s", w.Code, w.Body.String())
	}

	var views []desktopView
	w = do("GET", "/desktop/views", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &views); err != nil || len(views) != 1 || !samePath(t, views[0].Directory, view) || views[0].Repository != "git@github.com:o/view.git" {
		t.Fatalf("views = %s (%v)", w.Body.String(), err)
	}

	if w := do("POST", "/desktop/views", map[string]any{"viewId": "v1", "path": ""}); w.Code != 200 {
		t.Fatalf("clear: %d", w.Code)
	}
	if settings, _ := agentconfig.ReadSettings(root); len(settings.ViewDirectories) != 0 {
		t.Errorf("a cleared folder stays on disk: %v", settings.ViewDirectories)
	}
}

// US1: a view's tasks come from the server's view query, unfinished ones only
// when the desktop asks for launchable tasks, and the capability says so.
func TestDesktopViewTasks(t *testing.T) {
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/project.git")
	_, server, do := viewDaemon(t, root)

	w := do("GET", "/desktop/tasks?viewId=v1&q=abc&launchable=true", nil)
	var tasks []models.Task
	if err := json.Unmarshal(w.Body.Bytes(), &tasks); err != nil || len(tasks) != 1 || tasks[0].ID != "t1" {
		t.Fatalf("tasks = %s (%v)", w.Body.String(), err)
	}
	if len(server.queries) != 1 || !strings.Contains(server.queries[0], "viewId=v1") || strings.Contains(server.queries[0], "projectId") {
		t.Fatalf("server asked %v", server.queries)
	}
	var status struct{ Capabilities []string }
	w = do("GET", "/desktop/status", nil)
	if json.Unmarshal(w.Body.Bytes(), &status) != nil || !strings.Contains(strings.Join(status.Capabilities, ","), "board-views") {
		t.Fatalf("status = %s", w.Body.String())
	}
}

// US2 and FR3: a launch from a view runs in the view's folder, else in this
// workstation's checkout of the view's repository, carries the view to the
// server, and is remembered for the ticket; a launch without a view forgets it.
func TestDesktopLaunchFromAView(t *testing.T) {
	testhome.Temp(t)
	// The project is not mapped here: only the view's folder makes it
	// launchable.
	root := checkoutOf(t, "git@github.com:o/unrelated.git")
	viewFolder := checkoutOf(t, "git@github.com:o/view.git")
	mapped := checkoutOf(t, "https://github.com/o/view")
	d, server, do := viewDaemon(t, root)

	launch := map[string]any{"taskID": "t1", "skillID": "discuss", "viewID": "v1"}
	if w := do("POST", "/desktop/tasks?projectId=p", launch); w.Code != 400 {
		t.Fatalf("a view without a folder on an unmapped project: %d %s", w.Code, w.Body.String())
	}

	if err := agentconfig.WriteSettings(agentconfig.Overrides{Repositories: map[string]string{"github.com/o/view": mapped}}); err != nil {
		t.Fatal(err)
	}
	if w := do("POST", "/desktop/tasks?projectId=p", launch); w.Code != 200 {
		t.Fatalf("launch through the repository mapping: %d %s", w.Code, w.Body.String())
	}
	if got := d.viewRoots.get("t1"); got != mapped {
		t.Fatalf("recorded root = %q, want the mapped checkout", got)
	}

	if w := do("POST", "/desktop/views", map[string]any{"viewId": "v1", "path": viewFolder}); w.Code != 200 {
		t.Fatalf("set folder: %d %s", w.Code, w.Body.String())
	}
	if w := do("POST", "/desktop/tasks?projectId=p", launch); w.Code != 200 {
		t.Fatalf("launch from the view's folder: %d %s", w.Code, w.Body.String())
	}
	if got := d.viewRoots.get("t1"); !samePath(t, got, viewFolder) {
		t.Fatalf("recorded root = %q, want the view's folder", got)
	}
	server.mu.Lock()
	if len(server.launches) != 2 || server.launches[1]["viewId"] != "v1" {
		t.Fatalf("launches = %v", server.launches)
	}
	server.mu.Unlock()

	// Every later resolution for the ticket uses it, other settings included.
	config := agentconfig.Config{ProjectID: "p", GitRemoteURL: "git@github.com:o/project.git"}
	resolved, _, err := d.taskProjectRoot(context.Background(), config, "t1", false)
	if err != nil || !samePath(t, resolved, viewFolder) {
		t.Fatalf("task root = %q (%v)", resolved, err)
	}
	if _, _, err := d.taskProjectRoot(context.Background(), config, "t2", false); err == nil {
		t.Fatal("another ticket of the unmapped project resolved a root")
	}

	settings, err := agentconfig.ReadSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	settings.Projects = map[string]string{"p": checkoutOf(t, "git@github.com:o/project.git")}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	// A launch the server refuses leaves the recorded folder as it was.
	server.mu.Lock()
	server.refuse = true
	server.mu.Unlock()
	if w := do("POST", "/desktop/tasks?projectId=p", map[string]any{"taskID": "t1", "skillID": "discuss"}); w.Code != http.StatusConflict {
		t.Fatalf("refused launch: %d %s", w.Code, w.Body.String())
	}
	if got := d.viewRoots.get("t1"); !samePath(t, got, viewFolder) {
		t.Fatalf("a refused launch changed the recorded root to %q", got)
	}

	server.mu.Lock()
	server.refuse = false
	server.mu.Unlock()
	if w := do("POST", "/desktop/tasks?projectId=p", map[string]any{"taskID": "t1", "skillID": "discuss"}); w.Code != 200 {
		t.Fatalf("launch from the project: %d %s", w.Code, w.Body.String())
	}
	if got := d.viewRoots.get("t1"); got != "" {
		t.Fatalf("a launch from the project kept the view's folder %q", got)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if _, ok := server.launches[len(server.launches)-1]["viewId"]; ok {
		t.Fatalf("a launch from the project named a view: %v", server.launches[len(server.launches)-1])
	}
}
