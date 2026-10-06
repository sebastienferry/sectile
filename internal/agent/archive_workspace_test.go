package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// archiveCheckout is a checkout whose origin is a bare repository of this
// machine, so a branch can be pushed and get an upstream.
func archiveCheckout(t *testing.T) string {
	t.Helper()
	origin := t.TempDir()
	gitTest(t, origin, "init", "-q", "--bare", "-b", "main")
	root := t.TempDir()
	gitTest(t, root, "init", "-q", "-b", "main")
	gitTest(t, root, "remote", "add", "origin", origin)
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
	gitTest(t, root, "push", "-q", "-u", "origin", "main")
	return root
}

// taskWorktree adds the worktree of branch under root's .tasks/worktrees.
func taskWorktree(t *testing.T, root, branch string, push bool) string {
	t.Helper()
	path := filepath.Join(root, ".tasks", "worktrees", strings.ReplaceAll(branch, "/", "-"))
	gitTest(t, root, "worktree", "add", "-q", "-b", branch, path)
	gitTest(t, path, "commit", "-q", "--allow-empty", "-m", "work")
	if push {
		gitTest(t, path, "push", "-q", "-u", "origin", branch)
	}
	return path
}

func branchExists(t *testing.T, root, branch string) bool {
	t.Helper()
	return gitTest(t, root, "branch", "--list", branch) != ""
}

func archiveConfig() agentconfig.Config {
	return agentconfig.Config{ProjectID: "p", UseWorktrees: true}
}

