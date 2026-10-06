package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// A project whose code remote is o/a, and which declares o/b and o/c.
func multiRepoConfig() agentconfig.Config {
	return agentconfig.Config{ProjectID: "p", ProjectName: "Multi", GitRemoteURL: "git@github.com:o/a.git",
		Repositories: []string{"git@github.com:o/a.git", "git@github.com:o/b.git", "git@github.com:o/c.git"}}
}

func branchOf(name string) *string { return &name }

func TestPrimaryRootFollowsThePin(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	overrides := agentconfig.Settings{Repositories: map[string]string{"github.com/o/b": b}}

	root, identity, err := primaryRoot(ctx, multiRepoConfig(), overrides, projectRoot, models.Task{Key: "#1", Repository: "github.com/o/b"})
	if err != nil || root != b || identity != "github.com/o/b" {
		t.Fatalf("pinned: %q %q %v", root, identity, err)
	}
	// Another repository keeps .tasks/ out of its status by itself.
	exclude, _ := os.ReadFile(filepath.Join(b, ".git", "info", "exclude"))
	if !strings.Contains(string(exclude), "/.tasks/") {
		t.Errorf("info/exclude = %q", exclude)
	}

	if _, _, err := primaryRoot(ctx, multiRepoConfig(), overrides, projectRoot, models.Task{Key: "#1", Repository: "github.com/o/c"}); err == nil || !strings.Contains(err.Error(), "github.com/o/c") {
		t.Errorf("unmapped pin: %v", err)
	}
	// An unpinned ticket runs in the code repository, whatever else is mapped
	// here (#484): it never waits for a choice.
	if root, identity, err := primaryRoot(ctx, multiRepoConfig(), overrides, projectRoot, models.Task{Key: "#1"}); err != nil || root != projectRoot || identity != "github.com/o/a" {
		t.Errorf("unpinned with a and b mapped: %q %q %v", root, identity, err)
	}
	if root, _, err := primaryRoot(ctx, multiRepoConfig(), agentconfig.Settings{}, projectRoot, models.Task{Key: "#1"}); err != nil || root != projectRoot {
		t.Errorf("only the code repository mapped: %q %v", root, err)
	}
	// A pin to a repository the project does not declare needs the Any
	// repository option (#737); without it the launch fails, naming it,
	// rather than run the ticket in another repository.
	if _, _, err := primaryRoot(ctx, multiRepoConfig(), overrides, projectRoot, models.Task{Key: "#1", Repository: "github.com/o/gone"}); err == nil || !strings.Contains(err.Error(), "Any repository option") {
		t.Errorf("undeclared pin without the option: %v", err)
	}
	// What names no repository still reads as no pin.
	if root, _, err := primaryRoot(ctx, multiRepoConfig(), overrides, projectRoot, models.Task{Key: "#1", Repository: "gone"}); err != nil || root != projectRoot {
		t.Errorf("a pin naming no repository: %q %v", root, err)
	}
}

func TestFolderMapDescribesEveryFolder(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	spec := t.TempDir()
	gitTest(t, b, "branch", "feat/1")
	secondary := filepath.Join(t.TempDir(), "b-wt")
	gitTest(t, b, "worktree", "add", "-q", secondary, "feat/1")
	overrides := agentconfig.Settings{Repositories: map[string]string{"github.com/o/b": b}, ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {IssueSpecPath: spec}}}
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1"), ChangedRepositories: []string{"github.com/o/b"}}

	entries := buildFolderMap(ctx, multiRepoConfig(), overrides, projectRoot, "github.com/o/a", "/work/a", task)
	roles := map[string]models.FolderMapEntry{}
	for _, entry := range entries {
		roles[entry.Role+":"+entry.Identity] = entry
	}
	if e := roles["primary:github.com/o/a"]; e.Path != projectRoot || e.Worktree != "/work/a" {
		t.Errorf("primary = %+v", e)
	}
	if e := roles["changed:github.com/o/b"]; e.Path != b || !samePath(t, e.Worktree, secondary) {
		t.Errorf("changed = %+v", e)
	}
	if e := roles["context:github.com/o/c"]; e.Path != "" {
		t.Errorf("an unmapped context folder has no path: %+v", e)
	}
	if e := roles["spec:"]; e.Path != spec {
		t.Errorf("spec = %+v", e)
	}
	dirs := folderMapDirs(entries)
	if len(dirs) != 2 || !samePath(t, dirs[0], secondary) || dirs[1] != spec {
		t.Errorf("add-dirs = %v, want the secondary worktree and the spec folder", dirs)
	}
	prompt := folderMapPrompt(entries)
	for _, want := range []string{"github.com/o/c (context): not mapped on this workstation", "prepare_repository_worktree", "read-only", "prUrls"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt misses %q:\n%s", want, prompt)
		}
	}
	single := buildFolderMap(ctx, agentconfig.Config{ProjectID: "p", GitRemoteURL: "git@github.com:o/a.git"}, agentconfig.Settings{}, projectRoot, "github.com/o/a", projectRoot, models.Task{Key: "#1"})
	if len(single) != 1 || folderMapPrompt(single) != "" {
		t.Errorf("a single checkout needs no map in the prompt: %+v", single)
	}
}

