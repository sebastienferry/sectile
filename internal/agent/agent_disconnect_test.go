package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentconfig"
)

func disconnectRequest(d *agentDaemon, method, target, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, r)
	return w
}

func disconnectFixture(t *testing.T) (*agentDaemon, agentconfig.Config) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	settings := agentconfig.Overrides{Projects: map[string]string{"p": root, "other": "/other"}, Commands: map[string]string{"p": "custom {prompt}"}, Worktrees: map[string]bool{"p": true}, Parallelism: map[string]int{"p": 3}}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	return &agentDaemon{desktopToken: "private", repoRoot: root, projectID: "p"}, agentconfig.Config{SchemaVersion: 1, ProjectID: "p", GitRemoteURL: "https://example.test/project.git"}
}

func TestProjectDisconnectionPersistenceAndReadd(t *testing.T) {
	d, config := disconnectFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("unexpected server mutation")
		}
		if r.URL.Path == "/api/v1/agent/projects" {
			json.NewEncoder(w).Encode(map[string]any{"schemaVersion": 1, "projects": []map[string]string{{"id": "p", "name": "Project"}}})
			return
		}
		json.NewEncoder(w).Encode(config)
	}))
	defer srv.Close()
	d.serverURL = srv.URL
	exited := make(chan struct{})
	close(exited)
	d.queue.runs = map[string]*controlledRun{"history": {desktop: desktopRun{ProjectID: "p", Status: "completed"}, exited: exited}}
	for i := 0; i < 2; i++ {
		if w := disconnectRequest(d, "DELETE", "/desktop/projects?id=p", ""); w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	settings, err := agentconfig.ReadSettings(d.repoRoot)
	if err != nil || !settings.DisconnectedProjects["p"] || settings.Projects["p"] != "" || settings.Projects["other"] != "/other" || len(settings.Commands) != 0 || len(settings.Worktrees) != 0 || len(settings.Parallelism) != 0 {
		t.Fatalf("settings: %+v %v", settings, err)
	}
	if len(d.queue.runs) != 1 {
		t.Fatal("history deleted")
	}
	if err := d.syncLocalProject(context.Background(), config); err != nil {
		t.Fatal("disconnected project prevented agent reconnection", err)
	}
	if _, err := os.Stat(filepath.Join(d.repoRoot, ".taskflow", "remote-config.json")); !os.IsNotExist(err) {
		t.Fatal("reconnection deployed tooling")
	}
	for _, projectID := range []string{"p", "different"} {
		restarted := &agentDaemon{repoRoot: d.repoRoot, projectID: projectID}
		if _, _, err := restarted.localProjectRoot(context.Background(), config); err == nil || !strings.Contains(err.Error(), "disconnected") {
			t.Fatal("fallback reconnected", err)
		}
	}
	w := disconnectRequest(d, "GET", "/desktop/projects", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"disconnected":true`) || !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = disconnectRequest(d, "GET", "/desktop/status", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"disconnectedProjects":["p"]`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = disconnectRequest(d, "POST", "/desktop/projects", `{"projectId":"p","path":"/does-not-exist"}`)
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	settings, _ = agentconfig.ReadSettings(d.repoRoot)
	if !settings.DisconnectedProjects["p"] {
		t.Fatal("failed re-add reconnected")
	}
	body, _ := json.Marshal(map[string]string{"projectId": "p", "path": d.repoRoot})
	settingsPath, _ := agentconfig.SettingsPath()
	if err := os.Chmod(filepath.Dir(settingsPath), 0500); err != nil {
		t.Fatal(err)
	}
	w = disconnectRequest(d, "POST", "/desktop/projects", string(body))
	os.Chmod(filepath.Dir(settingsPath), 0700)
	if w.Code != 500 {
		t.Fatal("re-add persistence failure", w.Code)
	}
	settings, _ = agentconfig.ReadSettings(d.repoRoot)
	if !settings.DisconnectedProjects["p"] {
		t.Fatal("failed save cleared disconnection")
	}
	w = disconnectRequest(d, "POST", "/desktop/projects", string(body))
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	settings, _ = agentconfig.ReadSettings(d.repoRoot)
	if settings.DisconnectedProjects["p"] || len(settings.Commands) != 0 || len(settings.Worktrees) != 0 || len(settings.Parallelism) != 0 {
		t.Fatal("re-add restored deleted overrides", settings)
	}
	if _, _, err := d.localProjectRoot(context.Background(), config); err != nil {
		t.Fatal(err)
	}
}

func TestProjectDisconnectionRequiresConfirmedExit(t *testing.T) {
	for _, status := range []string{"queued", "preparing", "running", "completed", "failed", "canceled"} {
		t.Run(status, func(t *testing.T) {
			d, _ := disconnectFixture(t)
			run := &controlledRun{desktop: desktopRun{ProjectID: "p", Status: status}, exited: make(chan struct{})}
			d.queue.runs = map[string]*controlledRun{"target": run, "other": {desktop: desktopRun{ProjectID: "other", Status: "running"}, exited: make(chan struct{})}}
			if w := disconnectRequest(d, "DELETE", "/desktop/projects?id=p", ""); w.Code != 409 {
				t.Fatal(w.Code)
			}
			if run.canceled {
				t.Fatal("removal canceled execution")
			}
			settings, _ := agentconfig.ReadSettings(d.repoRoot)
			if settings.DisconnectedProjects["p"] {
				t.Fatal("conflict changed settings")
			}
			close(run.exited)
			if w := disconnectRequest(d, "DELETE", "/desktop/projects?id=p", ""); w.Code != 204 {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}

func TestProjectDisconnectionAdmissionOrdering(t *testing.T) {
	for _, admitFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "admission wins", false: "removal wins"}[admitFirst], func(t *testing.T) {
			d, config := disconnectFixture(t)
			admit := func() (*controlledRun, error) {
				return d.admitProjectRun(context.Background(), "task", agentconfig.Dispatch{RunID: "run"}, config)
			}
			if admitFirst {
				// Hold runsMu so admission has resolved the repository but cannot register yet.
				d.queue.mu.Lock()
				result := make(chan error, 1)
				go func() { _, err := admit(); result <- err }()
				deadline := time.Now().Add(5 * time.Second)
				for d.prepareMu.TryLock() {
					d.prepareMu.Unlock()
					if time.Now().After(deadline) {
						d.queue.mu.Unlock()
						t.Fatal("admission did not acquire preparation lock")
					}
					time.Sleep(time.Millisecond)
				}
				w := disconnectRequest(d, "DELETE", "/desktop/projects?id=p", "")
				d.queue.mu.Unlock()
				if w.Code != 409 {
					t.Fatal("removal passed admission before registration", w.Code)
				}
				if err := <-result; err != nil {
					t.Fatal(err)
				}
				if w := disconnectRequest(d, "DELETE", "/desktop/projects?id=p", ""); w.Code != 409 {
					t.Fatal(w.Code)
				}
			} else {
				if w := disconnectRequest(d, "DELETE", "/desktop/projects?id=p", ""); w.Code != 204 {
					t.Fatal(w.Code)
				}
				if _, err := admit(); err == nil {
					t.Fatal("admitted disconnected project")
				}
				if len(d.queue.runs) != 0 {
					t.Fatal("registered rejected execution")
				}
			}
		})
	}
}

func TestProjectDisconnectionErrorsPreserveSettings(t *testing.T) {
	d, _ := disconnectFixture(t)
	req := httptest.NewRequest("DELETE", "/desktop/projects?id=p", nil)
	w := httptest.NewRecorder()
	d.desktopHandler(w, req)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := disconnectRequest(d, "DELETE", "/desktop/projects", ""); w.Code != 400 {
		t.Fatal(w.Code)
	}
	d.prepareMu.Lock()
	w = disconnectRequest(d, "DELETE", "/desktop/projects?id=p", "")
	d.prepareMu.Unlock()
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	path, _ := agentconfig.SettingsPath()
	before, _ := os.ReadFile(path)
	if err := os.Chmod(filepath.Dir(path), 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Dir(path), 0700)
	w = disconnectRequest(d, "DELETE", "/desktop/projects?id=p", "")
	if w.Code != 500 {
		t.Fatal(w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed write changed settings")
	}
}
