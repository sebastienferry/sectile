package agent

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"tasks/internal/runner"
	"testing"
)

func TestDesktopGitDiffBoundary(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		b, e := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %s", e, b)
		}
	}
	git("init", "-b", "main")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.test")
	if e := os.WriteFile(filepath.Join(root, "file"), []byte("base\n"), 0644); e != nil {
		t.Fatal(e)
	}
	git("add", ".")
	git("commit", "-m", "base")
	git("checkout", "-b", "feat/test")
	d := &agentDaemon{desktopToken: "private", runs: map[string]*controlledRun{"run": {root: root, desktop: desktopRun{Directory: root, Branch: "feat/test", TaskID: "task", ProjectID: "project", Status: "running"}}, "unprepared": {}}}
	for _, tc := range []struct {
		name, method, id, token, origin string
		status                          int
	}{
		{"unauthorized", "GET", "run", "", "", 401}, {"origin", "GET", "run", "private", "https://example.test", 401}, {"method", "POST", "run", "private", "", 405}, {"unknown", "GET", "missing", "private", "", 404}, {"unprepared", "GET", "unprepared", "private", "", 409}, {"running", "GET", "run", "private", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/desktop/git-diff?id="+tc.id+"&directory=/etc", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			d.desktopHandler(w, req)
			if w.Code != tc.status {
				t.Fatalf("%d: %s", w.Code, w.Body)
			}
			if w.Code == 200 {
				var diff runner.WorktreeDiff
				if e := json.Unmarshal(w.Body.Bytes(), &diff); e != nil {
					t.Fatal(e)
				}
				if diff.Directory != root || diff.RunID != "run" || !diff.IsClean {
					t.Fatal("wrong execution result")
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("cacheable source")
				}
			}
		})
	}
	request := func() int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/desktop/git-diff?id=run", nil)
		r.Header.Set("Authorization", "Bearer private")
		d.desktopHandler(w, r)
		return w.Code
	}
	d.runs["run"].desktop.Status = "completed"
	if request() != 200 {
		t.Fatal("stopped run unavailable")
	}
	d.runs["run"].root = t.TempDir()
	if request() != 409 {
		t.Fatal("wrong repository accepted")
	}
	d.runs["run"].root = root
	git("checkout", "--detach")
	if request() != 409 {
		t.Fatal("detached HEAD accepted")
	}
	git("checkout", "feat/test")
	d.runs["run"].desktop.Directory = filepath.Join(root, "missing")
	if request() != 409 {
		t.Fatal("missing checkout accepted")
	}
}
