package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// A multi-repo project: o/a is the code remote, o/b and o/c are declared.
func multiRepoConfig() agentconfig.Config {
	no := false
	return agentconfig.Config{ProjectID: "p", ProjectName: "Multi", GitRemoteURL: "git@github.com:o/a.git", MonoRepo: &no,
		Repositories: []string{"git@github.com:o/a.git", "git@github.com:o/b.git", "git@github.com:o/c.git"}}
}

func branchOf(name string) *string { return &name }

func TestPrimaryRootFollowsThePin(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	overrides := agentconfig.Overrides{Repositories: map[string]string{"github.com/o/b": b}}

	root, identity, pin, err := primaryRoot(ctx, multiRepoConfig(), overrides, projectRoot, models.Task{Key: "#1", Repository: "github.com/o/b"})
	if err != nil || root != b || identity != "github.com/o/b" || pin != "" {
		t.Fatalf("pinned: %q %q %q %v", root, identity, pin, err)
	}
	// Another repository keeps .tasks/ out of its status by itself.
	exclude, _ := os.ReadFile(filepath.Join(b, ".git", "info", "exclude"))
	if !strings.Contains(string(exclude), "/.tasks/") {
		t.Errorf("info/exclude = %q", exclude)
	}

	if _, _, _, err := primaryRoot(ctx, multiRepoConfig(), overrides, projectRoot, models.Task{Key: "#1", Repository: "github.com/o/c"}); err == nil || !strings.Contains(err.Error(), "github.com/o/c") {
		t.Errorf("unmapped pin: %v", err)
	}
	if _, _, _, err := primaryRoot(ctx, multiRepoConfig(), overrides, projectRoot, models.Task{Key: "#1"}); !errors.Is(err, errRepositoryAmbiguous) {
		t.Errorf("unpinned with a and b mapped: %v, want ambiguous", err)
	}
	// Only the code remote is mapped here (through the project root): it is
	// the task's, and the task gets pinned to it.
	root, _, pin, err = primaryRoot(ctx, multiRepoConfig(), agentconfig.Overrides{}, projectRoot, models.Task{Key: "#1"})
	if err != nil || root != projectRoot || pin != "github.com/o/a" {
		t.Errorf("single candidate: %q %q %v", root, pin, err)
	}
	yes := true
	mono := multiRepoConfig()
	mono.MonoRepo = &yes
	if root, _, pin, err := primaryRoot(ctx, mono, overrides, projectRoot, models.Task{Key: "#1"}); err != nil || root != projectRoot || pin != "" {
		t.Errorf("mono-repo keeps the project root: %q %q %v", root, pin, err)
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
	overrides := agentconfig.Overrides{Repositories: map[string]string{"github.com/o/b": b}, SpecRepos: map[string]string{"p": spec}}
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
	single := buildFolderMap(ctx, agentconfig.Config{ProjectID: "p", GitRemoteURL: "git@github.com:o/a.git"}, agentconfig.Overrides{}, projectRoot, "github.com/o/a", projectRoot, models.Task{Key: "#1"})
	if len(single) != 1 || folderMapPrompt(single) != "" {
		t.Errorf("a single checkout needs no map in the prompt: %+v", single)
	}
}

func TestContextFoldersReachOnlyClaude(t *testing.T) {
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
	for _, provider := range []string{"codex", "vibe", "gemini"} {
		for _, mode := range []string{models.SkillModeInteractive, models.SkillModeAutonomous} {
			if line, _ := modeCommandLine(provider, "", "", "go", mode, launch); strings.Contains(line, "add-dir") || strings.Contains(line, "/src/b") {
				t.Errorf("%s %s guessed a flag: %q", provider, mode, line)
			}
		}
	}
	if line, _ := modeCommandLine("claude", "claude {addDirs} '{prompt}'", "", "go", models.SkillModeInteractive, launch); !strings.HasPrefix(line, "claude --add-dir='/src/b'") {
		t.Errorf("template {addDirs} = %q", line)
	}
	if line, _ := modeCommandLine("codex", "codex {addDirs} '{prompt}'", "", "go", models.SkillModeInteractive, launch); strings.Contains(line, "add-dir") {
		t.Errorf("template for codex = %q", line)
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
	overrides := agentconfig.Overrides{Repositories: map[string]string{"github.com/o/b": b}}
	task := models.Task{Key: "#1", BranchName: branchOf("feat/1")}

	first, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "https://github.com/o/b")
	if err != nil {
		t.Fatal(err)
	}
	if first.Repository != "github.com/o/b" || first.Branch != "feat/1" || !strings.Contains(first.Path, filepath.Join(".tasks", "worktrees", "#1")) {
		t.Fatalf("worktree = %+v", first)
	}
	if got := gitTest(t, first.Path, "branch", "--show-current"); got != "feat/1" {
		t.Errorf("worktree branch = %q", got)
	}
	again, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "github.com/o/b")
	if err != nil || !samePath(t, again.Path, first.Path) {
		t.Errorf("second request = %+v, %v", again, err)
	}
	if _, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "github.com/o/c"); err == nil || !strings.Contains(err.Error(), "github.com/o/c") {
		t.Errorf("unmapped: %v", err)
	}
	if _, err := repositoryWorktree(ctx, multiRepoConfig(), overrides, projectRoot, task, "github.com/o/elsewhere"); err == nil {
		t.Error("a repository outside the project was accepted")
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

// repositoryWaitServer is the server side of a parked launch: it records the
// waiting marks and answers the ticket, pinned once pin is called.
type repositoryWaitServer struct {
	mu     sync.Mutex
	marks  []bool
	pinned string
}

func (s *repositoryWaitServer) handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case strings.HasSuffix(r.URL.Path, "/awaiting-repository"):
		var body struct{ Waiting bool }
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.marks = append(s.marks, body.Waiting)
		_, _ = w.Write([]byte(`{}`))
	case strings.HasPrefix(r.URL.Path, "/api/tasks/"):
		_ = json.NewEncoder(w).Encode(models.Task{ID: "t1", Key: "#1", Repository: s.pinned})
	default:
		http.NotFound(w, r)
	}
}