func TestContextFoldersReachClaudeAndCodex(t *testing.T) {
	launch := agentCommandContext{AddDirs: []string{"/src/b", "/src/it's"}}
	headless, err := modeCommandLine("claude", "", "", "go", models.SkillModeAutonomous, launch)
	// --add-dir takes several values: the prompt must come before it, and
	// each folder is bound to its own option, or the prompt is swallowed.
	if err != nil || !strings.HasSuffix(headless, `'go' --add-dir='/src/b' --add-dir='/src/it'\''s'`) {
		t.Errorf("claude headless = %q, %v", headless, err)
	}
	interactive, _ := modeCommandLine("claude", "", "", "go", models.SkillModeInteractive, launch)
	if interactive != `claude 'go' --add-dir='/src/b' --add-dir='/src/it'\''s'` {
		t.Errorf("claude interactive = %q", interactive)
	}
	// The words the CLI receives, read through printf rather than by
	// starting the CLI: shellArguments runs the line it is given.
	echoed := "printf '%s\\0'" + strings.TrimPrefix(interactive, "claude")
	if argv := shellArguments(t, "sh", echoed); len(argv) != 3 || argv[0] != "go" || argv[1] != "--add-dir=/src/b" || argv[2] != "--add-dir=/src/it's" {
		t.Errorf("claude receives %q", argv)
	}
	// Codex repeats --add-dir once per folder, interactive and headless.
	codexInteractive, _ := modeCommandLine("codex", "", "", "go", models.SkillModeInteractive, launch)
	if codexInteractive != `codex 'go' --add-dir='/src/b' --add-dir='/src/it'\''s'` {
		t.Errorf("codex interactive = %q", codexInteractive)
	}
	if argv := shellArguments(t, "sh", "printf '%s\\0'"+strings.TrimPrefix(codexInteractive, "codex")); len(argv) != 3 || argv[1] != "--add-dir=/src/b" || argv[2] != "--add-dir=/src/it's" {
		t.Errorf("codex receives %q", argv)
	}
	if codexHeadless, err := modeCommandLine("codex", "", "", "go", models.SkillModeAutonomous, launch); err != nil || !strings.HasPrefix(codexHeadless, "codex exec") || !strings.HasSuffix(codexHeadless, `'go' --add-dir='/src/b' --add-dir='/src/it'\''s'`) {
		t.Errorf("codex headless = %q, %v", codexHeadless, err)
	}
	for _, provider := range []string{"agy"} {
		for _, mode := range []string{models.SkillModeInteractive, models.SkillModeAutonomous} {
			if line, _ := modeCommandLine(provider, "", "", "go", mode, launch); strings.Contains(line, "add-dir") || strings.Contains(line, "/src/b") {
				t.Errorf("%s %s guessed a flag: %q", provider, mode, line)
			}
		}
	}
	if line, _ := modeCommandLine("claude", "claude {addDirs} '{prompt}'", "", "go", models.SkillModeInteractive, launch); !strings.HasPrefix(line, "claude --add-dir='/src/b'") {
		t.Errorf("template {addDirs} = %q", line)
	}
	if line, _ := modeCommandLine("codex", "codex {addDirs} '{prompt}'", "", "go", models.SkillModeInteractive, launch); !strings.HasPrefix(line, "codex --add-dir='/src/b' --add-dir=") {
		t.Errorf("template {addDirs} for codex = %q", line)
	}
	if line, _ := modeCommandLine("agy", "agy {addDirs} '{prompt}'", "", "go", models.SkillModeInteractive, launch); strings.Contains(line, "add-dir") {
		t.Errorf("template for agy = %q", line)
	}
	if line, _ := modeCommandLine("claude", "", "", "go", models.SkillModeAutonomous); strings.Contains(line, "add-dir") {
		t.Errorf("no context folder, no flag: %q", line)
	}
}

