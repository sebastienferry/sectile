package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	diffGitTest(t, dir, "config", "submodule.sub.ignore", "all")
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
	// The source repository has to answer `main` for its own HEAD. A clone
	// records the remote's HEAD as origin/HEAD, and git does so for a
	// single-branch clone from some versions on: with the fixture left on
	// feat/test, origin/HEAD would name the one branch the shallow clone did
	// fetch and the baseline this test denies would exist after all.
	diffGitTest(t, dir, "checkout", "main")
	clone := filepath.Join(t.TempDir(), "shallow")
	diffGitTest(t, dir, "clone", "--depth=1", "--branch", "feat/test", "file://"+dir, clone)
	// The premise, stated rather than assumed: the default branch is absent
	// from the clone, whether or not this git recorded a dangling origin/HEAD.
	if out, err := exec.Command("git", "-C", clone, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}").Output(); err == nil {
		t.Fatalf("the shallow clone fetched the default branch after all: %s", out)
	}
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

func TestWorktreeDiffDirectoryReplacement(t *testing.T) {
	dir := diffFixture(t)
	os.Remove(filepath.Join(dir, "file.txt"))
	os.Mkdir(filepath.Join(dir, "file.txt"), 0755)
	writeDiffTest(t, filepath.Join(dir, "file.txt"), "child", "child\n")
	r := inspectDiffTest(t, dir)
	if len(r.Files) != 2 {
		t.Fatal("file replaced by directory")
	}
	diffGitTest(t, dir, "add", ".")
	diffGitTest(t, dir, "commit", "-m", "directory")
	diffGitTest(t, dir, "branch", "-f", "main", "HEAD")
	os.RemoveAll(filepath.Join(dir, "file.txt"))
	writeDiffTest(t, dir, "file.txt", "replacement\n")
	r = inspectDiffTest(t, dir)
	if len(r.Files) != 2 {
		t.Fatal("directory replaced by file")
	}
}

func diffDocumentsTest(t *testing.T, dir string) map[string]WorktreeDiffFile {
	t.Helper()
	files := map[string]WorktreeDiffFile{}
	for _, f := range inspectDiffTest(t, dir).Files {
		files[f.Path] = f
	}
	return files
}

func TestWorktreeDiffMarkdownDocuments(t *testing.T) {
	dir := diffFixture(t)
	writeDiffTest(t, dir, "committed.md", "# Base\n")
	writeDiffTest(t, dir, "deleted.md", "# Gone\n\nOld body.\n")
	writeDiffTest(t, dir, "doc.txt", "# Becomes Markdown\n")
	writeDiffTest(t, dir, "notes.md", "# Becomes text\n")
	diffGitTest(t, dir, "add", ".")
	diffGitTest(t, dir, "commit", "-m", "documents")
	diffGitTest(t, dir, "checkout", "main")
	diffGitTest(t, dir, "merge", "--ff-only", "feat/test")
	diffGitTest(t, dir, "checkout", "feat/test")

	writeDiffTest(t, dir, "committed.md", "# Committed\n")
	writeDiffTest(t, dir, "added.md", "# Added\n")
	diffGitTest(t, dir, "add", ".")
	diffGitTest(t, dir, "commit", "-m", "work")
	writeDiffTest(t, dir, "staged.md", "# Staged\n")
	diffGitTest(t, dir, "add", "staged.md")
	writeDiffTest(t, dir, "committed.md", "# Committed\n\nThen edited, not staged.\n")
	writeDiffTest(t, dir, "README.MARKDOWN", "# Upper case\n")
	writeDiffTest(t, dir, "untracked.Md", "# Untracked\n")
	writeDiffTest(t, dir, "page.mdx", "# MDX\n")
	writeDiffTest(t, dir, "plain.txt", "# Text\n")
	writeDiffTest(t, dir, "empty.md", "")
	diffGitTest(t, dir, "mv", "doc.txt", "doc.md")
	diffGitTest(t, dir, "mv", "notes.md", "notes.txt")
	diffGitTest(t, dir, "rm", "-q", "deleted.md")

	files := diffDocumentsTest(t, dir)
	want := map[string]DiffDocument{
		"committed.md":    {Side: "new", Content: "# Committed\n\nThen edited, not staged.\n"},
		"added.md":        {Side: "new", Content: "# Added\n"},
		"staged.md":       {Side: "new", Content: "# Staged\n"},
		"README.MARKDOWN": {Side: "new", Content: "# Upper case\n"},
		"untracked.Md":    {Side: "new", Content: "# Untracked\n"},
		"doc.md":          {Side: "new", Content: "# Becomes Markdown\n"},
		"deleted.md":      {Side: "old", Content: "# Gone\n\nOld body.\n"},
		"empty.md":        {Side: "new"},
	}
	for p, document := range want {
		f, ok := files[p]
		if !ok || f.Document == nil || !reflect.DeepEqual(*f.Document, document) {
			t.Fatalf("%s: got %+v, want %+v", p, f.Document, document)
		}
		if f.Patch == "" && p != "empty.md" {
			t.Fatalf("%s lost its patch", p)
		}
	}
	if files["doc.md"].OldPath != "doc.txt" || files["deleted.md"].Status != "deleted" {
		t.Fatalf("unexpected statuses: %+v %+v", files["doc.md"], files["deleted.md"])
	}
	for _, p := range []string{"page.mdx", "plain.txt", "notes.txt"} {
		if f, ok := files[p]; !ok || f.Document != nil {
			t.Fatalf("%s: want listed without a document, got %+v", p, f)
		}
	}
}

