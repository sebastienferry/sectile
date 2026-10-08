package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// Each specifications folder resolves on its own setting, and a missing one is
// refused naming the setting to fix (#736).
func TestSpecFoldersResolveOnTheirOwnSetting(t *testing.T) {
	root := t.TempDir()
	macro, issue := t.TempDir(), t.TempDir()
	overrides := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {MacroSpecPath: macro, IssueSpecPath: issue}}}
	if got, err := localMacroSpecRepo(overrides, "p", root); err != nil || got != macro {
		t.Fatalf("macro folder: %q %v", got, err)
	}
	if got, err := localIssueSpecRepo(overrides, "p", root); err != nil || got != issue {
		t.Fatalf("issue folder: %q %v", got, err)
	}
	only := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {MacroSpecPath: macro}}}
	if got, err := localIssueSpecRepo(only, "p", root); err != nil || got != root {
		t.Fatalf("an empty Issue folder is the code checkout, whatever the Macro folder: %q %v", got, err)
	}
	if err := os.Mkdir(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	relative := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {IssueSpecPath: "notes"}}}
	if got, err := localIssueSpecRepo(relative, "p", root); err != nil || got != filepath.Join(root, "notes") {
		t.Fatalf("a relative folder is relative to the code checkout: %q %v", got, err)
	}
	gone := filepath.Join(t.TempDir(), "gone")
	for _, c := range []struct {
		settings agentconfig.ProjectSettings
		resolve  func(agentconfig.Settings, string, string) (string, error)
		setting  string
	}{
		{agentconfig.ProjectSettings{MacroSpecPath: gone}, localMacroSpecRepo, "Macro specifications folder"},
		{agentconfig.ProjectSettings{IssueSpecPath: gone}, localIssueSpecRepo, "Issue specifications folder"},
	} {
		settings := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": c.settings}}
		if _, err := c.resolve(settings, "p", root); err == nil || !strings.Contains(err.Error(), gone) || !strings.Contains(err.Error(), c.setting) {
			t.Fatalf("a missing folder must be refused naming %q and the path, got %v", c.setting, err)
		}
	}
}

// Without a distinct Issue folder, the task's own worktree carries its
// artefacts as before, and nothing is created.
func TestTaskSpecWorkspaceIsTheTaskWorktreeByDefault(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, ".tasks", "worktrees", "issue-7")
	got, err := ensureTaskSpecWorktree(context.Background(), root, root, workDir, "feat/7", "#7", true)
	if err != nil {
		t.Fatal(err)
	}
	want := models.TaskSpecWorkspace{Repository: root, Path: workDir, Branch: "feat/7", Worktree: true}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got, _ := ensureTaskSpecWorktree(context.Background(), root, root, root, "feat/7", "#7", false); got.Worktree || got.Distinct || got.Path != root {
		t.Fatalf("worktrees off: %+v", got)
	}
}

