package agent

import (
	"golang.org/x/sys/windows"
	"testing"
)

// The clone of a repository the workstation lacks (#737) runs for the agent
// alone, so it must not open a console window.
func TestCloneDoesNotOpenAWindowsConsole(t *testing.T) {
	cmd := cloneCommand(t.Context(), "https://github.com/o/b.git", t.TempDir())
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("git clone would open a console window")
	}
}
