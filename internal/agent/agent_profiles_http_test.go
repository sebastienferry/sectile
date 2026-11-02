package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
)

func profileHTTPFixture(t *testing.T) *agentDaemon {
	t.Helper()
	d, config := disconnectFixture(t)
	canonical, err := filepath.EvalSymlinks(d.repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(d.repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	project := settings.ProjectSettings["p"]
	project.Path = canonical
	settings.ProjectSettings["p"] = project
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	d.repoRoot = canonical
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/config" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(config)
	}))
	t.Cleanup(server.Close)
	d.link.serverURL = server.URL
	return d
}

func TestAgentProfileDesktopContractAndAdmission(t *testing.T) {
	d := profileHTTPFixture(t)
	for _, request := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/desktop/agent-profiles", "", 400},
		{"DELETE", "/desktop/agent-profiles", "", 405},
		{"POST", "/desktop/agent-profiles", `{"projectId":"p","id":"../bad","prompt":"ok"}`, 400},
		{"POST", "/desktop/agent-profiles", `{"projectId":"p","id":"good","prompt":" "}`, 400},
		{"POST", "/desktop/agent-profiles", "{\"projectId\":\"p\",\"id\":\"good\",\"prompt\":\"\xff\"}", 400},
		{"POST", "/desktop/agent-profiles", `{"projectId":"p","id":"good","prompt":"ok"}{}`, 400},
		{"POST", "/desktop/agent-profiles", `{"projectId":"p","id":"good","prompt":"ok","path":"/escape"}`, 400},
		{"GET", "/desktop/agent-profiles?projectId=p", "", 200},
		{"POST", "/desktop/agent-profiles", `{"projectId":"p","id":"reviewer","prompt":"review code"}`, 201},
		{"POST", "/desktop/agent-profiles", `{"projectId":"p","id":"reviewer","prompt":"replacement"}`, 409},
		{"POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","agentProfileId":"../bad"}`, 400},
		{"POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","agentProfileId":"missing"}`, 404},
		{"POST", "/desktop/consoles", `{"projectId":"p","provider":"agy","agentProfileId":"reviewer"}`, 422},
	} {
		response := disconnectRequest(d, request.method, request.path, request.body)
		if response.Code != request.status {
			t.Fatalf("%s %s: %d %s", request.method, request.path, response.Code, response.Body.String())
		}
	}
	list := disconnectRequest(d, "GET", "/desktop/agent-profiles?projectId=p", "")
	if !strings.Contains(list.Body.String(), `"id":"reviewer"`) {
		t.Fatal(list.Body.String())
	}
	if len(d.queue.runs) > 0 {
		t.Fatal("failed profile request admitted a run")
	}
	raw, err := os.ReadFile(filepath.Join(d.repoRoot, ".sectile", "agents", "reviewer.md"))
	if err != nil || string(raw) != "review code" {
		t.Fatalf("existing profile changed: %q %v", raw, err)
	}
	req := httptest.NewRequest("GET", "/desktop/agent-profiles?projectId=p", nil)
	rec := httptest.NewRecorder()
	d.desktopHandler(rec, req)
	if rec.Code != 401 {
		t.Fatal("profile API bypassed desktop authorization")
	}
}

func TestAgentProfileInvalidFileLaunchNamesTheProfile(t *testing.T) {
	d := profileHTTPFixture(t)
	if err := createAgentProfile(d.repoRoot, "reviewer", "review code"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(d.repoRoot, ".sectile", "agents", "reviewer.md")
	for _, content := range []string{"", "nul\x00byte"} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		response := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","agentProfileId":"reviewer"}`)
		if response.Code != 422 || !strings.Contains(response.Body.String(), "invalid or unreadable") {
			t.Fatalf("invalid profile %q: %d %s", content, response.Code, response.Body.String())
		}
	}
	if len(d.queue.runs) > 0 {
		t.Fatal("invalid profile admitted a run")
	}
}

func TestAgentProfileMissingCheckoutAndUnwritableDirectory(t *testing.T) {
	d := profileHTTPFixture(t)
	if err := os.WriteFile(filepath.Join(d.repoRoot, ".sectile"), []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}
	response := disconnectRequest(d, "POST", "/desktop/agent-profiles", `{"projectId":"p","id":"reviewer","prompt":"review"}`)
	if response.Code != 500 {
		t.Fatalf("invalid storage: %d %s", response.Code, response.Body.String())
	}
	if err := os.RemoveAll(d.repoRoot); err != nil {
		t.Fatal(err)
	}
	response = disconnectRequest(d, "GET", "/desktop/agent-profiles?projectId=p", "")
	if response.Code != 409 {
		t.Fatalf("missing checkout: %d %s", response.Code, response.Body.String())
	}
}
