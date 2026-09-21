package agentexec

import (
	"os/exec"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

// controlledJobs keeps the Job Object handle of each supervised child, so a forced stop can
// terminate the whole tree rather than just the process the agent happens to know about.
var controlledJobs sync.Map

// StartControlled gives the child its own process group inside a Job Object. The group
// makes a polite stop deliverable as a console break; the job makes a forced stop reach every
// descendant, which is what the POSIX side gets from signalling a process group. The job is
// created without KILL_ON_JOB_CLOSE, so a restarting agent does not take a running skill with it.
func StartControlled(cmd *exec.Cmd) (func(), error) {
	job, jobErr := windows.CreateJobObject(nil, nil)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		if jobErr == nil {
			_ = windows.CloseHandle(job)
		}
		return func() {}, err
	}
	if jobErr != nil {
		// The child runs regardless; losing the job only costs us tree termination.
		return func() {}, nil
	}
	pid := cmd.Process.Pid
	release := func() {
		controlledJobs.Delete(pid)
		_ = windows.CloseHandle(job)
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		release()
		return func() {}, nil
	}
	err = windows.AssignProcessToJobObject(job, handle)
	_ = windows.CloseHandle(handle)
	if err != nil {
		release()
		return func() {}, nil
	}
	controlledJobs.Store(pid, job)
	return release, nil
}

// StopControlled interrupts the console group, or terminates the whole job when forced.
func StopControlled(cmd *exec.Cmd, force bool) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	if !force {
		// Deliverable only when the agent shares a console with the child; the job is the fallback.
		if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(pid)); err == nil {
			return
		}
	}
	if value, ok := controlledJobs.Load(pid); ok {
		if job, valid := value.(windows.Handle); valid {
			_ = windows.TerminateJobObject(job, 1)
			return
		}
	}
	_ = cmd.Process.Kill()
}

// DetachedSession runs a child in its own session. Windows has no controlling terminal to
// detach from, but the process group still matters: StopControlled addresses a group, and a
// child left in the agent's own group would take the whole console down with it instead of
// stopping alone.
func DetachedSession() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

// Hidden keeps a command the agent runs for itself from opening a console window.
// The desktop starts the agent detached, which on Windows means DETACHED_PROCESS:
// the agent owns no console, so Windows allocates a fresh one — a visible, focus
// stealing window — for every console child it starts. CREATE_NO_WINDOW says the
// child needs no console of its own, which is true of anything whose output the
// agent reads itself. The flag is OR-ed in, so a caller that already asked for its
// own process group keeps it.
func Hidden(cmd *exec.Cmd) *exec.Cmd {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
	return cmd
}