func TestRepositoryWorktreeReusesTheTaskBranch(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	gitTest(t, b, "branch", "feat/1")
	overrides := agentconfig.Settings{Repositories: map[string]string{"github.com/o/b": b}}
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	first, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "https://github.com/o/b", "laptop")
	if err != nil {
		t.Fatal(err)
	}
	if first.Repository != "github.com/o/b" || first.Branch != "feat/1" || !strings.Contains(first.Path, filepath.Join(".tasks", "worktrees", "issue-1")) {
		t.Fatalf("worktree = %+v", first)
	}
	if got := gitTest(t, first.Path, "branch", "--show-current"); got != "feat/1" {
		t.Errorf("worktree branch = %q", got)
	}
	again, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "github.com/o/b", "laptop")
	if err != nil || !samePath(t, again.Path, first.Path) {
		t.Errorf("second request = %+v, %v", again, err)
	}
	if _, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "github.com/o/c", "laptop"); err == nil || !strings.Contains(err.Error(), "github.com/o/c") {
		t.Errorf("unmapped: %v", err)
	}
	if _, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "github.com/o/elsewhere", "laptop"); err == nil {
		t.Error("a repository neither mapped nor attached was accepted")
	}

	removal := removeRepositoryWorktrees(ctx, multiRepoConfig(), overrides, projectRoot, task, []string{"github.com/o/a", "github.com/o/b", "github.com/o/c"})
	if strings.Join(removal.Removed, " ") != "github.com/o/a github.com/o/b" || len(removal.Failed) != 1 || removal.Failed[0].Repository != "github.com/o/c" {
		t.Errorf("removal = %+v", removal)
	}
	if _, err := os.Stat(first.Path); !os.IsNotExist(err) {
		t.Errorf("the secondary worktree is still there: %v", err)
	}
}

func TestLegacyRepoPathsResolveOnThisWorkstation(t *testing.T) {
	ctx := context.Background()
	b := checkoutOf(t, "git@github.com:o/b.git")
	plain := t.TempDir()
	noOrigin := t.TempDir()
	gitTest(t, noOrigin, "init", "-q")
	report := resolveLegacyRepoPaths(ctx, []models.LegacyRepoPath{
		{Path: b, TaskIDs: []string{"t1"}}, {Path: "/does/not/exist"}, {Path: plain}, {Path: noOrigin}, {Path: "relative"},
	})
	if len(report.Converted) != 1 || report.Converted[0].URL != "git@github.com:o/b.git" || report.Converted[0].TaskIDs[0] != "t1" {
		t.Errorf("converted = %+v", report.Converted)
	}
	reasons := map[string]string{}
	for _, dropped := range report.Dropped {
		reasons[dropped.Path] = dropped.Reason
	}
	if reasons["/does/not/exist"] != "not found" || reasons[plain] != "not a git checkout" || reasons[noOrigin] != "no origin" || reasons["relative"] != "not found" {
		t.Errorf("dropped = %+v", reasons)
	}
}

func TestConvertedFoldersAreKeptOnThisWorkstation(t *testing.T) {
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root}}}); err != nil {
		t.Fatal(err)
	}
	d := &agentDaemon{repoRoot: root}
	d.rememberConvertedFolders(context.Background(), multiRepoConfig(), []models.ConvertedRepoPath{
		{Path: root, URL: "git@github.com:o/a.git"}, {Path: filepath.Join(b, "."), URL: "git@github.com:o/b.git"},
	})
	settings, _ := agentconfig.ReadSettings(root)
	if len(settings.Repositories) != 1 || !samePath(t, settings.Repositories["github.com/o/b"], b) {
		t.Errorf("mappings = %v, want o/b only (o/a is the project's own)", settings.Repositories)
	}
}

