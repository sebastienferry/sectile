package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"testing"

	"tasks/internal/models"
)

func mustWorktreeName(t *testing.T, key string) string {
	t.Helper()
	name, err := safeWorktreeName(key)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func TestSafeWorktreeNames(t *testing.T) {
	keys := []string{"#289", "#1", "#01", "#0", "issue-289", "A?B", "A#B", "A B", "a b", "é", "é", "CON", "NUL", "...", "???", strings.Repeat("x", 500), "#" + strings.Repeat("9", 500)}
	seen := map[string]string{}
	for _, key := range keys {
		name := mustWorktreeName(t, key)
		if len(name) > worktreeNameLimit || !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(name) {
			t.Fatalf("unsafe name %q", name)
		}
		if name != mustWorktreeName(t, key) {
			t.Fatal("unstable name")
		}
		if other, ok := seen[strings.ToLower(name)]; ok {
			t.Fatalf("collision: %q and %q", key, other)
		}
		seen[strings.ToLower(name)] = key
	}
	if mustWorktreeName(t, "#289") != "issue-289" {
		t.Fatal("numeric mapping")
	}
	for _, key := range []string{"", ".", "..", "a/b", `a\b`} {
		if _, err := safeWorktreeName(key); err == nil {
			t.Fatalf("accepted %q", key)
		}
		if _, err := localTaskPath(context.Background(), t.TempDir(), models.Task{Key: key}, ""); err == nil {
			t.Fatalf("lookup accepted %q", key)
		}
	}
}

func TestTaskOccupiedDestinationsAndReuse(t *testing.T) {
	for _, kind := range []string{"file", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			gitTest(t, root, "init", "-q", "-b", "main")
			gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
			branch := "feat/289"
			task := models.Task{Key: "#289", BranchName: &branch}
			container := filepath.Join(root, ".tasks", "worktrees")
			if err := os.MkdirAll(container, 0755); err != nil {
				t.Fatal(err)
			}
			occupied := filepath.Join(container, "issue-289")
			var err error
			switch kind {
			case "file":
				err = os.WriteFile(occupied, []byte("preserved"), 0644)
			case "directory":
				err = os.Mkdir(occupied, 0755)
			case "symlink":
				err = os.Symlink(filepath.Join(root, "missing"), occupied)
			}
			if err != nil {
				t.Fatal(err)
			}
			sibling := filepath.Join(container, occupiedWorktreeName("issue-289", branch, 1))
			if err := os.WriteFile(sibling, []byte("preserved"), 0644); err != nil {
				t.Fatal(err)
			}
			path, got, err := ensureLocalWorktree(context.Background(), root, task, true, "")
			if err != nil || got != branch || filepath.Base(path) != occupiedWorktreeName("issue-289", branch, 2) {
				t.Fatalf("%s %s %v", path, got, err)
			}
			again, _, err := ensureLocalWorktree(context.Background(), root, task, true, "")
			if err != nil || !sameDirectory(path, again) {
				t.Fatalf("reuse %s %v", again, err)
			}
			lookup, err := localTaskPath(context.Background(), root, task, "")
			if err != nil || !sameDirectory(lookup, path) {
				t.Fatalf("lookup %s %v", lookup, err)
			}
			if _, err := os.Lstat(occupied); err != nil {
				t.Fatal(err)
			}
			if raw, err := os.ReadFile(sibling); err != nil || string(raw) != "preserved" {
				t.Fatal("occupied sibling changed")
			}
		})
	}
}

