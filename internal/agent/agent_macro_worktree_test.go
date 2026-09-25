package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// specRepoWithRemote is a clone of a bare remote whose default branch is
// main, plus the remote itself, so a test can land commits upstream.
func specRepoWithRemote(t *testing.T) (clone, remote string) {
	t.Helper()
	remote = filepath.Join(t.TempDir(), "specs.git")
	gitTest(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", remote)
	seed := t.TempDir()
	gitTest(t, seed, "init", "-q", "-b", "main")
	gitTest(t, seed, "commit", "-q", "--allow-empty", "-m", "init")
	gitTest(t, seed, "remote", "add", "origin", remote)
	gitTest(t, seed, "push", "-q", "origin", "main")
	clone = filepath.Join(t.TempDir(), "specs")
	gitTest(t, t.TempDir(), "clone", "-q", remote, clone)
	return clone, remote
}

// pushUpstream lands a commit on the remote's main that the clone has not seen.
func pushUpstream(t *testing.T, remote, message string) string {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	gitTest(t, t.TempDir(), "clone", "-q", remote, other)
	gitTest(t, other, "commit", "-q", "--allow-empty", "-m", message)
	gitTest(t, other, "push", "-q", "origin", "main")
	return gitTest(t, other, "rev-parse", "HEAD")
}

func samePath(t *testing.T, a, b string) bool {
	t.Helper()
	ra, _ := filepath.EvalSymlinks(a)
	rb, _ := filepath.EvalSymlinks(b)
	return ra == rb
}

func TestMacroWorktreeStartsFromTheFetchedDefaultBranch(t *testing.T) {
	ctx := context.Background()
	clone, remote := specRepoWithRemote(t)
	upstream := pushUpstream(t, remote, "landed after the clone")

	ws, err := ensureMacroWorktree(ctx, clone, "m-7", "Ux improvements and fixes", true)
	if err != nil {
		t.Fatalf("ensureMacroWorktree: %v", err)
	}
	if ws.Branch != "M-7-ux-improvements-and-fixes" || !ws.Worktree || ws.Warning != "" {
		t.Fatalf("unexpected workspace %+v", ws)
	}
	if !samePath(t, ws.Path, filepath.Join(clone, ".tasks", "worktrees", "M-7")) {
		t.Fatalf("worktree at %s, want .tasks/worktrees/M-7", ws.Path)
	}
	if head := gitTest(t, ws.Path, "rev-parse", "HEAD"); head != upstream {
		t.Fatalf("the branch must start from the fetched origin/main %s, got %s", upstream, head)
	}
	if status := gitTest(t, clone, "status", "--porcelain"); status != "" {
		t.Fatalf("the worktree must not show in the repository status, got %q", status)
	}
	if gitignore, _ := os.ReadFile(filepath.Join(clone, ".gitignore")); len(gitignore) != 0 {
		t.Fatalf(".gitignore must not be written, got %q", gitignore)
	}
}

func TestMacroWorktreeReusesTheExistingMacroBranch(t *testing.T) {
	ctx := context.Background()
	clone, _ := specRepoWithRemote(t)
	gitTest(t, clone, "branch", "M-7-foo")

	ws, err := ensureMacroWorktree(ctx, clone, "M-7", "Another title", true)
	if err != nil {
		t.Fatalf("ensureMacroWorktree: %v", err)
	}
	if ws.Branch != "M-7-foo" {
		t.Fatalf("the existing M-7 branch must be reused, got %q", ws.Branch)
	}
}

func TestMacroWorktreeTracksARemoteOnlyMacroBranch(t *testing.T) {
	ctx := context.Background()
	clone, remote := specRepoWithRemote(t)
	other := filepath.Join(t.TempDir(), "other")
	gitTest(t, t.TempDir(), "clone", "-q", remote, other)
	gitTest(t, other, "checkout", "-q", "-b", "M-7-remote")
	gitTest(t, other, "commit", "-q", "--allow-empty", "-m", "spec")
	gitTest(t, other, "push", "-q", "origin", "M-7-remote")
	pushed := gitTest(t, other, "rev-parse", "HEAD")

	ws, err := ensureMacroWorktree(ctx, clone, "M-7", "", true)
	if err != nil {
		t.Fatalf("ensureMacroWorktree: %v", err)
	}
	if ws.Branch != "M-7-remote" || gitTest(t, ws.Path, "rev-parse", "HEAD") != pushed {
		t.Fatalf("the remote macro branch must be checked out, got %+v", ws)
	}
}

func TestMacroWorktreeIsReusedWithItsUncommittedWork(t *testing.T) {
	ctx := context.Background()
	clone, _ := specRepoWithRemote(t)
	first, err := ensureMacroWorktree(ctx, clone, "M-7", "Ux", true)
	if err != nil {
		t.Fatalf("first preparation: %v", err)
	}
	draft := filepath.Join(first.Path, "draft.md")
	if err := os.WriteFile(draft, []byte("work in progress"), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := ensureMacroWorktree(ctx, clone, "M-7", "Ux", true)
	if err != nil {
		t.Fatalf("second preparation: %v", err)
	}
	if !samePath(t, first.Path, second.Path) || first.Branch != second.Branch {
		t.Fatalf("the same tree must be returned, got %+v then %+v", first, second)
	}
	if raw, err := os.ReadFile(draft); err != nil || string(raw) != "work in progress" {
		t.Fatalf("uncommitted work must survive a second preparation: %v %q", err, raw)
	}
}

func TestMacroWorktreeReturnsTheMainCheckoutOnTheMacroBranch(t *testing.T) {
	ctx := context.Background()
	clone, _ := specRepoWithRemote(t)
	gitTest(t, clone, "checkout", "-q", "-b", "M-7-here")

	ws, err := ensureMacroWorktree(ctx, clone, "M-7", "", true)
	if err != nil {
		t.Fatalf("ensureMacroWorktree: %v", err)
	}
	if ws.Path != clone || ws.Worktree || ws.Branch != "M-7-here" {
		t.Fatalf("the main checkout must be returned, got %+v", ws)
	}
}

func TestMacroWorktreesAreSeparatePerMacro(t *testing.T) {
	ctx := context.Background()
	clone, _ := specRepoWithRemote(t)
	seven, err := ensureMacroWorktree(ctx, clone, "M-7", "Seven", true)
	if err != nil {
		t.Fatal(err)
	}
	eight, err := ensureMacroWorktree(ctx, clone, "M-8", "Eight", true)
	if err != nil {
		t.Fatal(err)
	}
	if samePath(t, seven.Path, eight.Path) || seven.Branch == eight.Branch {
		t.Fatalf("two macros must get two trees on two branches: %+v %+v", seven, eight)
	}
	if err := os.WriteFile(filepath.Join(seven.Path, "spec.md"), []byte("M-7 only"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(eight.Path, "spec.md")); !os.IsNotExist(err) {
		t.Fatalf("an untracked file of M-7 must not appear in M-8: %v", err)
	}
}

func TestMacroWorktreeRecreatesAnEmptyStalePath(t *testing.T) {
	ctx := context.Background()
	clone, _ := specRepoWithRemote(t)
	stale := filepath.Join(clone, ".tasks", "worktrees", "M-7")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}

	ws, err := ensureMacroWorktree(ctx, clone, "M-7", "Ux", true)
	if err != nil {
		t.Fatalf("an empty leftover directory must be replaced: %v", err)
	}
	if !samePath(t, ws.Path, stale) || !ws.Worktree {
		t.Fatalf("unexpected workspace %+v", ws)
	}
}

func TestMacroWorktreeNeverDeletesANonEmptyStalePath(t *testing.T) {
	ctx := context.Background()
	clone, _ := specRepoWithRemote(t)
	stale := filepath.Join(clone, ".tasks", "worktrees", "M-7")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(stale, "notes.md")
	if err := os.WriteFile(kept, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ensureMacroWorktree(ctx, clone, "M-7", "Ux", true)
	if err == nil || !strings.Contains(err.Error(), stale) {
		t.Fatalf("the refusal must name the path, got %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("the directory's content must be left alone: %v", err)
	}
}

func TestMacroWorktreeWithoutARemoteWarnsAndProceeds(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	gitTest(t, repo, "init", "-q", "-b", "main")
	gitTest(t, repo, "commit", "-q", "--allow-empty", "-m", "init")

	ws, err := ensureMacroWorktree(ctx, repo, "M-7", "Ux", true)
	if err != nil {
		t.Fatalf("a repository without a remote must still get its worktree: %v", err)
	}
	if !ws.Worktree || ws.Warning == "" {
		t.Fatalf("expected a worktree and a warning, got %+v", ws)
	}
}

func TestMacroWorktreeFetchFailureIsAWarning(t *testing.T) {
	ctx := context.Background()
	clone, remote := specRepoWithRemote(t)
	if err := os.RemoveAll(remote); err != nil {
		t.Fatal(err)
	}

	ws, err := ensureMacroWorktree(ctx, clone, "M-7", "Ux", true)
	if err != nil {
		t.Fatalf("a failed fetch must not stop the preparation: %v", err)
	}
	if !strings.Contains(ws.Warning, "fetch") {
		t.Fatalf("the warning must say the fetch failed, got %q", ws.Warning)
	}
}

func TestMacroWorktreeOffUsesTheCheckout(t *testing.T) {
	ctx := context.Background()
	clone, _ := specRepoWithRemote(t)

	ws, err := ensureMacroWorktree(ctx, clone, "M-7", "Ux", false)
	if err != nil {
		t.Fatalf("ensureMacroWorktree: %v", err)
	}
	if ws.Path != clone || ws.Worktree || ws.Branch != "M-7-ux" || ws.Warning == "" {
		t.Fatalf("unexpected workspace %+v", ws)
	}
	if _, err := os.Stat(filepath.Join(clone, ".tasks")); !os.IsNotExist(err) {
		t.Fatalf("nothing must be created with worktrees off: %v", err)
	}
}

// A plain folder is used in place: no branch, no worktree, nothing created,
// and a warning that nothing will be committed.
func TestMacroWorktreeUsesAPlainFolderInPlace(t *testing.T) {
	dir := t.TempDir()
	for _, useWorktrees := range []bool{true, false} {
		ws, err := ensureMacroWorktree(context.Background(), dir, "M-7", "Ux", useWorktrees)
		if err != nil {
			t.Fatalf("a plain folder must be accepted: %v", err)
		}
		if ws.Path != dir || ws.Branch != "" || ws.Worktree || !strings.Contains(ws.Warning, "pas un dépôt Git") {
			t.Fatalf("unexpected workspace %+v", ws)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("nothing must be created in a plain folder, found %v", entries)
	}
}

func TestMacroWorktreeRefusesAMissingPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gone")
	_, err := ensureMacroWorktree(context.Background(), dir, "M-7", "Ux", true)
	if err == nil || !strings.Contains(err.Error(), dir) {
		t.Fatalf("the refusal must name the path, got %v", err)
	}
}

func TestConcurrentMacroPreparationsShareOneTree(t *testing.T) {
	ctx := context.Background()
	clone, _ := specRepoWithRemote(t)
	var wg sync.WaitGroup
	results := make([]macroWorkspace, 4)
	errs := make([]error, 4)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = ensureMacroWorktree(ctx, clone, "M-7", "Ux", true)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("preparation %d: %v", i, errs[i])
		}
		if !samePath(t, results[i].Path, results[0].Path) || results[i].Branch != results[0].Branch {
			t.Fatalf("every preparation must return the same tree: %+v vs %+v", results[i], results[0])
		}
	}
}