// attachedTo attaches folders to project p in settings.
func attachedTo(settings agentconfig.Settings, folders ...string) agentconfig.Settings {
	if settings.ProjectSettings == nil {
		settings.ProjectSettings = map[string]agentconfig.ProjectSettings{}
	}
	section := settings.ProjectSettings["p"]
	section.Folders = folders
	settings.ProjectSettings["p"] = section
	return settings
}

// An attached folder is read from the disk at each use (#484): a Git checkout
// at its top level with its origin, one without origin, a plain folder, or a
// folder gone since it was attached.
func TestAttachedFoldersAreReadFromTheDisk(t *testing.T) {
	ctx := context.Background()
	lib := checkoutOf(t, "git@github.com:o/lib.git")
	inside := filepath.Join(lib, "docs")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	noOrigin := t.TempDir()
	gitTest(t, noOrigin, "init", "-q")
	plain := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")

	folders := attachedFolders(ctx, attachedTo(agentconfig.Settings{}, inside, noOrigin, plain, missing, "  "), "p")
	if len(folders) != 4 {
		t.Fatalf("folders = %+v", folders)
	}
	if f := folders[0]; f.Kind != folderKindGit || f.Stored != inside || !samePath(t, f.Path, lib) || f.Identity != "github.com/o/lib" || f.Remote != "git@github.com:o/lib.git" {
		t.Errorf("subfolder of a checkout = %+v", f)
	}
	if f := folders[1]; f.Kind != folderKindGit || f.Identity != "" || f.Remote != "" {
		t.Errorf("checkout without origin = %+v", f)
	}
	if f := folders[2]; f.Kind != folderKindFolder || f.Path != plain {
		t.Errorf("plain folder = %+v", f)
	}
	if f := folders[3]; f.Kind != folderKindMissing || f.Path != missing {
		t.Errorf("missing folder = %+v", f)
	}
	if other := attachedFolders(ctx, attachedTo(agentconfig.Settings{}, plain), "q"); len(other) != 0 {
		t.Errorf("another project's folders leaked: %+v", other)
	}
}

func TestRepositoryFolderPrefersTheMapping(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	attached := checkoutOf(t, "git@github.com:o/lib.git")
	mapped := checkoutOf(t, "git@github.com:o/lib.git")
	overrides := attachedTo(agentconfig.Settings{}, attached)

	if root, ok := repositoryFolder(ctx, overrides, "p", projectRoot, "github.com/o/a", "github.com/o/lib"); !ok || !samePath(t, root, attached) {
		t.Errorf("attached: %q %v", root, ok)
	}
	if root, ok := repositoryFolder(ctx, overrides, "p", projectRoot, "github.com/o/a", "github.com/o/a"); !ok || root != projectRoot {
		t.Errorf("code repository: %q %v", root, ok)
	}
	overrides.Repositories = map[string]string{"github.com/o/lib": mapped}
	if root, ok := repositoryFolder(ctx, overrides, "p", projectRoot, "github.com/o/a", "github.com/o/lib"); !ok || root != mapped {
		t.Errorf("the mapping must win over the attached folder: %q %v", root, ok)
	}
	for _, identity := range []string{"github.com/o/elsewhere", ""} {
		if _, ok := repositoryFolder(ctx, overrides, "p", projectRoot, "github.com/o/a", identity); ok {
			t.Errorf("%q found", identity)
		}
	}
}

