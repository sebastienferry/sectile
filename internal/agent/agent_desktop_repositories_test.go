package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

// desktopFixture serves multiRepoConfig to an agent whose project p lives in
// root, and records every request the server receives.
func desktopFixture(t *testing.T, root string) (func() []string, func(method, path string, body any) *httptest.ResponseRecorder) {
	t.Helper()
	var mu sync.Mutex
	var received []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, r.Method+" "+r.URL.String()+" "+string(raw))
		mu.Unlock()
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/config"):
			config := multiRepoConfig()
			config.SchemaVersion = agentconfig.Version
			_ = json.NewEncoder(w).Encode(config)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, projectID: "p"}}
	do := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var reader io.Reader
		if body != nil {
			raw, _ := json.Marshal(body)
			reader = bytes.NewReader(raw)
		}
		r := httptest.NewRequest(method, path, reader)
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		return w
	}
	requests := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), received...)
	}
	return requests, do
}

func TestDesktopMapsARepositoryOnlyToACheckoutOfIt(t *testing.T) {
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "https://github.com/o/b")
	c := checkoutOf(t, "git@github.com:o/c.git")
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root}}}); err != nil {
		t.Fatal(err)
	}
	_, do := desktopFixture(t, root)

	if w := do("POST", "/desktop/repositories", map[string]any{"projectId": "p", "repository": "github.com/o/b", "path": c}); w.Code != 400 || !strings.Contains(w.Body.String(), "git@github.com:o/c.git") || !strings.Contains(w.Body.String(), "github.com/o/b") {
		t.Errorf("mismatched origin: %d %s", w.Code, w.Body.String())
	}
	if w := do("POST", "/desktop/repositories", map[string]any{"projectId": "p", "repository": "github.com/o/elsewhere", "path": b}); w.Code != 400 {
		t.Errorf("repository outside the project: %d", w.Code)
	}
	if w := do("POST", "/desktop/repositories", map[string]any{"projectId": "p", "repository": "git@github.com:o/b.git", "path": b}); w.Code != 204 {
		t.Fatalf("map: %d %s", w.Code, w.Body.String())
	}
	settings, _ := agentconfig.ReadSettings(root)
	if !samePath(t, settings.Repositories["github.com/o/b"], b) {
		t.Errorf("stored mappings = %v", settings.Repositories)
	}

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

// The desktop attaches, lists and detaches folders (#484): each one is checked
// against what the project already has here, a checkout of a project
// repository becomes that repository's folder, and no path reaches the server.
func TestDesktopAttachesFoldersToAProject(t *testing.T) {
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/a.git")
	spec := t.TempDir()
	b := checkoutOf(t, "git@github.com:o/b.git")
	c := checkoutOf(t, "git@github.com:o/c.git")
	lib := checkoutOf(t, "git@github.com:o/lib.git")
	libAgain := checkoutOf(t, "https://github.com/o/lib")
	codeAgain := checkoutOf(t, "https://github.com/o/a")
	notes := t.TempDir()
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := agentconfig.WriteSettings(agentconfig.Settings{
		ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root, SpecPath: spec}},
		Repositories:    map[string]string{"github.com/o/b": b},
	}); err != nil {
		t.Fatal(err)
	}
	requests, do := desktopFixture(t, root)
	attach := func(path string) *httptest.ResponseRecorder {
		return do("POST", "/desktop/folders", map[string]any{"projectId": "p", "path": path})
	}
	list := func() []desktopFolder {
		t.Helper()
		w := do("GET", "/desktop/folders?projectId=p", nil)
		var out []desktopFolder
		if err := json.Unmarshal(w.Body.Bytes(), &out); w.Code != 200 || err != nil {
			t.Fatalf("list: %d %s %v", w.Code, w.Body.String(), err)
		}
		return out
	}

	if got := list(); len(got) != 0 {
		t.Fatalf("a new project lists %+v", got)
	}
	for _, path := range []string{lib, filepath.Join(notes, "."), gone} {
		if w := attach(path); w.Code != 204 {
			t.Fatalf("attach %s: %d %s", path, w.Code, w.Body.String())
		}
	}

	for path, why := range map[string]string{
		"relative":                      "absolute",
		filepath.Join(notes, "missing"): "does not exist",
		root:                            "local repository",
		spec:                            "specifications folder",
		b:                               "already the folder of github.com/o/b",
		notes:                           "already attached",
		libAgain:                        "already attached for github.com/o/lib",
		codeAgain:                       "project's own repository",
	} {
		if w := attach(path); w.Code < 400 || !strings.Contains(w.Body.String(), why) {
			t.Errorf("%s: want a refusal saying %q, got %d %s", path, why, w.Code, w.Body.String())
		}
	}

	// A checkout of a project repository becomes its folder, not an entry.
	w := attach(c)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"mappedAs":"github.com/o/c"`) {
		t.Fatalf("project repository: %d %s", w.Code, w.Body.String())
	}
	settings, _ := agentconfig.ReadSettings(root)
	if !samePath(t, settings.Repositories["github.com/o/c"], c) || len(settings.Project("p").Folders) != 3 {
		t.Errorf("settings = %+v %+v", settings.Repositories, settings.Project("p").Folders)
	}

	// The folder gone since is still listed, as missing, and can go.
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	got := list()
	if len(got) != 3 || got[0].Kind != folderKindGit || got[0].Identity != "github.com/o/lib" || got[0].Remote != "git@github.com:o/lib.git" ||
		got[1].Kind != folderKindFolder || got[1].Path != notes || got[2].Kind != folderKindMissing || got[2].Path != gone {
		t.Fatalf("list = %+v", got)
	}
	if w := do("DELETE", "/desktop/folders?projectId=p&path="+url.QueryEscape(gone), nil); w.Code != 204 {
		t.Fatalf("detach missing: %d %s", w.Code, w.Body.String())
	}

	// A remote that became a project repository, which this workstation maps
	// elsewhere, is listed as a duplicate of that folder.
	gitTest(t, lib, "remote", "set-url", "origin", "git@github.com:o/b.git")
	if got := list(); len(got) != 2 || got[0].Duplicate != "github.com/o/b" {
		t.Errorf("duplicate = %+v", got)
	}

	for _, path := range []string{lib, notes} {
		if w := do("DELETE", "/desktop/folders?projectId=p&path="+url.QueryEscape(path), nil); w.Code != 204 {
			t.Fatalf("detach %s: %d", path, w.Code)
		}
	}
	settings, _ = agentconfig.ReadSettings(root)
	if folders := settings.Project("p").Folders; len(folders) != 0 {
		t.Errorf("folders left = %v", folders)
	}
	raw, _ := os.ReadFile(mustSettingsPath(t))
	if strings.Contains(string(raw), `"folders"`) {
		t.Errorf("an emptied list stays in the settings file: %s", raw)
	}

	// The paths never travel: the server only ever saw configuration reads.
	for _, request := range requests() {
		for _, path := range []string{lib, notes, gone, c} {
			if strings.Contains(request, path) || strings.Contains(request, url.QueryEscape(path)) {
				t.Errorf("a folder path reached the server: %s", request)
			}
		}
	}
}

func mustSettingsPath(t *testing.T) string {
	t.Helper()
	path, err := agentconfig.SettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	return path
}
