package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// specArtifactsDesktop is a desktop handler over a project whose server
// setting is serverValue, mapped to a fresh Git checkout.
type specArtifactsDesktop struct {
	t    *testing.T
	d    *agentDaemon
	root string
}

func newSpecArtifactsDesktop(t *testing.T, serverValue string) *specArtifactsDesktop {
	t.Helper()
	testhome.Temp(t)
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := agentconfig.WriteSettings(agentconfig.Overrides{Projects: map[string]string{"p": root}}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/config"):
			json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: "p", GitRemoteURL: "https://example.test/project.git", AIProvider: "claude", SpecArtifacts: serverValue})
		case strings.HasPrefix(r.URL.Path, "/api/projects/"):
			json.NewEncoder(w).Encode(models.Project{ID: "p", Name: "Project P", MonoRepo: true})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, projectID: "p"}}
	return &specArtifactsDesktop{t: t, d: d, root: root}
}

func (s *specArtifactsDesktop) do(method, path string, body any) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	r := httptest.NewRequest(method, path, reader)
	r.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	s.d.desktopHandler(w, r)
	return w
}

func (s *specArtifactsDesktop) save(extra map[string]any) int {
	s.t.Helper()
	body := map[string]any{"projectId": "p", "path": s.root}
	for key, value := range extra {
		body[key] = value
	}
	return s.do("POST", "/desktop/projects", body).Code
}

func (s *specArtifactsDesktop) info() map[string]any {
	s.t.Helper()
	w := s.do("GET", "/desktop/project?id=p", nil)
	if w.Code != 200 {
		s.t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		s.t.Fatal(err)
	}
	return out
}

// launchValue is what a dispatch reads: the settings merged by
// localProjectRoot, applied to the server configuration.
func (s *specArtifactsDesktop) launchValue(serverValue string) string {
	s.t.Helper()
	config := agentconfig.Config{ProjectID: "p", GitRemoteURL: "https://example.test/project.git", SpecArtifacts: serverValue}
	_, overrides, err := s.d.localProjectRoot(context.Background(), config)
	if err != nil {
		s.t.Fatal(err)
	}
	return agentconfig.ApplyOverrides(config, overrides).SpecArtifacts
}

// The desktop stores a keep or drop override for the project, shows the
// effective value beside the server's, and a reset follows the server again
// (#487). The override reaches launches through localProjectRoot.
func TestDesktopProjectSpecArtifactsOverride(t *testing.T) {
	s := newSpecArtifactsDesktop(t, "keep")

	got := s.info()
	if got["specArtifacts"] != "keep" || got["specArtifactsOverride"] != false {
		t.Fatalf("without an override the server value is inherited: %v", got)
	}
	if server, _ := got["server"].(map[string]any); server["specArtifacts"] != "keep" {
		t.Fatalf("the server value must be reported for the hint: %v", got["server"])
	}

	if code := s.save(map[string]any{"specArtifacts": "discard"}); code != 400 {
		t.Fatalf("an unknown value must be refused, got %d", code)
	}
	if code := s.save(map[string]any{"specArtifacts": "drop"}); code != http.StatusNoContent {
		t.Fatalf("saving drop: %d", code)
	}
	got = s.info()
	if got["specArtifacts"] != "drop" || got["specArtifactsOverride"] != true {
		t.Fatalf("the drop override must be in effect: %v", got)
	}
	if value := s.launchValue("keep"); value != "drop" {
		t.Fatalf("a launch must read the override, got %q", value)
	}

	// Saving another setting leaves the override alone.
	if code := s.save(map[string]any{"parallelism": 2}); code != http.StatusNoContent {
		t.Fatalf("saving parallelism: %d", code)
	}
	if s.info()["specArtifacts"] != "drop" {
		t.Fatal("a save that does not name the override must keep it")
	}

	// A save while dropping leaves the rules of the tasks in place.
	if _, err := ensureSpecExclusions(context.Background(), s.root, "p", "#9"); err != nil {
		t.Fatal(err)
	}
	if code := s.save(nil); code != http.StatusNoContent || !strings.Contains(readExclude(t, s.root), "/specs/9-*/") {
		t.Fatalf("a save with an effective drop must keep the block: %d\n%s", code, readExclude(t, s.root))
	}

	if code := s.save(map[string]any{"specArtifacts": "drop", "inheritSpecArtifacts": true}); code != http.StatusNoContent {
		t.Fatalf("resetting: %d", code)
	}
	got = s.info()
	if got["specArtifacts"] != "keep" || got["specArtifactsOverride"] != false {
		t.Fatalf("a reset must follow the server again: %v", got)
	}
	settings, err := agentconfig.ReadSettings(s.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := settings.SpecArtifacts["p"]; ok {
		t.Fatalf("a reset must store no override: %v", settings.SpecArtifacts)
	}
	if strings.Contains(readExclude(t, s.root), "sectile") {
		t.Fatalf("a save with an effective keep must remove the block:\n%s", readExclude(t, s.root))
	}
	if value := s.launchValue("drop"); value != "drop" {
		t.Fatalf("without an override a launch follows the server, got %q", value)
	}
}

// The desktop warns that specifications already committed stay in the
// history: it counts what the checkout tracks where artefacts live.
func TestDesktopProjectCountsTrackedSpecifications(t *testing.T) {
	s := newSpecArtifactsDesktop(t, "drop")
	if got := s.info()["specArtifactsTracked"]; got != float64(0) {
		t.Fatalf("an empty checkout tracks nothing: %v", got)
	}
	for _, path := range []string{"specs/1-a/spec.md", "docs/clarifications/1.md", "openspec/changes/2-b/proposal.md", "openspec/specs/kept.md", "src/specs.go"} {
		full := filepath.Join(s.root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitTest(t, s.root, "add", ".")
	gitTest(t, s.root, "commit", "-q", "-m", "specs")
	if got := s.info()["specArtifactsTracked"]; got != float64(3) {
		t.Fatalf("three tracked specification files expected, got %v", got)
	}
	if got := trackedSpecArtifacts(context.Background(), s.root, errors.New("unmapped")); got != 0 {
		t.Fatalf("an invalid mapping counts nothing: %d", got)
	}
}