func TestFolderMapListsAttachedFolders(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	ui := checkoutOf(t, "git@github.com:o/ui.git")
	lib := checkoutOf(t, "git@github.com:o/lib.git")
	gitTest(t, lib, "branch", "feat/1")
	libWorktree := filepath.Join(t.TempDir(), "lib-wt")
	gitTest(t, lib, "worktree", "add", "-q", libWorktree, "feat/1")
	bAgain := checkoutOf(t, "git@github.com:o/b.git")
	notes := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	spec := t.TempDir()
	overrides := attachedTo(agentconfig.Settings{Repositories: map[string]string{"github.com/o/b": b}}, ui, lib, bAgain, notes, missing, projectRoot, spec, notes)
	overrides.ProjectSettings["p"] = func(section agentconfig.ProjectSettings) agentconfig.ProjectSettings {
		section.IssueSpecPath = spec
		return section
	}(overrides.ProjectSettings["p"])
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1"), ChangedRepositories: []string{"github.com/o/lib"}}

	entries := buildFolderMap(ctx, multiRepoConfig(), overrides, projectRoot, "github.com/o/a", "/work/a", task)
	byPath := map[string]models.FolderMapEntry{}
	var attached []models.FolderMapEntry
	for _, entry := range entries {
		if entry.Attached {
			attached = append(attached, entry)
		}
		byPath[entry.Role+":"+entry.Identity] = entry
	}
	// o/ui as context, o/lib as changed, the notes as local, the missing
	// folder; o/b is a project repository already listed with its mapping,
	// and the project root, the specifications folder and a folder attached
	// twice are not listed again.
	if len(attached) != 4 {
		t.Fatalf("attached entries = %+v", attached)
	}
	if e := byPath["context:github.com/o/ui"]; !e.Attached || e.Kind != folderKindGit || !samePath(t, e.Path, ui) || e.Remote != "git@github.com:o/ui.git" {
		t.Errorf("attached context = %+v", e)
	}
	if e := byPath["changed:github.com/o/lib"]; !e.Attached || !samePath(t, e.Worktree, libWorktree) {
		t.Errorf("attached changed = %+v", e)
	}
	if e := byPath["local:"]; !e.Attached || e.Kind != folderKindFolder || e.Path != notes {
		t.Errorf("local folder = %+v", e)
	}
	if e := attached[3]; e.Kind != folderKindMissing || e.Role != models.FolderRoleContext || e.Path != missing {
		t.Errorf("missing folder = %+v", e)
	}
	if e := byPath["context:github.com/o/b"]; e.Attached || e.Path != b {
		t.Errorf("a project repository must keep its mapping: %+v", e)
	}
	if e := byPath["spec:"]; e.Path != spec {
		t.Errorf("spec = %+v", e)
	}

	dirs := folderMapDirs(entries)
	for _, want := range []string{b, ui, libWorktree, notes, spec} {
		if !containsPath(t, dirs, want) {
			t.Errorf("add-dirs %v miss %s", dirs, want)
		}
	}
	if containsPath(t, dirs, missing) || strings.Contains(strings.Join(dirs, " "), missing) {
		t.Errorf("a missing folder must not reach the CLI: %v", dirs)
	}

	prompt := folderMapPrompt(entries)
	for _, want := range []string{
		"github.com/o/ui (context, attached Git repository): ",
		"github.com/o/lib (changed, attached Git repository): ",
		notes + " (local, attached plain folder): " + notes,
		missing + " (context, attached folder): not found on this workstation",
		"Local folders have no remote: change them in place, with no worktree and no pull request.",
		"prepare_repository_worktree", "prUrls",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt misses %q:\n%s", want, prompt)
		}
	}

	// A single folder still needs no map, and a folder attached to it
	// makes the block appear.
	single := agentconfig.Config{ProjectID: "p", GitRemoteURL: "git@github.com:o/a.git"}
	if entries := buildFolderMap(ctx, single, agentconfig.Settings{}, projectRoot, "github.com/o/a", projectRoot, models.Task{Key: "#1"}); len(entries) != 1 || folderMapPrompt(entries) != "" {
		t.Errorf("a single checkout needs no map in the prompt: %+v", entries)
	}
	if entries := buildFolderMap(ctx, single, attachedTo(agentconfig.Settings{}, notes), projectRoot, "github.com/o/a", projectRoot, models.Task{Key: "#1"}); len(entries) != 2 || folderMapPrompt(entries) == "" {
		t.Errorf("an attached folder must be listed: %+v", entries)
	}
}

func containsPath(t *testing.T, list []string, want string) bool {
	t.Helper()
	for _, path := range list {
		if samePath(t, path, want) {
			return true
		}
	}
	return false
}

