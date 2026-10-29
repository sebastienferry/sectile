package agentexec

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os/exec"
)

// StopTree terminates the provider's job, including tools with separate consoles.
func StopTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return fmt.Errorf("provider has no process")
	}
	value, ok := controlledJobs.Load(cmd.Process.Pid)
	if !ok {
		return fmt.Errorf("provider process job is unavailable")
	}
	job, ok := value.(windows.Handle)
	if !ok {
		return fmt.Errorf("provider process job is invalid")
	}
	return windows.TerminateJobObject(job, 1)
}
