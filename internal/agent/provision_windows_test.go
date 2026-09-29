//go:build windows

package agent

import (
	"context"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// The desktop starts the agent detached, so it owns no console: the npm ci that
// follows a worktree's creation would get a window of its own and keep it open for
// the whole install. It must say it needs none, and keep its folder and its bound
// on a killed npm.
func TestNpmInstallRunsWithoutAConsoleWindow(t *testing.T) {
	dir := t.TempDir()
	cmd := npmCommand(context.Background(), dir)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("npm ci would open a console window: %+v", cmd.SysProcAttr)
	}
	if cmd.Dir != dir {
		t.Fatalf("npm ci runs in %q, want %q", cmd.Dir, dir)
	}
	if cmd.WaitDelay != 10*time.Second {
		t.Fatalf("npm ci lost its wait delay: %v", cmd.WaitDelay)
	}
}