func TestWorktreeDiffMarkdownDocumentReasons(t *testing.T) {
	dir := diffFixture(t)
	writeDiffTest(t, dir, "binary.md", string([]byte{0, 1, 2}))
	// Latin-1 text whose patch is too large to be checked for UTF-8 keeps the text kind.
	writeDiffTest(t, dir, "latin1.md", strings.Repeat("caf\xe9 au lait\n", 25000))
	writeDiffTest(t, dir, "oversized.md", strings.Repeat("a long Markdown line\n", 30000))
	writeDiffTest(t, dir, "huge.md", strings.Repeat("x", diffMetadataLimit+1))
	files := diffDocumentsTest(t, dir)
	if files["binary.md"].Kind != "binary" || files["binary.md"].Document != nil {
		t.Fatalf("binary Markdown: %+v", files["binary.md"])
	}
	for p, reason := range map[string]string{
		"latin1.md":    "Non-UTF-8 contents cannot be rendered.",
		"oversized.md": "File exceeds the 512 KiB rendering limit.",
		"huge.md":      "File exceeds the 8 MiB inspection limit.",
	} {
		document := files[p].Document
		if document == nil || document.OmittedReason != reason || document.Content != "" {
			t.Fatalf("%s: got %+v, want %q", p, document, reason)
		}
	}
}

func TestWorktreeDiffMarkdownDocumentBudget(t *testing.T) {
	dir := diffFixture(t)
	body := strings.Repeat("A paragraph of Markdown text.\n", (diffDocumentLimit-64)/30)
	for i := 0; i < 9; i++ {
		writeDiffTest(t, dir, fmt.Sprintf("big-%d.md", i), body)
	}
	writeDiffTest(t, dir, "a-small.txt", "small\n")
	writeDiffTest(t, dir, "a.md", "# First\n")
	writeDiffTest(t, dir, "zz.md", "# Last\n")
	files := diffDocumentsTest(t, dir)
	if len(files) != 12 {
		t.Fatalf("every changed file stays listed: %d", len(files))
	}
	if files["a.md"].Document.Content != "# First\n" || files["a.md"].Patch == "" || files["a-small.txt"].Patch == "" {
		t.Fatalf("documents within the budget keep their patch: %+v", files["a.md"])
	}
	rendered := 0
	for i := 0; i < 9; i++ {
		document := files[fmt.Sprintf("big-%d.md", i)].Document
		if document.Content == body {
			rendered++
		} else if document.OmittedReason != "Rendering skipped: the 4 MiB rendering budget was reached." {
			t.Fatalf("big-%d.md: %+v", i, document.OmittedReason)
		}
	}
	if rendered != (diffDocumentBudget-len("# First\n"))/len(body) {
		t.Fatalf("rendered %d documents within the budget", rendered)
	}
	// The aggregate patch limit applies as it did before documents existed.
	last := files["zz.md"]
	if last.Document.OmittedReason != "Rendering skipped: the 4 MiB rendering budget was reached." || last.OmittedReason != "Patch omitted because the aggregate display limit was reached." {
		t.Fatalf("zz.md: %+v %+v", last, last.Document)
	}
}

const pngTest = "\x89PNG\r\n\x1a\n"

func diffImagesTest(t *testing.T, dir string) (*WorktreeDiff, map[string]WorktreeDiffFile) {
	t.Helper()
	r := inspectDiffTest(t, dir)
	files := map[string]WorktreeDiffFile{}
	for _, f := range r.Files {
		files[f.Path] = f
	}
	return r, files
}

