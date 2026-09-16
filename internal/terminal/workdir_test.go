package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pty.Start reports an unusable working directory as "fork/exec /bin/sh: not a
// directory", which names the shell and hides the path that is actually wrong.
// A run that fails this way is only diagnosable if the message says which
// directory, and why.
func TestUnusableWorkingDirectoryNamesThePath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	manager := NewManager()
	_, err := manager.GetOrCreateSession("session", file, nil)
	if err == nil {
		t.Fatal("a file was accepted as a working directory")
	}
	if !strings.Contains(err.Error(), file) {
		t.Errorf("error %q does not name the path", err)
	}
	if strings.Contains(err.Error(), "/bin/sh") {
		t.Errorf("error %q blames the shell instead of the directory", err)
	}
}

// A path under a file cannot be created, and that failure must be reported
// against the path rather than surfacing later as a shell error.
func TestWorkingDirectoryThatCannotBeCreatedIsReported(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "blocking-file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	unreachable := filepath.Join(file, "workdir")

	manager := NewManager()
	_, err := manager.GetOrCreateSession("session", unreachable, nil)
	if err == nil {
		t.Fatal("an uncreatable working directory was accepted")
	}
	if !strings.Contains(err.Error(), "workdir") {
		t.Errorf("error %q does not name the path", err)
	}
}
