//go:build windows

package agentexec

import (
	"io"
	"os/exec"
	"testing"
	"time"

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

// A forced stop has to reach the whole tree. The daemon reads a headless run
// through a pipe every descendant inherits, so killing the direct child while a
// grandchild survives leaves that pipe open for good: the run never reports an
// exit, and the desktop shows it running with no way to stop it. Windows has no
// process group to signal for this, only the Job Object StartDetached creates.
func TestStartDetachedForcedStopClosesTheOutputPipe(t *testing.T) {
	// The inner cmd is the grandchild; ping keeps it alive well past the stop.
	cmd := exec.Command("cmd", "/c", "cmd /c ping -n 30 127.0.0.1")
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	release, err := StartDetached(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	closed := make(chan struct{})
	go func() { defer close(closed); _, _ = io.Copy(io.Discard, output) }()
	// Let the grandchild start and inherit the pipe before the stop lands.
	time.Sleep(2 * time.Second)
	StopControlled(cmd, true)
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("the output pipe stayed open after a forced stop: a descendant survived it")
	}
	_ = cmd.Wait()
}
