package agent

import (
	"context"
	"encoding/json"
	"os"
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

// anyRepositoryOn is settings with the project's Any repository option on.
func anyRepositoryOn(settings agentconfig.Settings) agentconfig.Settings {
	if settings.ProjectSettings == nil {
		settings.ProjectSettings = map[string]agentconfig.ProjectSettings{}
	}
	section := settings.ProjectSettings["p"]
	on := true
	section.AnyRepository = &on
	settings.ProjectSettings["p"] = section
	return settings
}

func TestRepositoryPathIsCheckedThenRemembered(t *testing.T) {
	testhome.Temp(t)
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	lib := checkoutOf(t, "git@github.com:o/lib.git")
	gitTest(t, lib, "branch", "feat/1")
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}
	settingsRoot := t.TempDir()
	request := repositoryRequest{Repository: "github.com/o/lib", Path: lib, Device: "laptop", SettingsRoot: settingsRoot}

	// The option off: the path is refused, and so is the repository alone.
	if _, err := repositoryWorktreeFor(ctx, multiRepoConfig(), agentconfig.Settings{}, projectRoot, task, request); err == nil || !strings.Contains(err.Error(), "Any repository option is off") {
		t.Fatalf("a path with the option off: %v", err)
	}
	request.Path = ""
	if _, err := repositoryWorktreeFor(ctx, multiRepoConfig(), agentconfig.Settings{}, projectRoot, task, request); err == nil || !strings.Contains(err.Error(), "Any repository") {
		t.Fatalf("the refusal names the option: %v", err)
	}
	if settings, _ := agentconfig.ReadSettings(settingsRoot); len(settings.Repositories) != 0 {
		t.Fatalf("a refusal remembered something: %v", settings.Repositories)
	}

	request.Path = lib
	got, err := repositoryWorktreeFor(ctx, multiRepoConfig(), anyRepositoryOn(agentconfig.Settings{}), projectRoot, task, request)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repository != "github.com/o/lib" || got.Source != models.RepositorySourcePath || !got.PathChecked || !got.Remembered || got.Branch != "feat/1" ||
		!strings.Contains(got.Path, filepath.Join(".tasks", "worktrees", "issue-1")) {
		t.Fatalf("worktree = %+v", got)
	}
	settings, err := agentconfig.ReadSettings(settingsRoot)
	if err != nil || !samePath(t, settings.Repositories["github.com/o/lib"], lib) {
		t.Fatalf("the checkout is not remembered: %v %v", settings.Repositories, err)
	}

	// The mapping now answers for every project, without a path; handoff
	// finds the worktree through it.
	request.Path = ""
	again, err := repositoryWorktreeFor(ctx, agentconfig.Config{ProjectID: "other", GitRemoteURL: "git@github.com:o/z.git"}, settings, projectRoot, task, request)
	if err != nil || !samePath(t, again.Path, got.Path) || again.Source != models.RepositorySourceMapping || again.Remembered {
		t.Fatalf("second request = %+v, %v", again, err)
	}
	removal := removeRepositoryWorktrees(ctx, multiRepoConfig(), settings, projectRoot, task, []string{"github.com/o/lib"})
	if strings.Join(removal.Removed, " ") != "github.com/o/lib" || len(removal.Failed) != 0 {
		t.Errorf("removal = %+v", removal)
	}
}

func TestRepositoryPathRefusals(t *testing.T) {
	testhome.Temp(t)
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	lib := checkoutOf(t, "git@github.com:o/lib.git")
	other := checkoutOf(t, "git@github.com:o/other.git")
	noOrigin := t.TempDir()
	gitTest(t, noOrigin, "init", "-q")
	sub := filepath.Join(lib, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}
	settingsRoot := t.TempDir()
	for path, want := range map[string]string{
		"relative/lib":                     "not an absolute path",
		filepath.Join(t.TempDir(), "gone"): "does not exist",
		t.TempDir():                        "not a Git checkout",
		sub:                                "give its top level",
		noOrigin:                           "no origin remote",
		other:                              "not github.com/o/lib",
	} {
		request := repositoryRequest{Repository: "github.com/o/lib", Path: path, SettingsRoot: settingsRoot}
		if _, err := repositoryWorktreeFor(ctx, multiRepoConfig(), anyRepositoryOn(agentconfig.Settings{}), projectRoot, task, request); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("path %s: %v, want %q", path, err, want)
		}
	}
	if settings, _ := agentconfig.ReadSettings(settingsRoot); len(settings.Repositories) != 0 {
		t.Fatalf("a refused path was remembered: %v", settings.Repositories)
	}
}