func TestLocalTaskResolutionPreservesBranchLocations(t *testing.T) {
	for _, location := range []string{"#289", "batch-shared", "arbitrary", "main"} {
		t.Run(location, func(t *testing.T) {
			root := t.TempDir()
			gitTest(t, root, "init", "-q", "-b", "main")
			gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
			branch := "feat/289"
			path := root
			if location == "main" {
				gitTest(t, root, "checkout", "-b", branch)
			} else {
				path = filepath.Join(root, ".tasks", "worktrees", location)
				gitTest(t, root, "worktree", "add", "-b", branch, path)
			}
			marker := filepath.Join(path, "dirty.txt")
			if err := os.WriteFile(marker, []byte("preserved"), 0644); err != nil {
				t.Fatal(err)
			}
			task := models.Task{Key: "#289", BranchName: &branch}
			for _, lookup := range []func() (string, error){func() (string, error) { return localTaskPath(context.Background(), root, task, "") }, func() (string, error) {
				p, _, e := ensureLocalWorktree(context.Background(), root, task, true, "")
				return p, e
			}} {
				got, err := lookup()
				if err != nil || !sameDirectory(got, path) {
					t.Fatalf("%s %v", got, err)
				}
			}
			if raw, err := os.ReadFile(marker); err != nil || string(raw) != "preserved" {
				t.Fatal("dirty content changed")
			}
		})
	}
}

func TestWrongBranchAndDetachedPredictionsAreNotUsed(t *testing.T) {
	for _, detached := range []bool{false, true} {
		root := t.TempDir()
		gitTest(t, root, "init", "-q", "-b", "main")
		gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
		for _, name := range []string{"#289", "issue-289"} {
			path := filepath.Join(root, ".tasks", "worktrees", name)
			if detached {
				gitTest(t, root, "worktree", "add", "--detach", path)
			} else {
				gitTest(t, root, "worktree", "add", "-b", "other-"+strings.TrimPrefix(name, "#"), path)
			}
		}
		branch := "feat/289"
		task := models.Task{Key: "#289", BranchName: &branch}
		path, err := localTaskPath(context.Background(), root, task, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("unrelated checkout returned %s", path)
		}
	}
}

