//go:build windows

package sddfiles

import (
	"context"
	"testing"

	"golang.org/x/sys/windows"
)

// The agent reads a macro's specification files for the board. Started by the
// desktop, it owns no console, so each git call it makes for that would flash a
// window of its own unless it says it needs none.
func TestGitOutputRunsWithoutAConsoleWindow(t *testing.T) {
	repo := t.TempDir()
	cmd := gitCommand(context.Background(), repo, "for-each-ref")
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("git would open a console window: %+v", cmd.SysProcAttr)
	}
	if cmd.Dir != repo {
		t.Fatalf("git runs in %q, want %q", cmd.Dir, repo)
	}
}
