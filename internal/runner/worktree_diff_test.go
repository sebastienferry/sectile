package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func diffFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	diffGitTest(t, dir, "init", "-b", "main")
	diffGitTest(t, dir, "config", "user.email", "test@example.test")
	diffGitTest(t, dir, "config", "user.name", "Test")
	writeDiffTest(t, dir, "file.txt", "base\n")
	diffGitTest(t, dir, "add", ".")
	diffGitTest(t, dir, "commit", "-m", "base")
	diffGitTest(t, dir, "checkout", "-b", "feat/test")
	return dir
}
func diffGitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s: %v", args, b, e)
	}
	return strings.TrimSpace(string(b))
}
func writeDiffTest(t *testing.T, dir, p, text string) {
	t.Helper()
	if e := os.WriteFile(filepath.Join(dir, p), []byte(text), 0644); e != nil {
		t.Fatal(e)
	}
}
func inspectDiffTest(t *testing.T, dir string) *WorktreeDiff {
	t.Helper()
	r, e := InspectWorktree(context.Background(), dir, "feat/test", dir)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestWorktreeDiffNetAndReadOnly(t *testing.T) {
	dir := diffFixture(t)
	writeDiffTest(t, dir, "file.txt", "committed\n")
	diffGitTest(t, dir, "commit", "-am", "change")
	writeDiffTest(t, dir, "file.txt", "staged\n")
	diffGitTest(t, dir, "add", ".")
	writeDiffTest(t, dir, "file.txt", "current\n")
	p := "space\ttab\n雪.txt"
	writeDiffTest(t, dir, p, "new\n")
	writeDiffTest(t, dir, ".gitignore", "ignored\n")
	writeDiffTest(t, dir, "ignored", "secret\n")
	index, _ := os.ReadFile(filepath.Join(dir, ".git/index"))
	before := diffGitTest(t, dir, "status", "--porcelain=v1", "-uall")
	refs := diffGitTest(t, dir, "show-ref")
	r := inspectDiffTest(t, dir)
	if r.FilesChanged != 3 || r.Additions != 3 || r.Deletions != 1 {
		t.Fatalf("unexpected result: %+v", r)
	}
	for _, f := range r.Files {
		if f.Path == "file.txt" && (!strings.Contains(f.Patch, "+current") || strings.Contains(f.Patch, "staged")) {
			t.Fatalf("not a net patch: %s", f.Patch)
		}
	}
	after, _ := os.ReadFile(filepath.Join(dir, ".git/index"))
	if string(index) != string(after) || before != diffGitTest(t, dir, "status", "--porcelain=v1", "-uall") || refs != diffGitTest(t, dir, "show-ref") {
		t.Fatal("inspection changed Git state")
	}
	writeDiffTest(t, dir, "file.txt", "base\n")
	r = inspectDiffTest(t, dir)
	for _, f := range r.Files {
		if f.Path == "file.txt" {
			t.Fatal("reverted content appears")
		}
	}
}
func TestWorktreeDiffRecreatedAndRenamed(t *testing.T) {
	dir := diffFixture(t)
	diffGitTest(t, dir, "rm", "file.txt")
	writeDiffTest(t, dir, "file.txt", "base\n")
	r := inspectDiffTest(t, dir)
	if !r.IsClean {
		t.Fatalf("recreated original should be clean: %+v", r)
	}
	writeDiffTest(t, dir, "file.txt", "recreated\n")
	r = inspectDiffTest(t, dir)
	if len(r.Files) != 1 || r.Files[0].Status != "modified" {
		t.Fatalf("recreated: %+v", r.Files)
	}
	diffGitTest(t, dir, "reset", "--hard", "HEAD")
	diffGitTest(t, dir, "mv", "file.txt", "renamed.txt")
	r = inspectDiffTest(t, dir)
	if len(r.Files) != 1 || r.Files[0].OldPath != "file.txt" || r.Files[0].Status != "renamed" {
		t.Fatalf("rename: %+v", r.Files)
	}
}
func TestWorktreeDiffKindsAndBounds(t *testing.T) {
	dir := diffFixture(t)
	writeDiffTest(t, dir, "binary", string([]byte{0, 1, 2}))
	if e := os.Symlink("file.txt", filepath.Join(dir, "link")); e != nil {
		t.Fatal(e)
	}
	if e := os.Chmod(filepath.Join(dir, "file.txt"), 0755); e != nil {
		t.Fatal(e)
	}
	writeDiffTest(t, dir, "large", strings.Repeat("long line\n", 40000))
	r := inspectDiffTest(t, dir)
	kinds := map[string]WorktreeDiffFile{}
	for _, f := range r.Files {
		kinds[f.Path] = f
	}
	if kinds["binary"].Kind != "binary" || kinds["link"].Kind != "symlink" || kinds["large"].OmittedReason == "" || r.Complete || r.IsClean {
		t.Fatalf("kinds: %+v", r)
	}
	if kinds["file.txt"].Additions == nil || *kinds["file.txt"].Additions != 0 {
		t.Fatal("mode change count")
	}
}
func TestWorktreeDiffBaselineAndErrors(t *testing.T) {
	dir := diffFixture(t)
	diffGitTest(t, dir, "checkout", "main")
	writeDiffTest(t, dir, "only-main", "main\n")
	diffGitTest(t, dir, "add", ".")
	diffGitTest(t, dir, "commit", "-m", "advance")
	diffGitTest(t, dir, "checkout", "feat/test")
	r := inspectDiffTest(t, dir)
	if !r.IsClean {
		t.Fatal("default-only changes included")
	}
	diffGitTest(t, dir, "update-ref", "refs/remotes/origin/trunk", "main")
	diffGitTest(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	if inspectDiffTest(t, dir).BaseRef != "refs/remotes/origin/trunk" {
		t.Fatal("nonstandard default ignored")
	}
	diffGitTest(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/missing")
	_, e := InspectWorktree(context.Background(), dir, "feat/test", dir)
	if e == nil {
		t.Fatal("dangling default accepted")
	}
	diffGitTest(t, dir, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
	_, e = InspectWorktree(context.Background(), dir, "wrong", dir)
	if e == nil {
		t.Fatal("wrong branch accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = InspectWorktree(ctx, dir, "feat/test", dir); e == nil {
		t.Fatal("canceled inspection accepted")
	}
}

func TestWorktreeDiffHelpersDisabledAndTrackedIgnored(t *testing.T) {
	dir := diffFixture(t)
	marker := filepath.Join(t.TempDir(), "called")
	script := filepath.Join(t.TempDir(), "helper")
	writeDiffTest(t, filepath.Dir(script), filepath.Base(script), "#!/bin/sh\ntouch '"+marker+"'\nexit 1\n")
	os.Chmod(script, 0755)
	diffGitTest(t, dir, "config", "diff.external", script)
	diffGitTest(t, dir, "config", "core.fsmonitor", script)
	diffGitTest(t, dir, "config", "diff.hostile.textconv", script)
	writeDiffTest(t, dir, ".gitattributes", "file.txt diff=hostile\n")
	writeDiffTest(t, dir, ".gitignore", "file.txt\n")
	writeDiffTest(t, dir, "file.txt", "still included\n")
	r := inspectDiffTest(t, dir)
	found := false
	for _, f := range r.Files {
		if f.Path == "file.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("tracked ignored file excluded")
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("configured helper executed")
	}
}
func TestWorktreeDiffUnmergedAndUnrelated(t *testing.T) {
	dir := diffFixture(t)
	writeDiffTest(t, dir, "file.txt", "feature\n")
	diffGitTest(t, dir, "commit", "-am", "feature")
	diffGitTest(t, dir, "checkout", "main")
	writeDiffTest(t, dir, "file.txt", "main\n")
	diffGitTest(t, dir, "commit", "-am", "main")
	diffGitTest(t, dir, "checkout", "feat/test")
	_ = exec.Command("git", "-C", dir, "merge", "main").Run()
	_, e := InspectWorktree(context.Background(), dir, "feat/test", dir)
	if de, ok := e.(*DiffError); !ok || de.Code != "unmerged_index" {
		t.Fatalf("conflict: %v", e)
	}
	diffGitTest(t, dir, "merge", "--abort")
	diffGitTest(t, dir, "checkout", "--orphan", "unrelated")
	diffGitTest(t, dir, "commit", "-am", "unrelated")
	diffGitTest(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	diffGitTest(t, dir, "checkout", "feat/test")
	_, e = InspectWorktree(context.Background(), dir, "feat/test", dir)
	if de, ok := e.(*DiffError); !ok || de.Code != "baseline_unavailable" {
		t.Fatalf("unrelated: %v", e)
	}
}
func TestWorktreeDiffSubmodule(t *testing.T) {
	source := diffFixture(t)
	dir := diffFixture(t)
	diffGitTest(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", source, "sub")
	diffGitTest(t, dir, "commit", "-am", "submodule")
	diffGitTest(t, dir, "branch", "-f", "main", "HEAD")
	writeDiffTest(t, filepath.Join(dir, "sub"), "file.txt", "dirty\n")
	r := inspectDiffTest(t, dir)
	if len(r.Files) != 1 || r.Files[0].Kind != "submodule" || r.Files[0].Additions != nil {
		t.Fatal("dirty submodule missing")
	}
}
func TestDiffBufferBounds(t *testing.T) {
	b := &diffBuffer{limit: 4}
	_, e := b.Write([]byte("12345"))
	if e == nil || b.buffer.Len() != 0 || !b.exceeded {
		t.Fatal("unbounded collector")
	}
}

func TestWorktreeDiffManyFiles(t *testing.T) {
	dir := diffFixture(t)
	for i := 0; i < 1002; i++ {
		writeDiffTest(t, dir, fmt.Sprintf("new-%04d", i), "new\n")
	}
	r := inspectDiffTest(t, dir)
	if len(r.Files) != 1000 || r.Complete || !r.CountsPartial || r.IsClean || r.Files[999].Path != "new-0999" {
		t.Fatalf("invalid bounded result: files=%d complete=%v", len(r.Files), r.Complete)
	}
	encoded, _ := json.Marshal(r)
	if len(encoded) > diffResponseLimit {
		t.Fatal("response bound exceeded")
	}
}
func TestWorktreeDiffNewSubmodule(t *testing.T) {
	source := diffFixture(t)
	dir := diffFixture(t)
	diffGitTest(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", source, "sub")
	r := inspectDiffTest(t, dir)
	for _, f := range r.Files {
		if f.Path == "sub" {
			if f.Kind != "submodule" || f.Status != "added" {
				t.Fatal("new submodule kind")
			}
			return
		}
	}
	t.Fatal("new submodule missing")
}
func TestWorktreeDiffWorktreeAndShallowHistory(t *testing.T) {
	dir := diffFixture(t)
	wt := filepath.Join(t.TempDir(), "checkout")
	diffGitTest(t, dir, "worktree", "add", "-b", "feat/worktree", wt)
	writeDiffTest(t, wt, "new", "new\n")
	r, e := InspectWorktree(context.Background(), wt, "feat/worktree", dir)
	if e != nil || r.FilesChanged != 1 {
		t.Fatalf("linked worktree: %v", e)
	}
	// Clone only the advanced task tip, then provide a default ref with no history.
	writeDiffTest(t, dir, "file.txt", "feature\n")
	diffGitTest(t, dir, "commit", "-am", "feature")
	clone := filepath.Join(t.TempDir(), "shallow")
	diffGitTest(t, dir, "clone", "--depth=1", "--branch", "feat/test", "file://"+dir, clone)
	_, e = InspectWorktree(context.Background(), clone, "feat/test", clone)
	if e == nil {
		t.Fatal("missing shallow baseline accepted")
	}
}
func TestWorktreeDiffMultipleAncestors(t *testing.T) {
	dir := diffFixture(t)
	base := diffGitTest(t, dir, "rev-parse", "HEAD")
	tree := diffGitTest(t, dir, "rev-parse", "HEAD^{tree}")
	a := diffGitTest(t, dir, "commit-tree", tree, "-p", base, "-m", "a")
	b := diffGitTest(t, dir, "commit-tree", tree, "-p", base, "-m", "b")
	one := diffGitTest(t, dir, "commit-tree", tree, "-p", a, "-p", b, "-m", "one")
	two := diffGitTest(t, dir, "commit-tree", tree, "-p", b, "-p", a, "-m", "two")
	diffGitTest(t, dir, "update-ref", "refs/heads/main", one)
	diffGitTest(t, dir, "update-ref", "refs/heads/feat/test", two)
	_, e := InspectWorktree(context.Background(), dir, "feat/test", dir)
	if de, ok := e.(*DiffError); !ok || de.Code != "baseline_unavailable" {
		t.Fatalf("ambiguous history: %v", e)
	}
}

func TestWorktreeDiffEmptyAndRefPrecedence(t *testing.T) {
	empty := t.TempDir()
	diffGitTest(t, empty, "init", "-b", "feat/test")
	if _, e := InspectWorktree(context.Background(), empty, "feat/test", empty); e == nil {
		t.Fatal("empty repository accepted")
	}
	dir := diffFixture(t)
	diffGitTest(t, dir, "update-ref", "refs/remotes/origin/master", "HEAD")
	if inspectDiffTest(t, dir).BaseRef != "refs/remotes/origin/master" {
		t.Fatal("remote master precedence")
	}
	diffGitTest(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	if inspectDiffTest(t, dir).BaseRef != "refs/remotes/origin/main" {
		t.Fatal("remote main precedence")
	}
}
func TestWorktreeDiffDeletionTypeChangeAndTemporaryCleanup(t *testing.T) {
	dir := diffFixture(t)
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	before := diffGitTest(t, dir, "count-objects", "-v")
	os.Remove(filepath.Join(dir, "file.txt"))
	r := inspectDiffTest(t, dir)
	if len(r.Files) != 1 || r.Files[0].Status != "deleted" || *r.Files[0].Deletions != 1 {
		t.Fatal("deletion")
	}
	os.Symlink("/does-not-exist", filepath.Join(dir, "file.txt"))
	writeDiffTest(t, dir, "z-after", "after\n")
	r = inspectDiffTest(t, dir)
	if len(r.Files) != 2 || !strings.Contains(r.Files[1].Patch, "+after") {
		t.Fatal("patch after type change")
	}
	if *r.Files[0].Additions != 1 || *r.Files[0].Deletions != 1 {
		t.Fatal("type change counts")
	}
	if r.Files[0].Kind != "symlink" || r.Files[0].Status != "type-changed" || !strings.Contains(r.Files[0].Patch, "/does-not-exist") {
		t.Fatalf("type change: %+v", r.Files)
	}
	if before != diffGitTest(t, dir, "count-objects", "-v") {
		t.Fatal("real object store changed")
	}
	files, e := os.ReadDir(temp)
	if e != nil {
		t.Fatal(e)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "sectile-diff-") {
			t.Fatal("temporary snapshot not cleaned")
		}
	}
}
func TestWorktreeDiffConcurrentEdits(t *testing.T) {
	dir := diffFixture(t)
	for i := 0; i < 150; i++ {
		writeDiffTest(t, dir, fmt.Sprintf("new-%d", i), "new\n")
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for n := 0; ; n++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte(fmt.Sprintf("edit %d\n", n)), 0644)
			time.Sleep(time.Millisecond)
		}
	}()
	_, e := InspectWorktree(context.Background(), dir, "feat/test", dir)
	close(stop)
	<-done
	if de, ok := e.(*DiffError); !ok || de.Code != "checkout_changed" {
		t.Fatalf("concurrent writes: %v", e)
	}
}

func TestWorktreeDiffUnsupportedEntry(t *testing.T) {
	dir := diffFixture(t)
	if e := exec.Command("mkfifo", filepath.Join(dir, "pipe")).Run(); e != nil {
		t.Skip("mkfifo unavailable")
	}
	r := inspectDiffTest(t, dir)
	if len(r.Files) != 1 || r.Files[0].Kind != "unsupported" || r.Complete {
		t.Fatal("unsupported entry not marked")
	}
}
func TestWorktreeDiffAggregatePatchBound(t *testing.T) {
	dir := diffFixture(t)
	for i := 0; i < 24; i++ {
		writeDiffTest(t, dir, fmt.Sprintf("large-%02d", i), strings.Repeat("a moderately long line of text\n", 7000))
	}
	r := inspectDiffTest(t, dir)
	if r.Complete || !r.CountsPartial || r.IsClean {
		t.Fatal("aggregate omission not marked")
	}
	encoded, _ := json.Marshal(r)
	if len(encoded) > diffResponseLimit {
		t.Fatal("serialized response exceeds budget")
	}
}
