package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

func TestDesktopMapsARepositoryOnlyToACheckoutOfIt(t *testing.T) {
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "https://github.com/o/b")
	c := checkoutOf(t, "git@github.com:o/c.git")
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root}}}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var patched []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/config"):
			config := multiRepoConfig()
			config.SchemaVersion = agentconfig.Version
			_ = json.NewEncoder(w).Encode(config)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/tasks/"):
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			patched = append(patched, strings.TrimPrefix(r.URL.Path, "/api/tasks/")+"="+body["repository"])
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, projectID: "p"}}
	do := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var reader *bytes.Reader
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
		r := httptest.NewRequest(method, path, reader)
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		return w
	}

	if w := do("POST", "/desktop/repositories", map[string]any{"projectId": "p", "repository": "github.com/o/b", "path": c}); w.Code != 400 || !strings.Contains(w.Body.String(), "git@github.com:o/c.git") || !strings.Contains(w.Body.String(), "github.com/o/b") {
		t.Errorf("mismatched origin: %d %s", w.Code, w.Body.String())
	}
	if w := do("POST", "/desktop/repositories", map[string]any{"projectId": "p", "repository": "github.com/o/elsewhere", "path": b}); w.Code != 400 {
		t.Errorf("repository outside the project: %d", w.Code)
	}
	if w := do("POST", "/desktop/repositories", map[string]any{"projectId": "p", "repository": "git@github.com:o/b.git", "path": b, "taskId": "t1"}); w.Code != 204 {
		t.Fatalf("map and pin: %d %s", w.Code, w.Body.String())
	}
	settings, _ := agentconfig.ReadSettings(root)
	if !samePath(t, settings.Repositories["github.com/o/b"], b) {
		t.Errorf("stored mappings = %v", settings.Repositories)
	}
	mu.Lock()
	if strings.Join(patched, ",") != "t1=github.com/o/b" {
		t.Errorf("pins = %v", patched)
	}
	mu.Unlock()

	w := do("GET", "/desktop/repositories?projectId=p", nil)
	var list []desktopRepository
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 3 {
		t.Fatalf("list = %s, %v", w.Body.String(), err)
	}
	if !list[0].Code || list[0].Path != root || !samePath(t, list[1].Path, b) || list[2].Path != "" {
		t.Errorf("list = %+v", list)
	}

	if w := do("POST", "/desktop/repositories", map[string]any{"projectId": "p", "repository": "github.com/o/b", "path": ""}); w.Code != 204 {
		t.Fatalf("clear: %d", w.Code)
	}
	if settings, _ := agentconfig.ReadSettings(root); len(settings.Repositories) != 0 {
		t.Errorf("a cleared last mapping stays on disk: %v", settings.Repositories)
	}
}
