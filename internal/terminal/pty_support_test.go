package terminal

import (
	"runtime"
	"testing"
)

// requirePTY skips a test on a host without a pseudo-terminal. The PTY-bound suites are the
// agent's Unix console; on Windows the agent runs a task in the host terminal instead, so a
// failure here would report a platform limitation as a regression.
func requirePTY(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("no pseudo-terminal on Windows; tasks run in the host terminal there")
	}
}