// A ticket changes an attached Git folder through a worktree on its branch,
// and changes a folder without a remote in place (#484).
func TestRepositoryWorktreeInAnAttachedFolder(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	lib := checkoutOf(t, "git@github.com:o/lib.git")
	gitTest(t, lib, "branch", "feat/1")
	noOrigin := t.TempDir()
	gitTest(t, noOrigin, "init", "-q")
	missing := filepath.Join(t.TempDir(), "gone")
	overrides := attachedTo(agentconfig.Settings{}, lib, noOrigin)
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	first, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "git@github.com:o/lib.git", "laptop")
	if err != nil {
		t.Fatal(err)
	}
	if first.Repository != "github.com/o/lib" || first.Branch != "feat/1" || !strings.Contains(first.Path, filepath.Join(".tasks", "worktrees", "issue-1")) {
		t.Fatalf("worktree = %+v", first)
	}
	if got := gitTest(t, first.Path, "branch", "--show-current"); got != "feat/1" {
		t.Errorf("worktree branch = %q", got)
	}
	exclude, _ := os.ReadFile(filepath.Join(lib, ".git", "info", "exclude"))
	if !strings.Contains(string(exclude), "/.tasks/") {
		t.Errorf("the attached checkout does not ignore .tasks/: %q", exclude)
	}
	if again, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "https://github.com/o/lib", "laptop"); err != nil || !samePath(t, again.Path, first.Path) {
		t.Errorf("second request = %+v, %v", again, err)
	}

	if _, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, noOrigin, "laptop"); err == nil || !strings.Contains(err.Error(), "change it in place") {
		t.Errorf("folder without a remote: %v", err)
	}
	if _, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "github.com/o/elsewhere", "laptop"); err == nil || !strings.Contains(err.Error(), "attachez") {
		t.Errorf("neither mapped nor attached: %v", err)
	}

	removal := removeRepositoryWorktrees(ctx, multiRepoConfig(), attachedTo(overrides, lib, missing), projectRoot, task, []string{"github.com/o/lib", "github.com/o/gone"})
	if strings.Join(removal.Removed, " ") != "github.com/o/lib" || len(removal.Failed) != 1 || removal.Failed[0].Repository != "github.com/o/gone" || removal.Failed[0].Error != "not found on this workstation" {
		t.Errorf("removal = %+v", removal)
	}
	if _, err := os.Stat(first.Path); !os.IsNotExist(err) {
		t.Errorf("the attached worktree is still there: %v", err)
	}
}

// brokenCheckout is a Git checkout git refuses to read for another reason
// than "not a checkout" or "no origin": its configuration does not parse.
func brokenCheckout(t *testing.T) string {
	t.Helper()
	dir := checkoutOf(t, "git@github.com:o/broken.git")
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("garbage[\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A git failure on an attached folder is kept, and never read as a plain
// folder or a checkout without origin (#589). The kinds stay the ones the
// desktop lists.
func TestDescribeFolderKeepsTheGitError(t *testing.T) {
	ctx := context.Background()
	lib := checkoutOf(t, "git@github.com:o/lib.git")
	noOrigin := t.TempDir()
	gitTest(t, noOrigin, "init", "-q")
	plain := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	broken := brokenCheckout(t)

	for path, want := range map[string]attachedFolder{
		lib:      {Kind: folderKindGit, Identity: "github.com/o/lib"},
		noOrigin: {Kind: folderKindGit},
		plain:    {Kind: folderKindFolder},
		missing:  {Kind: folderKindMissing},
	} {
		got := describeFolder(ctx, path)
		if got.Kind != want.Kind || got.Identity != want.Identity || got.Err != "" {
			t.Errorf("%s = %+v, want kind %q identity %q and no error", path, got, want.Kind, want.Identity)
		}
	}
	got := describeFolder(ctx, broken)
	if got.Kind != folderKindFolder || !strings.Contains(got.Err, "bad config") || got.Identity != "" {
		t.Errorf("broken checkout = %+v", got)
	}
}

// A git that speaks the workstation's language still has a plain folder read
// as a plain folder and a checkout without origin as one: the diagnosis asks
// git for its untranslated messages.
func TestDescribeFolderReadsGitInEnglish(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the translated git is a shell script")
	}
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip(err)
	}
	ctx := context.Background()
	noOrigin := t.TempDir()
	gitTest(t, noOrigin, "init", "-q")
	plain := t.TempDir()
	bin := t.TempDir()
	script := `#!/bin/sh
if [ "$LC_ALL" != "C" ]; then
  case "$*" in
    *rev-parse*) echo "fatal: ni ceci ni aucun de ses répertoires parents n'est un dépôt git" >&2; exit 128 ;;
    *get-url*) echo "erreur : Pas de dépôt distant 'origin'" >&2; exit 2 ;;
  esac
fi
exec "` + real + `" "$@"
`
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LC_ALL", "fr_FR.UTF-8")

	if got := describeFolder(ctx, plain); got.Kind != folderKindFolder || got.Err != "" {
		t.Errorf("plain folder = %+v", got)
	}
	if got := describeFolder(ctx, noOrigin); got.Kind != folderKindGit || got.Identity != "" || got.Err != "" {
		t.Errorf("checkout without origin = %+v", got)
	}
}

