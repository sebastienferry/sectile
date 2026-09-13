package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchOperationsPreserveWork(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s (%v)", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-b", "main")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial")
	git("branch", "existing")
	original := git("rev-parse", "existing")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Later")
	dirty := filepath.Join(root, "personal.txt")
	if err := os.WriteFile(dirty, []byte("unfinished"), 0600); err != nil {
		t.Fatal(err)
	}
	w := New(context.Background())
	if _, err := w.SwitchGitBranch(root, "existing", true); err == nil {
		t.Fatal("creation reset an existing branch")
	}
	if git("rev-parse", "existing") != original {
		t.Fatal("existing commit changed")
	}
	if raw, _ := os.ReadFile(dirty); string(raw) != "unfinished" {
		t.Fatal("uncommitted work changed")
	}
	if _, err := w.SwitchGitBranch(root, "--orphan", true); err == nil {
		t.Fatal("branch accepted as option")
	}
	if err := w.DeleteGitBranch(root, "main", false); err == nil {
		t.Fatal("deleted primary branch")
	}
}
