package sddfiles

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const tasksFile = "## 1. First group\n\n- [ ] 1.1 Do it.\n"

func writeSpecDir(t *testing.T, folder, root, name string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(folder, filepath.FromSlash(root), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for file, content := range files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func gitRepo(t *testing.T) (string, func(...string)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-qm", "first")
	return repo, run
}

// The folder is looked for by the prefix of the key, ignoring case: an
// OpenSpec change carries the key in lower case.
func TestFindMacroDirMatchesThePrefixIgnoringCase(t *testing.T) {
	folder := t.TempDir()
	dir := writeSpecDir(t, folder, "openspec/changes", "pe-69-slicing-from-sdd", nil)

	got, err := FindMacroDir(folder, "openspec", "PE-69")
	if err != nil || got != dir {
		t.Fatalf("expected %q, got %q (%v)", dir, got, err)
	}
}

// The framework decides the root: specs under Spec Kit, openspec/changes
// under OpenSpec.
func TestFindMacroDirSearchesTheFrameworkRoot(t *testing.T) {
	folder := t.TempDir()
	writeSpecDir(t, folder, "specs", "pe-70-speckit", nil)

	if _, err := FindMacroDir(folder, "speckit", "PE-70"); err != nil {
		t.Fatalf("not found under specs: %v", err)
	}
	if _, err := FindMacroDir(folder, "openspec", "PE-70"); err == nil {
		t.Fatal("an OpenSpec project must not read the Spec Kit root")
	}
}

func TestFindMacroDirWithoutFolderNamesTheSetting(t *testing.T) {
	_, err := FindMacroDir("", "openspec", "PE-69")
	if err == nil || !strings.Contains(err.Error(), "Specifications folder") {
		t.Fatalf("the refusal must name the setting, got %v", err)
	}
}

// Two folders for one key: the most recently modified wins.
func TestFindMacroDirPrefersTheMostRecentFolder(t *testing.T) {
	folder := t.TempDir()
	old := writeSpecDir(t, folder, "openspec/changes", "pe-69-abandoned", nil)
	recent := writeSpecDir(t, folder, "openspec/changes", "pe-69-resumed", nil)
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	got, err := FindMacroDir(folder, "openspec", "pe-69")
	if err != nil || got != recent {
		t.Fatalf("expected the resumed folder, got %q (%v)", got, err)
	}
}

// A plain folder, outside any Git checkout, is read like any other.
func TestReadFromAPlainFolder(t *testing.T) {
	folder := t.TempDir()
	dir := writeSpecDir(t, folder, "specs", "M-1-plain", map[string]string{"tasks.md": tasksFile})

	content, origin, err := Read(context.Background(), folder, "speckit", "M-1", "tasks.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if content != tasksFile || origin != filepath.Join(dir, "tasks.md") {
		t.Fatalf("unexpected read: %q from %q", content, origin)
	}
}

// A plain folder has no branch: the refusal names the folder and the setting,
// and never suggests a branch.
func TestReadMissingInAPlainFolderDoesNotMentionABranch(t *testing.T) {
	folder := t.TempDir()

	_, _, err := Read(context.Background(), folder, "speckit", "M-2", "tasks.md")
	if err == nil {
		t.Fatal("a missing folder must be refused")
	}
	msg := err.Error()
	if !strings.Contains(msg, "M-2") || !strings.Contains(msg, folder) || !strings.Contains(msg, "Specifications folder") {
		t.Fatalf("the refusal must name the macro, the folder and the setting: %v", err)
	}
	if strings.Contains(msg, "branche") {
		t.Fatalf("a plain folder has no branch to suggest: %v", err)
	}
}

// The folder is there but not the chosen file: the other source is suggested.
func TestReadMissingFileSuggestsTheOtherSource(t *testing.T) {
	folder := t.TempDir()
	writeSpecDir(t, folder, "specs", "M-3-spec-only", map[string]string{"spec.md": "# spec\n"})

	_, _, err := Read(context.Background(), folder, "speckit", "M-3", "tasks.md")
	if err == nil || !strings.Contains(err.Error(), "tasks.md") || !strings.Contains(err.Error(), "l'autre") {
		t.Fatalf("the refusal must name the file and suggest the other source: %v", err)
	}
}

func TestReadRefusesAnUnknownFile(t *testing.T) {
	if _, _, err := Read(context.Background(), t.TempDir(), "speckit", "M-4", "../secret"); err == nil {
		t.Fatal("only tasks.md and spec.md may be read")
	}
}

// On a Git folder the macro's branch is read when the working tree does not
// carry its folder, so slicing works before the branch is merged.
func TestReadFallsBackOnTheMacroBranchOfAGitFolder(t *testing.T) {
	repo, run := gitRepo(t)
	run("checkout", "-q", "-b", "PE-446-on-the-branch")
	writeSpecDir(t, repo, "openspec/changes", "pe-446-on-the-branch", map[string]string{"tasks.md": tasksFile})
	run("add", ".")
	run("commit", "-qm", "spec")
	run("checkout", "-q", "main")

	content, origin, err := Read(context.Background(), repo, "openspec", "PE-446", "tasks.md")
	if err != nil {
		t.Fatalf("the branch fallback must work: %v", err)
	}
	if content != tasksFile || origin != "PE-446-on-the-branch:openspec/changes/pe-446-on-the-branch/tasks.md" {
		t.Fatalf("unexpected read: %q from %q", content, origin)
	}
}

// On a Git folder with nothing anywhere, the branch hypothesis is part of the
// refusal.
func TestReadMissingInAGitFolderSuggestsTheBranch(t *testing.T) {
	repo, _ := gitRepo(t)

	_, _, err := Read(context.Background(), repo, "speckit", "PE-447", "tasks.md")
	if err == nil || !strings.Contains(err.Error(), "branche") || !strings.Contains(err.Error(), "Specifications folder") {
		t.Fatalf("the refusal must suggest the branch and name the setting: %v", err)
	}
}