func TestArchiveWorkspaceKeepsADirtyWorktreeThenRemovesItClean(t *testing.T) {
	ctx := context.Background()
	root := archiveCheckout(t)
	path := taskWorktree(t, root, "feat/1", true)
	task := models.Task{ID: "t", ProjectID: "p", Key: "#1", BranchName: branchOf("feat/1")}
	op := agentprotocol.Operation{Action: "archive_workspace", DeleteBranch: true}
	if err := os.WriteFile(filepath.Join(path, "notes.txt"), []byte("unsaved"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty := archiveWorkspace(ctx, archiveConfig(), agentconfig.Settings{}, root, task, op)
	if dirty.Archivable() || len(dirty.Repositories) != 1 || dirty.Repositories[0].Outcome != models.ArchiveFailed || dirty.Repositories[0].Error == "" {
		t.Fatalf("dirty = %+v", dirty)
	}
	if raw, err := os.ReadFile(filepath.Join(path, "notes.txt")); err != nil || string(raw) != "unsaved" {
		t.Fatalf("the untracked file was lost: %v", err)
	}
	if dirty.Repositories[0].BranchOutcome != "" || !branchExists(t, root, "feat/1") {
		t.Errorf("a refused worktree touched its branch: %+v", dirty.Repositories[0])
	}

	if err := os.Remove(filepath.Join(path, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	clean := archiveWorkspace(ctx, archiveConfig(), agentconfig.Settings{}, root, task, op)
	entry := clean.Repositories[0]
	if !clean.Archivable() || entry.Outcome != models.ArchiveRemoved || entry.Role != "code" || !samePath(t, entry.Path, path) {
		t.Fatalf("clean = %+v", clean)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the worktree is still there: %v", err)
	}
	if entry.BranchOutcome != models.ArchiveBranchDeleted || branchExists(t, root, "feat/1") {
		t.Errorf("merged and pushed branch kept: %+v", entry)
	}

	again := archiveWorkspace(ctx, archiveConfig(), agentconfig.Settings{}, root, task, op)
	if !again.Archivable() || again.Repositories[0].Outcome != models.ArchiveAbsent || again.Repositories[0].BranchReason != "missing" {
		t.Errorf("second archive = %+v", again)
	}
}

func TestArchiveWorkspaceKeepsTheBranchUnlessMergedAndPushed(t *testing.T) {
	ctx := context.Background()
	root := archiveCheckout(t)
	taskWorktree(t, root, "feat/open", true)
	taskWorktree(t, root, "feat/local", false)
	ahead := taskWorktree(t, root, "feat/ahead", true)
	gitTest(t, ahead, "commit", "-q", "--allow-empty", "-m", "not pushed")

	cases := []struct {
		branch string
		delete bool
		reason string
	}{
		{"feat/open", false, "no-merged-pr"},
		{"feat/local", true, "no-upstream"},
		{"feat/ahead", true, "unpushed"},
	}
	for _, c := range cases {
		task := models.Task{ID: "t", ProjectID: "p", Key: "#2", BranchName: branchOf(c.branch)}
		got := archiveWorkspace(ctx, archiveConfig(), agentconfig.Settings{}, root, task, agentprotocol.Operation{DeleteBranch: c.delete})
		entry := got.Repositories[0]
		if !got.Archivable() || entry.Outcome != models.ArchiveRemoved || entry.BranchOutcome != models.ArchiveBranchKept || entry.BranchReason != c.reason {
			t.Errorf("%s: %+v", c.branch, entry)
		}
		if !branchExists(t, root, c.branch) {
			t.Errorf("%s: branch deleted", c.branch)
		}
	}
}

func TestArchiveWorkspaceWithoutAWorktree(t *testing.T) {
	ctx := context.Background()
	root := archiveCheckout(t)
	task := models.Task{ID: "t", ProjectID: "p", Key: "#3", Title: "Never launched"}

	absent := archiveWorkspace(ctx, archiveConfig(), agentconfig.Settings{}, root, task, agentprotocol.Operation{})
	if !absent.Archivable() || absent.Repositories[0].Outcome != models.ArchiveAbsent {
		t.Errorf("never launched = %+v", absent)
	}

	gitTest(t, root, "checkout", "-q", "-b", "feat/3")
	task.BranchName = branchOf("feat/3")
	inCheckout := archiveWorkspace(ctx, archiveConfig(), agentconfig.Settings{}, root, task, agentprotocol.Operation{DeleteBranch: true})
	if entry := inCheckout.Repositories[0]; !inCheckout.Archivable() || entry.Outcome != models.ArchiveAbsent || entry.BranchReason != "checked-out" {
		t.Errorf("branch of the main checkout = %+v", inCheckout)
	}

	deleted := taskWorktree(t, root, "feat/deleted", false)
	if err := os.RemoveAll(deleted); err != nil {
		t.Fatal(err)
	}
	task.BranchName = branchOf("feat/deleted")
	pruned := archiveWorkspace(ctx, archiveConfig(), agentconfig.Settings{}, root, task, agentprotocol.Operation{})
	if !pruned.Archivable() || pruned.Repositories[0].Outcome != models.ArchiveAbsent || strings.Contains(gitTest(t, root, "worktree", "list"), "feat/deleted") {
		t.Errorf("worktree deleted by hand = %+v", pruned)
	}
	task.BranchName = branchOf("feat/3")

	off := archiveConfig()
	off.UseWorktrees = false
	disabled := archiveWorkspace(ctx, off, agentconfig.Settings{}, root, task, agentprotocol.Operation{DeleteBranch: true})
	if !disabled.Archivable() || disabled.Repositories[0].Outcome != models.ArchiveDisabled || gitTest(t, root, "branch", "--show-current") != "feat/3" {
		t.Errorf("worktrees off = %+v", disabled)
	}
}

func TestArchiveWorkspaceCoversEveryRepositoryAndTheSpecifications(t *testing.T) {
	ctx := context.Background()
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	spec := checkoutOf(t, "git@github.com:o/specs.git")
	codeA := taskWorktree(t, projectRoot, "feat/4", false)
	codeB := taskWorktree(t, b, "feat/4", false)
	specPath := taskWorktree(t, spec, "feat/4", false)
	overrides := agentconfig.Settings{Repositories: map[string]string{"github.com/o/b": b},
		ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {IssueSpecPath: spec}}}
	config := multiRepoConfig()
	config.UseWorktrees = true
	task := models.Task{ID: "t", ProjectID: "p", Key: "#4", BranchName: branchOf("feat/4")}

	got := archiveWorkspace(ctx, config, overrides, projectRoot, task, agentprotocol.Operation{Repositories: []string{"github.com/o/a", "github.com/o/b", "github.com/o/c"}})
	if got.Archivable() || len(got.Repositories) != 4 {
		t.Fatalf("archive = %+v", got)
	}
	outcomes := map[string]string{}
	for _, entry := range got.Repositories {
		outcomes[entry.Repository+"/"+entry.Role] = entry.Outcome
	}
	want := map[string]string{"github.com/o/a/code": "removed", "github.com/o/b/code": "removed", "github.com/o/c/code": "failed", "/specifications": "removed"}
	for key, outcome := range want {
		if outcomes[key] != outcome {
			t.Errorf("%s = %q, want %q (all: %v)", key, outcomes[key], outcome, outcomes)
		}
	}
	for _, path := range []string{codeA, codeB, specPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s is still there: %v", path, err)
		}
	}
}
