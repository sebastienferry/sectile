package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
)

// projectTerminalFixture is a daemon whose project p is mapped to a local
// repository and whose server serves the configuration of p and of q, a project
// with no folder on this workstation. Every terminal the daemon opens is
// recorded instead of started.
func projectTerminalFixture(t *testing.T) (*agentDaemon, *[][2]string) {
	t.Helper()
	d, config := disconnectFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/config" {
			http.NotFound(w, r)
			return
		}
		served := config
		served.ProjectID = r.URL.Query().Get("projectId")
		if served.ProjectID == "q" {
			served.GitRemoteURL = "https://example.test/elsewhere.git"
		}
		_ = json.NewEncoder(w).Encode(served)
	}))
	t.Cleanup(server.Close)
	d.link.serverURL = server.URL
	opened := &[][2]string{}
	d.openTerminalFn = func(terminal, directory string) error {
		*opened = append(*opened, [2]string{terminal, directory})
		return nil
	}
	return d, opened
}

// Open terminal opens the project's mapped folder in the terminal its settings
// name, and never a folder the request names (#761).
func TestDesktopProjectTerminalOpensTheProjectFolder(t *testing.T) {
	d, opened := projectTerminalFixture(t)
	settings, err := agentconfig.ReadSettings(d.repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	project := settings.ProjectSettings["p"]
	project.Terminal = "kitty"
	settings.SetProject("p", project)
	if err = agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	w := disconnectRequest(d, http.MethodPost, "/desktop/project-terminal", `{"projectId":"p","directory":"/etc"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("open: %d %s", w.Code, w.Body.String())
	}
	var answer struct {
		Opened    bool   `json:"opened"`
		Terminal  string `json:"terminal"`
		Directory string `json:"directory"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	want := [2]string{"kitty", d.repoRoot}
	if !answer.Opened || answer.Terminal != want[0] || answer.Directory != want[1] || len(*opened) != 1 || (*opened)[0] != want {
		t.Fatalf("answer %+v, opened %v, want %v", answer, *opened, want)
	}
}

func TestDesktopProjectTerminalRefusals(t *testing.T) {
	d, opened := projectTerminalFixture(t)
	for _, tc := range []struct {
		name, method, body string
		code               int
	}{
		{"wrong method", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"no project", http.MethodPost, `{}`, http.StatusBadRequest},
		{"blank project", http.MethodPost, `{"projectId":"  "}`, http.StatusBadRequest},
		{"malformed body", http.MethodPost, `{"projectId":`, http.StatusBadRequest},
		{"no local folder", http.MethodPost, `{"projectId":"q"}`, http.StatusConflict},
	} {
		if w := disconnectRequest(d, tc.method, "/desktop/project-terminal", tc.body); w.Code != tc.code {
			t.Errorf("%s: %d %s, want %d", tc.name, w.Code, w.Body.String(), tc.code)
		}
	}
	settings, err := agentconfig.ReadSettings(d.repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	settings.DisconnectedProjects = map[string]bool{"p": true}
	if err = agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	if w := disconnectRequest(d, http.MethodPost, "/desktop/project-terminal", `{"projectId":"p"}`); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "disconnected") {
		t.Errorf("disconnected project: %d %s", w.Code, w.Body.String())
	}
	if len(*opened) != 0 {
		t.Fatalf("a refused request opened a terminal: %v", *opened)
	}
}

func TestDesktopProjectTerminalReportsTheServerAndTheLaunch(t *testing.T) {
	d, _ := projectTerminalFixture(t)
	d.openTerminalFn = func(string, string) error { return errors.New("failed to open terminal kitty: not found") }
	if w := disconnectRequest(d, http.MethodPost, "/desktop/project-terminal", `{"projectId":"p"}`); w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "kitty: not found") {
		t.Fatalf("launch failure: %d %s", w.Code, w.Body.String())
	}
	d.link.serverURL = "http://127.0.0.1:1"
	if w := disconnectRequest(d, http.MethodPost, "/desktop/project-terminal", `{"projectId":"p"}`); w.Code != http.StatusBadGateway {
		t.Fatalf("unreachable server: %d %s", w.Code, w.Body.String())
	}
}

func TestDesktopStatusAnnouncesProjectTerminal(t *testing.T) {
	d, _ := projectTerminalFixture(t)
	w := disconnectRequest(d, http.MethodGet, "/desktop/status", "")
	var status struct {
		Capabilities []string `json:"capabilities"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &status) != nil {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	if !slices.Contains(status.Capabilities, projectTerminalCapability) {
		t.Fatalf("%s not announced: %v", projectTerminalCapability, status.Capabilities)
	}
}
