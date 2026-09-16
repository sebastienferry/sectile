package agentexec

import (
	"fmt"
	"os/exec"
	"syscall"
)

// StartControlled starts cmd in its own foreground process group and returns a
// function that restores the caller's terminal.
func StartControlled(cmd *exec.Cmd) (func(), error) {
	return func() {}, fmt.Errorf("supervised native execution is not supported on Windows")
}

// StopControlled signals the whole process group cmd owns.
func StopControlled(cmd *exec.Cmd, force bool) {}

// Windows has no controlling terminal to detach from.
// DetachedSession runs a child in its own session, with no controlling terminal.
func DetachedSession() *syscall.SysProcAttr {
	return nil
}