func TestAParkedLaunchResumesOnceTheTicketIsPinned(t *testing.T) {
	previous := repositoryPollInterval
	repositoryPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { repositoryPollInterval = previous })
	server := &repositoryWaitServer{}
	srv := httptest.NewServer(http.HandlerFunc(server.handler))
	t.Cleanup(srv.Close)
	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}
	run := &controlledRun{limit: 1, exited: make(chan struct{}), desktop: desktopRun{ID: "run-1", ProjectID: "p", Status: "preparing"}}

	done := make(chan error, 1)
	go func() { done <- d.awaitRepository(context.Background(), multiRepoConfig(), run, "t1", "run-1") }()
	time.Sleep(50 * time.Millisecond)
	d.queue.mu.Lock()
	status := run.desktop.Status
	d.queue.mu.Unlock()
	if status != "waiting" {
		t.Errorf("status while parked = %q, want waiting", status)
	}
	// A pin to a repository the project does not declare is no pin: the
	// launch keeps waiting instead of parking again in a loop.
	server.mu.Lock()
	server.pinned = "github.com/o/gone"
	server.mu.Unlock()
	time.Sleep(60 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("a pin outside the project resumed the launch")
	default:
	}
	server.mu.Lock()
	server.pinned = "github.com/o/b"
	server.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the launch never resumed")
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.marks) != 2 || !server.marks[0] || server.marks[1] {
		t.Errorf("waiting marks = %v, want set then cleared", server.marks)
	}
	if run.desktop.Status != "preparing" {
		t.Errorf("status on resume = %q, want the slot taken again", run.desktop.Status)
	}
}

func TestAParkedLaunchEndsWhenCanceled(t *testing.T) {
	previous := repositoryPollInterval
	repositoryPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { repositoryPollInterval = previous })
	srv := httptest.NewServer(http.HandlerFunc((&repositoryWaitServer{}).handler))
	t.Cleanup(srv.Close)
	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}
	run := &controlledRun{limit: 1, exited: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- d.awaitRepository(context.Background(), multiRepoConfig(), run, "t1", "run-1") }()
	time.Sleep(30 * time.Millisecond)
	d.queue.mu.Lock()
	run.canceled = true
	d.queue.mu.Unlock()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a canceled wait resumed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the canceled wait never ended")
	}
}

func TestAWaitingRunHoldsNoSlot(t *testing.T) {
	d := &agentDaemon{}
	waiting := &controlledRun{sequence: 1, limit: 1, exited: make(chan struct{}), desktop: desktopRun{ID: "w", ProjectID: "p", Status: "waiting"}}
	next := &controlledRun{sequence: 2, limit: 1, exited: make(chan struct{}), desktop: desktopRun{ID: "n", ProjectID: "p", Status: "queued"}}
	d.queue.runs = map[string]*controlledRun{"w": waiting, "n": next}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := d.awaitRunSlot(ctx, next); err != nil {
		t.Fatalf("a run queued behind a waiting one did not start: %v", err)
	}
}

func TestConvertedFoldersAreKeptOnThisWorkstation(t *testing.T) {
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	if err := agentconfig.WriteSettings(agentconfig.Overrides{Projects: map[string]string{"p": root}}); err != nil {
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
