package main

import (
	"os/exec"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

// controlledJobs keeps the Job Object handle of each supervised child, so a forced stop can
// terminate the whole tree rather than just the process the agent happens to know about.
var controlledJobs sync.Map

// startControlledCommand gives the child its own process group inside a Job Object. The group
// makes a polite stop deliverable as a console break; the job makes a forced stop reach every
// descendant, which is what the POSIX side gets from signalling a process group. The job is
// created without KILL_ON_JOB_CLOSE, so a restarting agent does not take a running skill with it.
func startControlledCommand(cmd *exec.Cmd) (func(), error) {
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

// stopControlledCommand interrupts the console group, or terminates the whole job when forced.
func stopControlledCommand(cmd *exec.Cmd, force bool) {
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

// Windows has no controlling terminal to detach from.
func detachedSession() *syscall.SysProcAttr {
	return nil
}
