package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// bareOrigin is a bare repository holding main with one commit, and a clone of
// it whose origin is that bare repository's path.
func bareOrigin(t *testing.T) (origin, clone string) {
	t.Helper()
	origin = filepath.Join(t.TempDir(), "origin.git")
	gitTest(t, filepath.Dir(origin), "init", "-q", "--bare", "-b", "main", origin)
	seed := t.TempDir()
	gitTest(t, seed, "init", "-q", "-b", "main")
	gitTest(t, seed, "commit", "-q", "--allow-empty", "-m", "first")
	gitTest(t, seed, "push", "-q", origin, "main")
	clone = filepath.Join(t.TempDir(), "clone")
	gitTest(t, filepath.Dir(clone), "clone", "-q", origin, clone)
	return origin, clone
}

// advance adds a commit to origin's branch through a throwaway clone and
// answers its sha.
func advance(t *testing.T, origin, branch, message string) string {
	t.Helper()
	work := filepath.Join(t.TempDir(), "work")
	gitTest(t, filepath.Dir(work), "clone", "-q", origin, work)
	if branch != "main" {
		gitTest(t, work, "checkout", "-q", "-b", branch)
	}
	gitTest(t, work, "commit", "-q", "--allow-empty", "-m", message)
	gitTest(t, work, "push", "-q", "origin", branch)
	return gitTest(t, work, "rev-parse", "HEAD")
}

func TestFetchedWorktreeStartsFromTheRemoteDefaultBranch(t *testing.T) {
	ctx := context.Background()
	origin, clone := bareOrigin(t)
	// The clone is stale and sits on a feature branch of its own.
	gitTest(t, clone, "checkout", "-q", "-b", "old-work")
	gitTest(t, clone, "commit", "-q", "--allow-empty", "-m", "unrelated")
	head := advance(t, origin, "main", "second")
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	dir, branch, warning, err := ensureFetchedWorktree(ctx, clone, task, "")
	if err != nil || branch != "feat/1" || warning != "" {
		t.Fatalf("worktree = %q %q %q %v", dir, branch, warning, err)
	}
	if got := gitTest(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("the branch starts at %s, want origin/main %s", got, head)
	}
}

func TestFetchedWorktreeTakesTheRemoteBranch(t *testing.T) {
	ctx := context.Background()
	origin, clone := bareOrigin(t)
	head := advance(t, origin, "feat/1", "pushed elsewhere")
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	dir, _, warning, err := ensureFetchedWorktree(ctx, clone, task, "")
	if err != nil || warning != "" {
		t.Fatalf("worktree: %q %v", warning, err)
	}
	if got := gitTest(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("the branch starts at %s, want origin/feat/1 %s", got, head)
	}
}

func TestFetchedWorktreeKeepsALocalBranch(t *testing.T) {
	ctx := context.Background()
	origin, clone := bareOrigin(t)
	gitTest(t, clone, "branch", "feat/1")
	local := gitTest(t, clone, "rev-parse", "feat/1")
	advance(t, origin, "main", "second")
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	dir, _, _, err := ensureFetchedWorktree(ctx, clone, task, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := gitTest(t, dir, "rev-parse", "HEAD"); got != local {
		t.Errorf("the local branch moved to %s, want %s", got, local)
	}
}

func TestFetchedWorktreeWarnsWhenTheFetchFails(t *testing.T) {
	ctx := context.Background()
	_, clone := bareOrigin(t)
	head := gitTest(t, clone, "rev-parse", "HEAD")
	gitTest(t, clone, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	dir, _, warning, err := ensureFetchedWorktree(ctx, clone, task, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warning, "fetch failed") {
		t.Errorf("warning = %q", warning)
	}
	// origin/main is still known locally, so it is the base.
	if got := gitTest(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("the branch starts at %s, want %s", got, head)
	}
}

func TestLaunchWorktreeDoesNotFetch(t *testing.T) {
	ctx := context.Background()
	origin, clone := bareOrigin(t)
	head := gitTest(t, clone, "rev-parse", "HEAD")
	advance(t, origin, "main", "second")
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	dir, _, err := ensureLocalWorktree(ctx, clone, task, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := gitTest(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("the launch's code worktree starts at %s, want the local HEAD %s", got, head)
	}
}

func TestDesktopProjectAnyRepository(t *testing.T) {
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/a.git")
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root}}}); err != nil {
		t.Fatal(err)
	}
	_, _, do := desktopAgent(t, root, models.Task{})
	info := func() map[string]any {
		t.Helper()
		w := do("GET", "/desktop/project?id=p", nil)
		if w.Code != 200 {
			t.Fatalf("GET /desktop/project returned %d: %s", w.Code, w.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := info(); got["anyRepository"] != false || got["clonesPath"] != "" || !samePath(t, got["clonesDefault"].(string), filepath.Dir(root)) {
		t.Fatalf("the option is off and clones go next to the checkout: %v", got)
	}

	clones := t.TempDir()
	if w := do("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "anyRepository": true, "clonesPath": clones}); w.Code >= 300 {
		t.Fatalf("saving: %d %s", w.Code, w.Body.String())
	}
	if got := info(); got["anyRepository"] != true || got["clonesPath"] != clones || got["clonesDefault"] != clones {
		t.Fatalf("saved values: %v", got)
	}
	// A save that does not mention the option keeps it.
	if w := do("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root}); w.Code >= 300 {
		t.Fatalf("saving without the option: %d %s", w.Code, w.Body.String())
	}
	if settings, _ := agentconfig.ReadSettings(root); !settings.AnyRepository("p") {
		t.Fatalf("a save without the option turned it off: %+v", settings.Project("p"))
	}
	if w := do("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "clonesPath": "relative"}); w.Code != 400 || !strings.Contains(w.Body.String(), "Clones folder") {
		t.Fatalf("a relative clones folder is refused by name: %d %s", w.Code, w.Body.String())
	}
	if w := do("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "anyRepository": false, "clonesPath": ""}); w.Code >= 300 {
		t.Fatalf("clearing: %d %s", w.Code, w.Body.String())
	}
	if settings, _ := agentconfig.ReadSettings(root); settings.AnyRepository("p") || settings.Project("p").ClonesPath != "" || settings.Project("p").AnyRepository != nil {
		t.Fatalf("clearing leaves nothing stored: %+v", settings.Project("p"))
	}
}
