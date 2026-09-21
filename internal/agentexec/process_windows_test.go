//go:build windows

package agentexec

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

// A headless run is stopped by addressing its process group. Without a group of
// its own the child stays in the agent's, and the break event meant for it goes
// to every process sharing that console — the agent, and whatever started it.
func TestDetachedSessionGivesTheChildItsOwnGroup(t *testing.T) {
	attrs := DetachedSession()
	if attrs == nil {
		t.Fatal("a detached child needs process attributes on Windows")
	}
	if attrs.CreationFlags&windows.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatalf("detached child is not its own process group: flags %#x", attrs.CreationFlags)
	}
}

// The desktop starts the agent detached, so it owns no console: every console
// child it starts is handed a window of its own. Anything the agent runs to read
// its own state must say it needs no console at all.
func TestHiddenSuppressesTheConsoleWindow(t *testing.T) {
	cmd := Hidden(exec.Command("git", "status"))
	if cmd.SysProcAttr == nil {
		t.Fatal("a hidden child needs process attributes on Windows")
	}
	if cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("hidden child would open a console window: flags %#x", cmd.SysProcAttr.CreationFlags)
	}
}

// A headless run asks for both: its own process group, so a stop reaches it, and
// no window. Hidden is applied after DetachedSession and must not undo it.
func TestHiddenKeepsAnExistingProcessGroup(t *testing.T) {
	cmd := exec.Command("git", "status")
	cmd.SysProcAttr = DetachedSession()
	Hidden(cmd)
	flags := cmd.SysProcAttr.CreationFlags
	if flags&windows.CREATE_NEW_PROCESS_GROUP == 0 || flags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("hidden detached child lost a flag: %#x", flags)
	}
}