func TestKnownFolderWinsOverAPath(t *testing.T) {
	testhome.Temp(t)
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	gitTest(t, b, "branch", "feat/1")
	elsewhere := checkoutOf(t, "git@github.com:o/b.git")
	overrides := anyRepositoryOn(agentconfig.Settings{Repositories: map[string]string{"github.com/o/b": b}})
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	got, err := repositoryWorktreeFor(ctx, multiRepoConfig(), overrides, projectRoot, task, repositoryRequest{Repository: "github.com/o/b", Path: elsewhere, SettingsRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != models.RepositorySourceMapping || got.Remembered || !got.PathChecked || !strings.Contains(got.Warning, "was not used") || !strings.HasPrefix(evalPath(t, got.Path), evalPath(t, b)) {
		t.Fatalf("worktree = %+v", got)
	}
}

func TestRepositoryIsClonedWhenNothingHoldsIt(t *testing.T) {
	testhome.Temp(t)
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	origin, _ := bareOrigin(t)
	head := gitTest(t, origin, "rev-parse", "main")
	remote := "file://" + origin
	identity := models.RepositoryIdentity(remote)
	clones := filepath.Join(t.TempDir(), "clones")
	overrides := anyRepositoryOn(agentconfig.Settings{})
	section := overrides.ProjectSettings["p"]
	section.ClonesPath = clones
	overrides.ProjectSettings["p"] = section
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}
	settingsRoot := t.TempDir()

	got, err := repositoryWorktreeFor(ctx, multiRepoConfig(), overrides, projectRoot, task, repositoryRequest{Repository: identity, URL: remote, SettingsRoot: settingsRoot})
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(clones, "origin")
	if got.Source != models.RepositorySourceClone || !got.Remembered || got.PathChecked || got.Warning != "" || !strings.HasPrefix(evalPath(t, got.Path), evalPath(t, clone)) {
		t.Fatalf("worktree = %+v", got)
	}
	if sha := gitTest(t, got.Path, "rev-parse", "HEAD"); sha != head {
		t.Errorf("the branch starts at %s, want origin/main %s", sha, head)
	}
	if entries, _ := os.ReadDir(clones); len(entries) != 1 {
		t.Errorf("the clones folder holds %d entries, want the clone alone", len(entries))
	}
	if settings, _ := agentconfig.ReadSettings(settingsRoot); !samePath(t, settings.Repositories[identity], clone) {
		t.Errorf("the clone is not remembered: %v", settings.Repositories)
	}
}

func TestCloneNeverOverwritesAFolder(t *testing.T) {
	ctx := context.Background()
	origin, _ := bareOrigin(t)
	remote := "file://" + origin
	identity := models.RepositoryIdentity(remote)
	clones := t.TempDir()

	// Another repository under the same name is refused.
	gitTest(t, clones, "init", "-q", filepath.Join(clones, "origin"))
	if _, err := cloneRepository(ctx, clones, remote, identity); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a foreign folder: %v", err)
	}
	// A checkout of the same repository is used as it is.
	same := t.TempDir()
	gitTest(t, same, "clone", "-q", remote, filepath.Join(same, "origin"))
	if folder, err := cloneRepository(ctx, same, remote, identity); err != nil || !samePath(t, folder, filepath.Join(same, "origin")) {
		t.Fatalf("an existing clone: %q %v", folder, err)
	}
	// A failed clone leaves nothing behind.
	empty := t.TempDir()
	gone := "file://" + filepath.Join(t.TempDir(), "gone.git")
	if _, err := cloneRepository(ctx, empty, gone, models.RepositoryIdentity(gone)); err == nil || !strings.Contains(err.Error(), "git clone") {
		t.Fatalf("a failed clone: %v", err)
	}
	if entries, _ := os.ReadDir(empty); len(entries) != 0 {
		t.Errorf("a failed clone left %d entries", len(entries))
	}
}

func TestCloneURL(t *testing.T) {
	for _, c := range []struct{ requested, identity, code, want string }{
		{"https://gitlab.com/g/p", "gitlab.com/g/p", "git@github.com:o/a.git", "https://gitlab.com/g/p"},
		{"git@gitlab.com:g/p.git", "gitlab.com/g/p", "https://github.com/o/a", "git@gitlab.com:g/p.git"},
		{"gitlab.com/g/sub/p", "gitlab.com/g/sub/p", "git@github.com:o/a.git", "git@gitlab.com:g/sub/p.git"},
		{"gitlab.com/g/p", "gitlab.com/g/p", "ssh://git@github.com/o/a.git", "git@gitlab.com:g/p.git"},
		{"", "gitlab.com/g/p", "https://github.com/o/a.git", "https://gitlab.com/g/p.git"},
		{"file:///srv/p.git", "file////srv/p", "", "file:///srv/p.git"},
	} {
		if got := cloneURL(c.requested, c.identity, c.code); got != c.want {
			t.Errorf("cloneURL(%q, %q, %q) = %q, want %q", c.requested, c.identity, c.code, got, c.want)
		}
	}
}

// evalPath resolves the symbolic links of path, /private on macOS included.
func evalPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
