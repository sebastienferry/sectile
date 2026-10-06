package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

func TestDesktopArchiveWorkspaceRelaysToTheServer(t *testing.T) {
	testhome.Temp(t)
	var forwarded, token string
	answer := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"archivable":false,"repositories":[{"repository":"github.com/o/a","role":"code","outcome":"failed","error":"contains untracked files"}]}`))
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/config"):
			_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: "p"})
		case r.URL.Path == "/api/tasks/gh-1/archive-workspace" && r.Method == http.MethodPost:
			forwarded, token = r.URL.Path, r.Header.Get("Authorization")
			answer(w)
		case r.URL.Path == "/api/tasks/gone/archive-workspace":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Task not found"}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Route non trouvée"}`))
		}
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, token: "device-token"}}
	post := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, req)
		return w
	}

	w := post("/desktop/tasks/archive-workspace?projectId=p", `{"taskId":"gh-1"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "untracked files") {
		t.Fatalf("relay: %d %s", w.Code, w.Body.String())
	}
	if forwarded != "/api/tasks/gh-1/archive-workspace" || token != "Bearer device-token" {
		t.Errorf("forwarded %q with %q", forwarded, token)
	}
	if w := post("/desktop/tasks/archive-workspace?projectId=p", `{}`); w.Code != http.StatusBadRequest {
		t.Errorf("no task: %d", w.Code)
	}
	if w := post("/desktop/tasks/archive-workspace?projectId=p", `{"taskId":"gone"}`); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "Task not found") {
		t.Errorf("unknown task: %d %s", w.Code, w.Body.String())
	}
	if w := post("/desktop/tasks/archive-workspace?projectId=p", `{"taskId":"old-server"}`); w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "update the server") {
		t.Errorf("server without the route: %d %s", w.Code, w.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/desktop/status", nil)
	req.Header.Set("Authorization", "Bearer private")
	status := httptest.NewRecorder()
	d.desktopHandler(status, req)
	var body struct {
		Capabilities []string `json:"capabilities"`
	}
	if json.Unmarshal(status.Body.Bytes(), &body) != nil || !slices.Contains(body.Capabilities, archiveWorkspaceCapability) {
		t.Errorf("%s not announced: %s", archiveWorkspaceCapability, status.Body.String())
	}
}
