package agent

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	d := &agentDaemon{queue: runQueue{runs: map[string]*controlledRun{"run": {root: root, desktop: desktopRun{Directory: root, Branch: "feat/test", TaskID: "task", ProjectID: "project", Status: "running"}}, "unprepared": {}}}, loopback: loopbackServer{desktopToken: "private"}}
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
	d.queue.runs["run"].desktop.Status = "completed"
	if request() != 200 {
		t.Fatal("stopped run unavailable")
	}
	d.queue.runs["run"].root = t.TempDir()
	if request() != 409 {
		t.Fatal("wrong repository accepted")
	}
	d.queue.runs["run"].root = root
	git("checkout", "--detach")
	if request() != 409 {
		t.Fatal("detached HEAD accepted")
	}
	git("checkout", "feat/test")
	d.queue.runs["run"].desktop.Directory = filepath.Join(root, "missing")
	if request() != 409 {
		t.Fatal("missing checkout accepted")
	}
}

func TestDesktopGitDiffCarriesMarkdownImages(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		b, e := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %s", e, b)
		}
	}
	write := func(p, content string) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(root, p), []byte(content), 0644); e != nil {
			t.Fatal(e)
		}
	}
	git("init", "-b", "main")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.test")
	write("flow.png", "\x89PNG\r\n\x1a\nflow")
	git("add", ".")
	git("commit", "-m", "base")
	git("checkout", "-b", "feat/test")
	write("guide.md", "![Flow](flow.png) ![Missing](missing.png)\n")
	d := &agentDaemon{queue: runQueue{runs: map[string]*controlledRun{"run": {root: root, desktop: desktopRun{Directory: root, Branch: "feat/test", TaskID: "task", ProjectID: "project", Status: "running"}}}}, loopback: loopbackServer{desktopToken: "private"}}
	req := httptest.NewRequest("GET", "/desktop/git-diff?id=run", nil)
	req.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, req)
	if w.Code != 200 {
		t.Fatalf("%d: %s", w.Code, w.Body)
	}
	var diff struct {
		Files []struct {
			Path     string `json:"path"`
			Document struct {
				Images []map[string]string `json:"images"`
			} `json:"document"`
		} `json:"files"`
		Images map[string]map[string]string `json:"images"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &diff); e != nil {
		t.Fatal(e)
	}
	if image := diff.Images["new:flow.png"]; image["mimeType"] != "image/png" || image["data"] != "iVBORw0KGgpmbG93" {
		t.Fatalf("image payload: %+v", diff.Images)
	}
	refs := diff.Files[0].Document.Images
	if len(refs) != 2 || refs[0]["image"] != "new:flow.png" || refs[1]["omittedReason"] != "Image not found in the inspected state." {
		t.Fatalf("document refs: %+v", refs)
	}
}

// The desktop may inspect another folder of the run (#784), only one the run
// lists: it is read on its own branch, against its own default branch.
func TestDesktopGitDiffSelectedFolder(t *testing.T) {
	repo := func() string {
		dir := t.TempDir()
		for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "Test"}, {"config", "user.email", "test@example.test"}, {"commit", "--allow-empty", "-m", "base"}} {
			if b, e := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); e != nil {
				t.Fatalf("%v: %s", e, b)
			}
		}
		return dir
	}
	root, context := repo(), repo()
	if b, e := exec.Command("git", "-C", root, "checkout", "-b", "feat/test").CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	if e := os.WriteFile(filepath.Join(context, "local.txt"), []byte("edit\n"), 0644); e != nil {
		t.Fatal(e)
	}
	folders := []runFolder{{Path: root, Name: "app", Role: "primary"}, {Path: context, Name: "docs", Role: "context"}}
	d := &agentDaemon{queue: runQueue{runs: map[string]*controlledRun{"run": {root: root, desktop: desktopRun{Directory: root, Branch: "feat/test", TaskID: "task", Status: "running", Folders: folders}}}}, loopback: loopbackServer{desktopToken: "private"}}
	inspect := func(folder string) (*httptest.ResponseRecorder, runner.WorktreeDiff) {
		t.Helper()
		req := httptest.NewRequest("GET", "/desktop/git-diff?id=run&folder="+url.QueryEscape(folder), nil)
		req.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, req)
		var diff runner.WorktreeDiff
		if w.Code == 200 {
			if e := json.Unmarshal(w.Body.Bytes(), &diff); e != nil {
				t.Fatal(e)
			}
		}
		return w, diff
	}
	for _, folder := range []string{"", root, root + "/"} {
		if w, diff := inspect(folder); w.Code != 200 || diff.Directory != root || diff.Branch != "feat/test" {
			t.Fatalf("primary %q: %d %s", folder, w.Code, w.Body)
		}
	}
	w, diff := inspect(context)
	if w.Code != 200 || diff.Directory != context || diff.Branch != "main" || diff.FilesChanged != 1 || diff.RunID != "run" || diff.TaskID != "task" {
		t.Fatalf("context folder: %d %s", w.Code, w.Body)
	}
	for _, folder := range []string{t.TempDir(), "/etc", filepath.Join(context, "..")} {
		if w, _ := inspect(folder); w.Code != 404 || !strings.Contains(w.Body.String(), "folder_not_found") {
			t.Fatalf("unlisted %q: %d %s", folder, w.Code, w.Body)
		}
	}
	plain := t.TempDir()
	d.queue.runs["run"].desktop.Folders = append(folders, runFolder{Path: plain, Name: "notes", Role: "local", Attached: true})
	if w, _ := inspect(plain); w.Code != 409 || !strings.Contains(w.Body.String(), "not_a_repository") {
		t.Fatalf("plain folder: %d %s", w.Code, w.Body)
	}
}
