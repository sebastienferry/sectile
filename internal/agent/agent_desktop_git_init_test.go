package agent

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tasks/internal/testhome"
)

// gitInitDaemon isolates the test from the workstation's Git configuration,
// so the identity a commit uses is the one the test writes, and returns a
// daemon with the desktop token and a call helper.
func gitInitDaemon(t *testing.T, gitconfig string) func(method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	home := testhome.Temp(t)
	config := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(config, []byte(gitconfig), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, name := range []string{"EMAIL", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	return func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		return w
	}
}

const gitInitIdentity = "[user]\n\tname = Sectile Test\n\temail = test@sectile.invalid\n[init]\n\tdefaultBranch = trunk\n"

type gitInitAnswer struct {
	Path        string `json:"path"`
	State       string `json:"state"`
	Initialized bool   `json:"initialized"`
	Committed   bool   `json:"committed"`
}

func decodeGitInit(t *testing.T, w *httptest.ResponseRecorder) gitInitAnswer {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var answer gitInitAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatalf("%v: %s", err, w.Body.String())
	}
	return answer
}

func gitState(t *testing.T, do func(string, string, any) *httptest.ResponseRecorder, path string) gitInitAnswer {
	t.Helper()
	return decodeGitInit(t, do("GET", "/desktop/git-init?path="+url.QueryEscape(path), nil))
}

// unbornRepository is a repository with no commit, on branch.
func unbornRepository(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q")
	gitTest(t, dir, "symbolic-ref", "HEAD", "refs/heads/"+branch)
	return dir
}

func TestGitInitReportsTheFourFolderStates(t *testing.T) {
	do := gitInitDaemon(t, gitInitIdentity)
	folder := t.TempDir()
	file := filepath.Join(folder, "notes.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		checkoutOf(t, "git@github.com:o/a.git"): gitStateReady,
		unbornRepository(t, "main"):             gitStateUnborn,
		folder:                                  gitStateFolder,
		file:                                    gitStateMissing,
		filepath.Join(folder, "absent"):         gitStateMissing,
		"relative/path":                         gitStateMissing,
		"":                                      gitStateMissing,
	} {
		if got := gitState(t, do, path).State; got != want {
			t.Errorf("state of %q = %s, want %s", path, got, want)
		}
	}
}