// A distinct Issue folder gets the task's own worktree, on a branch named like
// the task's, started from the up-to-date default branch, and reused as it is.
func TestTaskSpecWorktreeInADistinctIssueFolder(t *testing.T) {
	ctx := context.Background()
	code := t.TempDir()
	specs, remote := specRepoWithRemote(t)
	upstream := pushUpstream(t, remote, "upstream specification")

	got, err := ensureTaskSpecWorktree(ctx, specs, code, filepath.Join(code, "wt"), "feat/7", "#7", true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Distinct || !got.Worktree || got.Branch != "feat/7" || got.Repository != specs ||
		!samePath(t, got.Path, filepath.Join(specs, ".tasks", "worktrees", "issue-7")) {
		t.Fatalf("workspace = %+v", got)
	}
	if head := gitTest(t, got.Path, "rev-parse", "HEAD"); head != upstream {
		t.Fatalf("the branch must start from the fetched default branch: %s, want %s", head, upstream)
	}
	if branch := gitTest(t, got.Path, "branch", "--show-current"); branch != "feat/7" {
		t.Fatalf("branch = %q", branch)
	}
	if _, err := os.Stat(filepath.Join(code, "wt")); !os.IsNotExist(err) {
		t.Fatalf("the code worktree must be left alone: %v", err)
	}

	// Uncommitted work survives a second preparation.
	if err := os.WriteFile(filepath.Join(got.Path, "draft.md"), []byte("draft"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := ensureTaskSpecWorktree(ctx, specs, code, filepath.Join(code, "wt"), "feat/7", "#7", true)
	if err != nil || !samePath(t, again.Path, got.Path) {
		t.Fatalf("reuse: %+v %v", again, err)
	}
	if _, err := os.Stat(filepath.Join(again.Path, "draft.md")); err != nil {
		t.Fatalf("the reused tree lost its work: %v", err)
	}
}

// A Jira key gets its short lower-cased folder in a distinct Issue folder (#798).
func TestTaskSpecWorktreeUsesTheShortJiraFolder(t *testing.T) {
	ctx := context.Background()
	code := t.TempDir()
	specs, _ := specRepoWithRemote(t)

	got, err := ensureTaskSpecWorktree(ctx, specs, code, filepath.Join(code, "wt"), "feat/AUC-1234", "AUC-1234", true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Worktree || !samePath(t, got.Path, filepath.Join(specs, ".tasks", "worktrees", "auc-1234")) {
		t.Fatalf("workspace = %+v", got)
	}
}

func TestTaskSpecWorktreeEdgeCases(t *testing.T) {
	ctx := context.Background()
	code := t.TempDir()

	plain := t.TempDir()
	got, err := ensureTaskSpecWorktree(ctx, plain, code, code, "feat/7", "#7", true)
	if err != nil || got.Path != plain || got.Branch != "" || got.Worktree || !got.Distinct || !strings.Contains(got.Warning, "n'est pas un dépôt Git") {
		t.Fatalf("a plain folder is written in place: %+v %v", got, err)
	}

	specs, _ := specRepoWithRemote(t)
	got, err = ensureTaskSpecWorktree(ctx, specs, code, code, "feat/7", "#7", false)
	if err != nil || got.Path != specs || got.Branch != "feat/7" || got.Worktree || !strings.Contains(got.Warning, "worktrees désactivés") {
		t.Fatalf("worktrees off returns the checkout: %+v %v", got, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(specs, ".tasks")); len(entries) != 0 {
		t.Fatalf("worktrees off must create nothing: %v", entries)
	}

	if _, err := ensureTaskSpecWorktree(ctx, specs, code, code, "", "#7", true); err == nil || !strings.Contains(err.Error(), "#7") {
		t.Fatalf("a task without a branch cannot name its specifications branch: %v", err)
	}
	if _, err := ensureTaskSpecWorktree(ctx, specs, code, code, "main", "#7", true); err == nil || !strings.Contains(err.Error(), "branche par défaut") {
		t.Fatalf("the default branch is refused: %v", err)
	}

	noRemote := t.TempDir()
	gitTest(t, noRemote, "init", "-q", "-b", "main")
	gitTest(t, noRemote, "commit", "-q", "--allow-empty", "-m", "init")
	got, err = ensureTaskSpecWorktree(ctx, noRemote, code, code, "feat/7", "#7", true)
	if err != nil || !got.Worktree || !strings.Contains(got.Warning, "pas de distant origin") {
		t.Fatalf("without a remote the base is local and the warning says so: %+v %v", got, err)
	}
}

// An empty Issue folder is the task's code checkout, even for a task pinned to
// another repository than the project's own; a distinct one gets the dropped
// artefacts exclusion (#487).
func TestPrepareTaskSpecWorkspaceFollowsTheSetting(t *testing.T) {
	ctx := context.Background()
	projectRoot, taskRoot := t.TempDir(), t.TempDir()
	config := agentconfig.Config{ProjectID: "p", UseWorktrees: true, SpecArtifacts: models.SpecArtifactsDrop}
	task := models.Task{Key: "#7"}
	got, err := prepareTaskSpecWorkspace(ctx, config, agentconfig.Settings{}, projectRoot, taskRoot, task, taskRoot, "feat/7")
	if err != nil || got.Distinct || got.Path != taskRoot || got.Repository != taskRoot {
		t.Fatalf("pinned task without an Issue folder: %+v %v", got, err)
	}

	specs, _ := specRepoWithRemote(t)
	overrides := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {IssueSpecPath: specs}}}
	got, err = prepareTaskSpecWorkspace(ctx, config, overrides, projectRoot, taskRoot, task, taskRoot, "feat/7")
	if err != nil || !got.Distinct {
		t.Fatalf("distinct: %+v %v", got, err)
	}
	path, err := excludeFilePath(ctx, specs)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "/docs/clarifications/7.md") {
		t.Fatalf("the Issue folder must carry the dropped artefacts exclusion:\n%s", raw)
	}

	gone := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {IssueSpecPath: filepath.Join(t.TempDir(), "gone")}}}
	if _, err := prepareTaskSpecWorkspace(ctx, config, gone, projectRoot, taskRoot, task, taskRoot, "feat/7"); err == nil || !strings.Contains(err.Error(), issueSpecSetting) {
		t.Fatalf("a missing Issue folder refuses the launch by name: %v", err)
	}
}

