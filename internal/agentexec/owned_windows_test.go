//go:build windows

package agentexec

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"testing"
)

func TestOwnedProviderIsHiddenAndSuspendedBeforeJobAssignment(t *testing.T) {
	cmd := Hidden(exec.Command("cmd", "/c", "exit"))
	ownedAttributes(cmd)
	flags := cmd.SysProcAttr.CreationFlags
	required := uint32(windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED | windows.CREATE_NEW_PROCESS_GROUP)
	if flags&required != required {
		t.Fatalf("owned provider lost startup flags: %#x", flags)
	}
}