func TestGitInitMakesAPlainFolderARepositoryWithAnEmptyCommit(t *testing.T) {
	do := gitInitDaemon(t, gitInitIdentity)
	folder := t.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "notes.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	answer := decodeGitInit(t, do("POST", "/desktop/git-init", map[string]string{"path": folder}))
	if !answer.Initialized || !answer.Committed || answer.State != gitStateReady || !samePath(t, answer.Path, folder) {
		t.Fatalf("answer = %+v", answer)
	}
	if branch := gitTest(t, folder, "symbolic-ref", "--short", "HEAD"); branch != "main" {
		t.Errorf("branch = %s, want main whatever init.defaultBranch says", branch)
	}
	if count := gitTest(t, folder, "rev-list", "--count", "HEAD"); count != "1" {
		t.Errorf("commits = %s", count)
	}
	if tree := gitTest(t, folder, "ls-tree", "-r", "HEAD"); tree != "" {
		t.Errorf("the first commit is not empty: %s", tree)
	}
	if status := gitTest(t, folder, "status", "--porcelain"); status != "?? notes.txt" {
		t.Errorf("status = %q, want the folder's file untracked", status)
	}
	if raw, _ := os.ReadFile(filepath.Join(folder, "notes.txt")); string(raw) != "keep me" {
		t.Errorf("the folder's file changed: %q", raw)
	}
	if raw, _ := os.ReadFile(filepath.Join(folder, ".git", "info", "exclude")); !strings.Contains(string(raw), "/.tasks/") {
		t.Errorf("exclude = %q", raw)
	}
	if _, err := os.Stat(filepath.Join(folder, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf(".gitignore created: %v", err)
	}
	if remotes := gitTest(t, folder, "remote"); remotes != "" {
		t.Errorf("remotes = %q", remotes)
	}
	worktree := filepath.Join(t.TempDir(), "task")
	gitTest(t, folder, "worktree", "add", "-q", "-b", "feat/1", worktree, "HEAD")
	if _, err := os.Stat(filepath.Join(worktree, "notes.txt")); !os.IsNotExist(err) {
		t.Errorf("the worktree holds an uncommitted file: %v", err)
	}

	// A second click finds the folder ready and commits nothing more.
	again := decodeGitInit(t, do("POST", "/desktop/git-init", map[string]string{"path": folder}))
	if again.Initialized || again.Committed || again.State != gitStateReady {
		t.Errorf("second answer = %+v", again)
	}
	if count := gitTest(t, folder, "rev-list", "--count", "HEAD"); count != "1" {
		t.Errorf("commits after a second click = %s", count)
	}
}

func TestGitInitCommitsAnUnbornRepositoryWhereItsHeadPoints(t *testing.T) {
	do := gitInitDaemon(t, gitInitIdentity)
	repo := unbornRepository(t, "develop")
	gitTest(t, repo, "config", "sectile.test", "kept")
	if runtime.GOOS != "windows" {
		hook := filepath.Join(repo, ".git", "hooks", "commit-msg")
		if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	answer := decodeGitInit(t, do("POST", "/desktop/git-init", map[string]string{"path": repo}))
	if answer.Initialized || !answer.Committed || answer.State != gitStateReady {
		t.Fatalf("answer = %+v", answer)
	}
	if branch := gitTest(t, repo, "symbolic-ref", "--short", "HEAD"); branch != "develop" {
		t.Errorf("branch = %s, the unborn branch was renamed", branch)
	}
	if value := gitTest(t, repo, "config", "sectile.test"); value != "kept" {
		t.Errorf("local config = %q, the repository was initialized again", value)
	}
	if count := gitTest(t, repo, "rev-list", "--count", "HEAD"); count != "1" {
		t.Errorf("commits = %s", count)
	}
}

func TestGitInitCommitsTheCheckoutAroundASubfolder(t *testing.T) {
	do := gitInitDaemon(t, gitInitIdentity)
	repo := unbornRepository(t, "main")
	sub := filepath.Join(repo, "docs", "specs")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if state := gitState(t, do, sub); state.State != gitStateUnborn || !samePath(t, state.Path, repo) {
		t.Fatalf("subfolder state = %+v", state)
	}
	answer := decodeGitInit(t, do("POST", "/desktop/git-init", map[string]string{"path": sub}))
	if answer.Initialized || !answer.Committed || !samePath(t, answer.Path, repo) {
		t.Fatalf("answer = %+v", answer)
	}
	if _, err := os.Stat(filepath.Join(sub, ".git")); !os.IsNotExist(err) {
		t.Errorf("a nested repository was created: %v", err)
	}
	if count := gitTest(t, repo, "rev-list", "--count", "HEAD"); count != "1" {
		t.Errorf("commits = %s", count)
	}
}

func TestGitInitLeavesAReadyRepositoryAlone(t *testing.T) {
	do := gitInitDaemon(t, gitInitIdentity)
	repo := checkoutOf(t, "git@github.com:o/a.git")
	head := gitTest(t, repo, "rev-parse", "HEAD")
	answer := decodeGitInit(t, do("POST", "/desktop/git-init", map[string]string{"path": repo}))
	if answer.Initialized || answer.Committed || answer.State != gitStateReady {
		t.Errorf("answer = %+v", answer)
	}
	if now := gitTest(t, repo, "rev-parse", "HEAD"); now != head {
		t.Errorf("HEAD moved from %s to %s", head, now)
	}
	if raw, _ := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude")); strings.Contains(string(raw), ".tasks") {
		t.Errorf("a ready repository was touched: %q", raw)
	}
}

func TestGitInitRefusesWhatIsNotAProjectFolder(t *testing.T) {
	do := gitInitDaemon(t, gitInitIdentity)
	home, _ := os.UserHomeDir()
	folder := t.TempDir()
	file := filepath.Join(folder, "notes.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = filepath.VolumeName(folder) + `\`
	}
	for _, path := range []string{"relative", filepath.Join(folder, "absent"), file, root, home} {
		w := do("POST", "/desktop/git-init", map[string]string{"path": path})
		if w.Code != 400 {
			t.Errorf("%q: status %d: %s", path, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{home, folder} {
		if _, err := os.Stat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
			t.Errorf("a repository was created in %s: %v", path, err)
		}
	}
	if w := do("POST", "/desktop/git-init", map[string]string{"path": home}); !strings.Contains(w.Body.String(), "choose the project's own folder") {
		t.Errorf("home refusal = %s", w.Body.String())
	}
}

func TestGitInitWithoutAnIdentityShowsGitsErrorAndLeavesTheFolderUnborn(t *testing.T) {
	do := gitInitDaemon(t, "[user]\n\tuseConfigOnly = true\n")
	folder := t.TempDir()
	w := do("POST", "/desktop/git-init", map[string]string{"path": folder})
	if w.Code != 422 || !strings.Contains(w.Body.String(), "git commit") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if state := gitState(t, do, folder); state.State != gitStateUnborn {
		t.Errorf("state after the failure = %+v, want unborn for a retry", state)
	}
}

func TestDesktopStatusListsGitInit(t *testing.T) {
	testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir(), loopback: loopbackServer{desktopToken: "private"}}
	r := httptest.NewRequest("GET", "/desktop/status", nil)
	r.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, r)
	var status struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("%v: %s", err, w.Body.String())
	}
	if !strings.Contains(strings.Join(status.Capabilities, ","), "git-init") {
		t.Errorf("capabilities = %v", status.Capabilities)
	}
}