func TestTaskSpecEnvironment(t *testing.T) {
	env := taskSpecEnvironment(models.TaskSpecWorkspace{Path: "/specs/.tasks/worktrees/issue-7", Branch: "feat/7", Worktree: true, Distinct: true})
	if env["SECTILE_SPEC_REPO"] != "/specs/.tasks/worktrees/issue-7" || env["SECTILE_SPEC_BRANCH"] != "feat/7" || env["SECTILE_SPEC_WORKTREE"] != "true" {
		t.Fatalf("env = %v", env)
	}
	if env := taskSpecEnvironment(models.TaskSpecWorkspace{}); len(env) != 0 {
		t.Fatalf("an unknown workspace sets nothing, so the skill asks the agent: %v", env)
	}
}

// A ticket's map lists only the Issue folder, with the task's specifications
// worktree; a project session lists both folders.
func TestFolderMapListsTheSpecificationsFolderOfItsSkillSet(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	macro, issue := t.TempDir(), t.TempDir()
	config := agentconfig.Config{ProjectID: "p", GitRemoteURL: "git@github.com:o/a.git"}
	overrides := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {MacroSpecPath: macro, IssueSpecPath: issue}}}
	specPaths := func(entries []models.FolderMapEntry) []string {
		var paths []string
		for _, entry := range entries {
			if entry.Role == models.FolderRoleSpec {
				paths = append(paths, entry.Path)
			}
		}
		return paths
	}
	task := models.Task{Key: "#1"}
	entries := buildFolderMap(ctx, config, overrides, projectRoot, "github.com/o/a", projectRoot, task)
	if got := specPaths(entries); len(got) != 1 || got[0] != issue {
		t.Fatalf("a ticket lists the Issue folder only: %v", got)
	}
	worktree := filepath.Join(issue, ".tasks", "worktrees", "issue-1")
	entries = withSpecWorktree(entries, models.TaskSpecWorkspace{Repository: issue, Path: worktree, Distinct: true, Worktree: true})
	for _, entry := range entries {
		if entry.Role == models.FolderRoleSpec && entry.Worktree != worktree {
			t.Fatalf("the spec entry names the task's specifications worktree: %+v", entry)
		}
	}
	if dirs := folderMapDirs(entries); len(dirs) != 1 || dirs[0] != issue {
		t.Fatalf("a worktree inside the folder is covered by it: %v", dirs)
	}
	if got := specPaths(buildFolderMap(ctx, config, overrides, projectRoot, "github.com/o/a", projectRoot, models.Task{})); len(got) != 2 || got[0] != macro || got[1] != issue {
		t.Fatalf("a project session lists both folders, Macro first: %v", got)
	}
	same := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {MacroSpecPath: macro, IssueSpecPath: macro}}}
	if got := specPaths(buildFolderMap(ctx, config, same, projectRoot, "github.com/o/a", projectRoot, models.Task{})); len(got) != 1 {
		t.Fatalf("one folder serving both is listed once: %v", got)
	}
}

// A skill typed by hand gets, through task_spec_worktree, the workspace a
// launch would have prepared, without any code worktree being created.
func TestTaskSpecWorktreeOperation(t *testing.T) {
	testhome.Temp(t)
	ctx := context.Background()
	root := checkoutOf(t, "git@github.com:o/a.git")
	task := models.Task{ID: "t", Key: "#1", ProjectID: "p", BranchName: branchOf("feat/1")}
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root}}}); err != nil {
		t.Fatal(err)
	}
	d, _, _ := desktopAgent(t, root, task)
	op := agentprotocol.Operation{ProjectID: "p", TaskID: "t", Action: "task_spec_worktree"}

	value, err := d.executeOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.(models.TaskSpecWorkspace); got.Distinct || !samePath(t, got.Path, root) || got.Branch != "feat/1" {
		t.Fatalf("without an Issue folder: %+v", got)
	}

	specs, _ := specRepoWithRemote(t)
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root, IssueSpecPath: specs}}}); err != nil {
		t.Fatal(err)
	}
	value, err = d.executeOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	got := value.(models.TaskSpecWorkspace)
	if !got.Distinct || !got.Worktree || got.Branch != "feat/1" || !samePath(t, got.Path, filepath.Join(specs, ".tasks", "worktrees", "issue-1")) {
		t.Fatalf("with an Issue folder: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".tasks")); !os.IsNotExist(err) {
		t.Fatalf("the operation must not create the code worktree: %v", err)
	}
}