func TestOccupiedCandidatesExhaustSafely(t *testing.T) {
	root := t.TempDir()
	container := filepath.Join(root, ".tasks", "worktrees")
	if err := os.MkdirAll(container, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		name := "issue-289"
		if i > 0 {
			name = occupiedWorktreeName(name, "feat/289", i)
		}
		if err := os.WriteFile(filepath.Join(container, name), []byte("preserved"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := availableTaskWorktreePath(root, "issue-289", "feat/289"); err == nil {
		t.Fatal("exhaustion accepted")
	}
	entries, err := os.ReadDir(container)
	if err != nil || len(entries) != 100 {
		t.Fatal("occupied candidates changed")
	}
}

func TestWorktreeCreationFailurePreservesBlockingEntry(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q", "-b", "main")
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
	blocker := filepath.Join(root, ".tasks")
	if err := os.WriteFile(blocker, []byte("preserved"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ensureLocalWorktree(context.Background(), root, models.Task{Key: "#289"}, true, ""); err == nil {
		t.Fatal("blocking file overwritten")
	}
	if raw, err := os.ReadFile(blocker); err != nil || string(raw) != "preserved" {
		t.Fatal("blocking entry changed")
	}
}

func TestMissingAssignedBranchUsesPreparationFallback(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q", "-b", "main")
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
	task := models.Task{Key: "#289"}
	path, branch, err := ensureLocalWorktree(context.Background(), root, task, true, "")
	if err != nil || branch != "feat/289" {
		t.Fatalf("%s %v", branch, err)
	}
	got, err := localTaskPath(context.Background(), root, task, "")
	if err != nil || !sameDirectory(got, path) {
		t.Fatalf("fallback lookup %s %v", got, err)
	}
}

func TestTaskWorktreeBranchFollowsTheProjectFormat(t *testing.T) {
	assigned := "feat/621"
	cases := []struct {
		task   models.Task
		format string
		want   string
	}{
		{models.Task{Key: "#621"}, "", "feat/621"},
		{models.Task{Key: "AUC-1234"}, "", "feat/auc-1234"},
		{models.Task{Key: "AUC-1234"}, "{key}", "AUC-1234"},
		{models.Task{Key: "#621"}, "{key}", "621"},
		{models.Task{Key: "AUC-1234", Title: "Choose the format"}, "feat/{key}-{title}", "feat/AUC-1234-choose-the-format"},
		// An assigned branch is never renamed by a format.
		{models.Task{Key: "#621", BranchName: &assigned}, "{key}", "feat/621"},
	}
	for _, c := range cases {
		if got, err := taskWorktreeBranch(c.task, c.format); err != nil || got != c.want {
			t.Errorf("taskWorktreeBranch(%s, %q) = %q, %v, want %q", c.task.Key, c.format, got, err, c.want)
		}
	}
}

func TestWorktreeIsCreatedOnTheFormattedBranch(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q", "-b", "main")
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
	task := models.Task{Key: "AUC-1234", Title: "Choose the format"}
	path, branch, err := ensureLocalWorktree(context.Background(), root, task, true, "{key}")
	if err != nil || branch != "AUC-1234" {
		t.Fatalf("%s %v", branch, err)
	}
	if current := strings.TrimSpace(gitTest(t, path, "branch", "--show-current")); current != "AUC-1234" {
		t.Fatalf("the worktree is on %q, want AUC-1234", current)
	}
	got, err := localTaskPath(context.Background(), root, task, "{key}")
	if err != nil || !sameDirectory(got, path) {
		t.Fatalf("formatted lookup %s %v", got, err)
	}
}

func TestMacroNumericNamingAndSymlinkProtection(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q", "-b", "main")
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
	ws, err := ensureMacroWorktree(context.Background(), root, "#289", "Macro", true)
	if err != nil || filepath.Base(ws.Path) != "issue-289" {
		t.Fatalf("%+v %v", ws, err)
	}
	again, err := ensureMacroWorktree(context.Background(), root, "#289", "Macro", true)
	if err != nil || !sameDirectory(again.Path, ws.Path) {
		t.Fatalf("macro reuse %+v %v", again, err)
	}
	target := filepath.Join(root, ".tasks", "worktrees", "issue-290")
	if err := os.Symlink(filepath.Join(root, "missing"), target); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureMacroWorktree(context.Background(), root, "#290", "Macro", true); err == nil {
		t.Fatal("macro symlink accepted")
	}
	if fi, err := os.Lstat(target); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("macro symlink changed")
	}
}

func TestWorkspaceOperationsResolveAndProtectDirtyLegacyCheckout(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gitTest(t, root, "init", "-q", "-b", "main")
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
	branch := "feat/289"
	path := filepath.Join(root, ".tasks", "worktrees", "#289")
	gitTest(t, root, "worktree", "add", "-b", branch, path)
	task := models.Task{ID: "task", ProjectID: "project", Key: "#289", BranchName: &branch}
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: task.ProjectID, UseWorktrees: true, AIProvider: "claude"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
		} else {
			_ = json.NewEncoder(w).Encode(task)
		}
	}))
	defer server.Close()
	daemon := &agentDaemon{repoRoot: root, link: serverLink{serverURL: server.URL, token: "token", projectID: task.ProjectID}}
	op := agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "workspace_info"}
	value, err := daemon.executeOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	info := value.(models.WorktreeInfo)
	if !info.Exists || !sameDirectory(info.WorktreePath, path) || info.Branch != branch || info.TaskKey != "#289" {
		t.Fatalf("%+v", info)
	}
	marker := filepath.Join(path, "dirty.txt")
	if err := os.WriteFile(marker, []byte("preserved"), 0644); err != nil {
		t.Fatal(err)
	}
	op.Action = "remove_workspace"
	if _, err := daemon.executeOperation(ctx, op); err == nil {
		t.Fatal("dirty checkout removed")
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "preserved" {
		t.Fatal("dirty legacy content changed")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if _, err := daemon.executeOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("clean legacy checkout not removed")
	}
	gitTest(t, root, "checkout", branch)
	if _, err := daemon.executeOperation(ctx, op); err == nil {
		t.Fatal("main checkout removal allowed")
	}
}
