package agentexec

import (
	"fmt"
	"os/exec"
	"strconv"

	"golang.org/x/sys/windows"
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

// StopProcessTree terminates a PTY-owned process and its descendants. PTY
// children are started by the terminal library, outside the agent's Job Object.
func StopProcessTree(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("process has no valid ID")
	}
	if err := stopProcessTreeCommand(pid).Run(); err != nil {
		return fmt.Errorf("terminate process tree %d: %w", pid, err)
	}
	return nil
}

func stopProcessTreeCommand(pid int) *exec.Cmd {
	return Hidden(exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F"))
}