// The desktop settings read and write the Issue folder beside the Macro one,
// each with its effective value, and the attach dialog refuses either.
func TestDesktopProjectIssueSpecificationsFolder(t *testing.T) {
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
	got := info()
	if got["issueSpecPath"] != "" || !samePath(t, got["issueSpecDefault"].(string), root) || got["issueSpecKind"] != "git" {
		t.Fatalf("an existing workstation has no Issue folder and inherits its checkout: %v", got)
	}

	macro, issue := t.TempDir(), t.TempDir()
	if w := do("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "specPath": macro}); w.Code >= 300 {
		t.Fatalf("saving the Macro folder: %d %s", w.Code, w.Body.String())
	}
	if w := do("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "issueSpecPath": issue}); w.Code >= 300 {
		t.Fatalf("saving the Issue folder: %d %s", w.Code, w.Body.String())
	}
	settings, err := agentconfig.ReadSettings(root)
	if err != nil || settings.MacroSpecPath("p") != macro || settings.IssueSpecPath("p") != issue {
		t.Fatalf("each save keeps the other folder: %+v %v", settings.Project("p"), err)
	}
	if got := info(); got["issueSpecPath"] != issue || got["issueSpecKind"] != "folder" || got["specPath"] != macro {
		t.Fatalf("both folders are reported: %v", got)
	}

	for folder, setting := range map[string]string{macro: "Macro specifications folder", issue: "Issue specifications folder"} {
		if w := do("POST", "/desktop/folders", map[string]any{"projectId": "p", "path": folder}); w.Code != 409 || !strings.Contains(w.Body.String(), setting) {
			t.Fatalf("attaching the %s: %d %s", setting, w.Code, w.Body.String())
		}
	}

	if w := do("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "issueSpecPath": "relative"}); w.Code != 400 || !strings.Contains(w.Body.String(), "Issue specifications folder") {
		t.Fatalf("a relative Issue folder is refused by name: %d %s", w.Code, w.Body.String())
	}
	if w := do("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "issueSpecPath": ""}); w.Code >= 300 {
		t.Fatalf("clearing: %d %s", w.Code, w.Body.String())
	}
	if settings, _ := agentconfig.ReadSettings(root); settings.IssueSpecPath("p") != "" || settings.MacroSpecPath("p") != macro {
		t.Fatalf("clearing the Issue folder keeps the Macro one: %+v", settings.Project("p"))
	}
}

// A terminal on a task creates no specifications worktree: it names one only
// once it exists, and never the Issue folder's default branch.
func TestKnownTaskSpecWorkspaceCreatesNothing(t *testing.T) {
	testhome.Temp(t)
	ctx := context.Background()
	root := checkoutOf(t, "git@github.com:o/a.git")
	specs, _ := specRepoWithRemote(t)
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root, IssueSpecPath: specs}}}); err != nil {
		t.Fatal(err)
	}
	d, _, _ := desktopAgent(t, root, models.Task{})
	config := multiRepoConfig()
	task := models.Task{ID: "t", Key: "#1", ProjectID: "p"}
	if got := d.knownTaskSpecWorkspace(ctx, config, task, root, "feat/1"); got.Path != "" {
		t.Fatalf("no worktree yet, nothing named: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(specs, ".tasks")); !os.IsNotExist(err) {
		t.Fatalf("a terminal must create nothing: %v", err)
	}
	if got := d.knownTaskSpecWorkspace(ctx, config, task, root, "main"); got.Path != "" {
		t.Fatalf("the default branch is never named: %+v", got)
	}
	prepared, err := ensureTaskSpecWorktree(ctx, specs, root, root, "feat/1", "#1", true)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.knownTaskSpecWorkspace(ctx, config, task, root, "feat/1"); !got.Distinct || !samePath(t, got.Path, prepared.Path) {
		t.Fatalf("an existing worktree is named: %+v", got)
	}
}
