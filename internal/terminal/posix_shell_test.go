package terminal

import (
	"runtime"
	"testing"
)

// requirePosixShell skips a test that speaks POSIX shell to the session.
//
// The pseudo-terminal itself works everywhere, and TestSessionRunsAnInjectedLine proves it
// on this host. What does not travel is RunCommandInSession's marker protocol: it wraps the
// command in printf calls and reads $? for the exit code, which a Windows shell does not
// understand. The helper has no caller outside these tests; whoever gives it one on Windows
// has to render the markers for the target shell first.
func requirePosixShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("RunCommandInSession speaks POSIX shell; no Windows renderer for its markers yet")
	}
}