func TestWorktreeDiffMarkdownImages(t *testing.T) {
	dir := diffFixture(t)
	for _, d := range []string{"docs/images", "docs/dir", "web"} {
		if e := os.MkdirAll(filepath.Join(dir, d), 0755); e != nil {
			t.Fatal(e)
		}
	}
	writeDiffTest(t, dir, "docs/guide.md", "# Guide\n")
	writeDiffTest(t, dir, "docs/images/flow.png", pngTest+"flow")
	writeDiffTest(t, dir, "docs/changed.png", pngTest+"before")
	writeDiffTest(t, dir, "docs/shot.png", pngTest+"baseline shot")
	writeDiffTest(t, dir, "docs/old.md", "![Shot](shot.png) ![Added](added.png)\n")
	writeDiffTest(t, dir, "docs/dir/child.txt", "child\n")
	writeDiffTest(t, dir, "docs/lfs.png", "version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 12345\n")
	writeDiffTest(t, dir, "docs/text.png", "not an image\n")
	writeDiffTest(t, dir, "docs/real.svg", "\x89PNG\r\n\x1a\nnamed svg")
	writeDiffTest(t, dir, "web/logo.svg", `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`)
	if e := os.Symlink("images/flow.png", filepath.Join(dir, "docs/link.png")); e != nil {
		t.Fatal(e)
	}
	diffGitTest(t, dir, "add", ".")
	diffGitTest(t, dir, "commit", "-m", "documents")
	diffGitTest(t, dir, "checkout", "main")
	diffGitTest(t, dir, "merge", "--ff-only", "feat/test")
	diffGitTest(t, dir, "checkout", "feat/test")

	writeDiffTest(t, dir, "docs/guide.md", strings.Join([]string{
		"![Flow](images/flow.png \"Request flow\")",
		"![Changed](changed.png) ![New](<new image.png>) ![Logo](/web/logo.svg#dark)",
		"![Missing](missing.png) ![Dir](dir) ![Link](link.png) ![LFS](lfs.png) ![Text](text.png?raw=1)",
		"![Named](real.svg) ![Huge](huge.png) ![Out](../../outside.png) ![Remote](https://example.com/a.png)",
		"![Again](./images/flow.png)",
		"",
	}, "\n"))
	writeDiffTest(t, dir, "docs/second.md", "![Flow](images/flow.png) ![Changed](changed.png)\n")
	writeDiffTest(t, dir, "docs/changed.png", pngTest+"after")
	writeDiffTest(t, dir, "docs/new image.png", "GIF89a new")
	writeDiffTest(t, dir, "docs/added.png", pngTest+"added")
	writeDiffTest(t, dir, "docs/shot.png", pngTest+"modified shot")
	writeDiffTest(t, dir, "docs/huge.png", pngTest+strings.Repeat("x", diffMetadataLimit))
	diffGitTest(t, dir, "rm", "-q", "docs/old.md")

	r, files := diffImagesTest(t, dir)
	refs := func(p string) []DiffImageRef { return files[p].Document.Images }
	if got, want := refs("docs/guide.md"), []DiffImageRef{
		{Path: "docs/images/flow.png", Image: "new:docs/images/flow.png"},
		{Path: "docs/changed.png", Image: "new:docs/changed.png"},
		{Path: "docs/new image.png", Image: "new:docs/new image.png"},
		{Path: "web/logo.svg", Image: "new:web/logo.svg"},
		{Path: "docs/missing.png", OmittedReason: "Image not found in the inspected state."},
		{Path: "docs/dir", OmittedReason: "Image not found in the inspected state."},
		{Path: "docs/link.png", OmittedReason: "Symbolic links are not followed."},
		{Path: "docs/lfs.png", OmittedReason: "This file is not a supported image."},
		{Path: "docs/text.png", OmittedReason: "This file is not a supported image."},
		{Path: "docs/real.svg", Image: "new:docs/real.svg"},
		{Path: "docs/huge.png", OmittedReason: "File exceeds the 8 MiB inspection limit."},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("guide.md refs:\n got %+v\nwant %+v", got, want)
	}
	if got, want := refs("docs/second.md"), []DiffImageRef{
		{Path: "docs/images/flow.png", Image: "new:docs/images/flow.png"},
		{Path: "docs/changed.png", Image: "new:docs/changed.png"},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second.md refs: %+v", got)
	}
	// A deleted document reads its images from the merge-base.
	if got, want := refs("docs/old.md"), []DiffImageRef{
		{Path: "docs/shot.png", Image: "old:docs/shot.png"},
		{Path: "docs/added.png", OmittedReason: "Image not found in the inspected state."},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("old.md refs: %+v", got)
	}
	want := map[string]DiffImage{
		"new:docs/images/flow.png": {"image/png", []byte(pngTest + "flow")},
		"new:docs/changed.png":     {"image/png", []byte(pngTest + "after")},
		"new:docs/new image.png":   {"image/gif", []byte("GIF89a new")},
		"new:web/logo.svg":         {"image/svg+xml", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`)},
		"new:docs/real.svg":        {"image/png", []byte(pngTest + "named svg")},
		"old:docs/shot.png":        {"image/png", []byte(pngTest + "baseline shot")},
	}
	if !reflect.DeepEqual(r.Images, want) {
		t.Fatalf("images: got %d %+v", len(r.Images), r.Images)
	}
	encoded, _ := json.Marshal(r)
	if !strings.Contains(string(encoded), `"new:docs/changed.png":{"mimeType":"image/png","data":"iVBORw0KGgphZnRlcg=="}`) {
		t.Fatalf("image not Base64 in JSON: %s", encoded)
	}
}

func TestWorktreeDiffMarkdownImageBudget(t *testing.T) {
	limit, budget := diffImageLimit, diffImageBudget
	diffImageLimit, diffImageBudget = 100, 250
	t.Cleanup(func() { diffImageLimit, diffImageBudget = limit, budget })
	dir := diffFixture(t)
	image := func(tag string, size int) string {
		return pngTest + tag + strings.Repeat("x", size-len(pngTest)-len(tag))
	}
	writeDiffTest(t, dir, "x1.png", image("1", 80))
	writeDiffTest(t, dir, "big.png", image("big", 101))
	writeDiffTest(t, dir, "x2.png", image("2", 80))
	writeDiffTest(t, dir, "text.png", strings.Repeat("t", 80))
	writeDiffTest(t, dir, "x3.png", image("3", 80))
	writeDiffTest(t, dir, "x4.png", image("4", 10))
	// File order is path order: a.md, then b.md, whatever the references.
	writeDiffTest(t, dir, "b.md", "![one](x1.png) ![three](x3.png) ![four](x4.png)\n")
	writeDiffTest(t, dir, "a.md", "![one](x1.png) ![big](big.png) ![two](x2.png) ![text](text.png)\n")
	r, files := diffImagesTest(t, dir)
	skipped := "Image skipped: the 8 MiB image budget was reached."
	if got, want := files["a.md"].Document.Images, []DiffImageRef{
		{Path: "x1.png", Image: "new:x1.png"},
		{Path: "big.png", OmittedReason: "Image exceeds the 2 MiB display limit."},
		{Path: "x2.png", Image: "new:x2.png"},
		// A refused format still spent its reservation.
		{Path: "text.png", OmittedReason: "This file is not a supported image."},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("a.md refs: %+v", got)
	}
	// x1 is shared and counted once; x3 overflows, and x4 follows it even
	// though it would fit.
	if got, want := files["b.md"].Document.Images, []DiffImageRef{
		{Path: "x1.png", Image: "new:x1.png"},
		{Path: "x3.png", OmittedReason: skipped},
		{Path: "x4.png", OmittedReason: skipped},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("b.md refs: %+v", got)
	}
	if len(r.Images) != 2 {
		t.Fatalf("images carried: %d", len(r.Images))
	}
}

func TestWorktreeDiffMarkdownImagesLeaveTheRestUnchanged(t *testing.T) {
	dir := diffFixture(t)
	writeDiffTest(t, dir, "shot.png", pngTest+"shot")
	writeDiffTest(t, dir, "guide.md", "# Guide\n\n![Shot](shot.png)\n")
	writeDiffTest(t, dir, "oversized.md", "![Shot](shot.png)\n"+strings.Repeat("a long Markdown line\n", 30000))
	with, files := diffImagesTest(t, dir)
	if document := files["oversized.md"].Document; document.OmittedReason == "" || document.Images != nil {
		t.Fatalf("an omitted document carries no image: %+v", document)
	}
	if len(with.Images) != 1 || files["guide.md"].Document.Images == nil {
		t.Fatalf("guide.md image missing: %+v", with.Images)
	}
	attach := attachImages
	attachImages = func(diffGit, *WorktreeDiff, map[string]WorktreeDiffFile, string, string) error { return nil }
	t.Cleanup(func() { attachImages = attach })
	without := inspectDiffTest(t, dir)
	with.Images = nil
	for i := range with.Files {
		if with.Files[i].Document != nil {
			with.Files[i].Document.Images = nil
		}
	}
	with.GeneratedAt, without.GeneratedAt = "", ""
	if !reflect.DeepEqual(with, without) {
		t.Fatal("images changed the files, patches or documents")
	}
}

func TestWorktreeDiffMarkdownImageSubmodule(t *testing.T) {
	dir := diffFixture(t)
	head := diffGitTest(t, dir, "rev-parse", "HEAD")
	diffGitTest(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",vendor")
	diffGitTest(t, dir, "commit", "-m", "submodule")
	diffGitTest(t, dir, "branch", "-f", "main", "HEAD")
	writeDiffTest(t, dir, "guide.md", "![Sub](vendor) ![Inside](vendor/logo.png)\n")
	_, files := diffImagesTest(t, dir)
	notFound := "Image not found in the inspected state."
	if got, want := files["guide.md"].Document.Images, []DiffImageRef{
		{Path: "vendor", OmittedReason: notFound},
		{Path: "vendor/logo.png", OmittedReason: notFound},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("submodule refs: %+v", got)
	}
}