// A refusal of repository_worktree says what stands in the way (#589): the
// advice to attach the folder comes only when every attached folder was read,
// with the workstation that answered, and each folder is named with its own
// reason.
func TestRepositoryWorktreeRefusalNamesTheReason(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	other := checkoutOf(t, "git@github.com:o/other.git")
	noOrigin := t.TempDir()
	gitTest(t, noOrigin, "init", "-q")
	plain := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	broken := brokenCheckout(t)
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}
	const advice = "attachez son dossier"

	for _, c := range []struct {
		name    string
		folders []string
		want    []string
		advice  bool
	}{
		{"nothing attached", nil, []string{"github.com/o/lib", "ni associé ni attaché", "(laptop)"}, true},
		{"another origin", []string{other}, []string{"(laptop)", other + " : a pour origin git@github.com:o/other.git"}, true},
		{"plain folder", []string{plain}, []string{plain + " : n'est pas un checkout Git"}, true},
		{"no origin", []string{noOrigin}, []string{noOrigin + " : checkout Git sans origin", "sur place"}, true},
		{"missing", []string{missing}, []string{"(laptop)", missing + " : introuvable"}, false},
		{"git failing", []string{broken}, []string{broken + " : git a échoué", "bad config"}, false},
		{"several reasons", []string{missing, broken, other}, []string{missing + " : introuvable", broken + " : git a échoué", other + " : a pour origin"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := repositoryWorktree(ctx, multiRepoConfig(), attachedTo(agentconfig.Settings{}, c.folders...), projectRoot, task, "github.com/o/lib", "laptop")
			if err == nil {
				t.Fatal("refusal expected")
			}
			for _, want := range c.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal misses %q:\n%s", want, err)
				}
			}
			if got := strings.Contains(err.Error(), advice); got != c.advice {
				t.Errorf("advice to attach = %v, want %v:\n%s", got, c.advice, err)
			}
		})
	}

	// A folder that answers for the repository wins over one that fails.
	lib := checkoutOf(t, "git@github.com:o/lib.git")
	gitTest(t, lib, "branch", "feat/1")
	got, err := repositoryWorktree(ctx, multiRepoConfig(), attachedTo(agentconfig.Settings{}, missing, broken, lib), projectRoot, task, "github.com/o/lib", "laptop")
	if err != nil || got.Repository != "github.com/o/lib" || got.Branch != "feat/1" {
		t.Errorf("a failing folder beside the repository's: %+v, %v", got, err)
	}
}

// A conversation or a free console has no ticket: its folders are the
// project's other repositories, its specifications folder and its present
// attached folders, never its own directory nor a folder gone since (#676).
func TestProjectFolderMapListsTheProjectFolders(t *testing.T) {
	testhome.Temp(t)
	ctx := context.Background()
	root := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	spec := t.TempDir()
	notes := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	if err := agentconfig.WriteSettings(agentconfig.Settings{
		ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root, MacroSpecPath: spec, Folders: []string{notes, missing}}},
		Repositories:    map[string]string{"github.com/o/b": b},
	}); err != nil {
		t.Fatal(err)
	}
	d, _, _ := desktopAgent(t, root, models.Task{})

	entries, err := d.projectFolderMap(ctx, multiRepoConfig(), root)
	if err != nil {
		t.Fatal(err)
	}
	dirs := folderMapDirs(entries)
	if len(dirs) != 3 || !containsPath(t, dirs, b) || !containsPath(t, dirs, spec) || !containsPath(t, dirs, notes) {
		t.Fatalf("add-dirs = %v", dirs)
	}
	if containsPath(t, dirs, root) || strings.Contains(strings.Join(dirs, " "), missing) {
		t.Fatalf("the directory itself or a missing folder reached the CLI: %v", dirs)
	}

	other := agentconfig.Config{ProjectID: "elsewhere", GitRemoteURL: "git@github.com:o/z.git"}
	if _, err := d.projectFolderMap(ctx, other, root); err == nil {
		t.Fatal("a project without a folder here must report it")
	}
}
