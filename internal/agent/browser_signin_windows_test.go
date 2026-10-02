//go:build windows

package agent

import (
	"testing"

	"golang.org/x/sys/windows"
)

// The agent owns no console when the desktop starts it: opening the browser
// must not hand the user a stray console window as well.
func TestBrowserSignInOpensTheBrowserWithoutAConsoleWindow(t *testing.T) {
	cmd := browserCommand("windows", "https://sectile.example.test/auth/workstation")
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("browser opener would open a console window: %+v", cmd.SysProcAttr)
	}
}
