//go:build windows

package agentexec

import (
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
