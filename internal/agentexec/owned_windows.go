//go:build windows

package agentexec

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// StartOwned assigns a suspended provider to a kill-on-close job before it can
// spawn any tools. Only the agent owns the job handle, so its death terminates
// the entire tree. Unlike legacy terminals, conversations cannot outlive Go.
func StartOwned(cmd *exec.Cmd) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return func() {}, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return func() {}, err
	}
	ownedAttributes(cmd)
	if err = cmd.Start(); err != nil {
		_ = windows.CloseHandle(job)
		return func() {}, err
	}
	pid := cmd.Process.Pid
	var once sync.Once
	release := func() { once.Do(func() { controlledJobs.Delete(pid); _ = windows.CloseHandle(job) }) }
	fail := func(err error) (func(), error) {
		_ = cmd.Process.Kill()
		release()
		_ = cmd.Wait()
		return func() {}, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fail(err)
	}
	err = windows.AssignProcessToJobObject(job, handle)
	_ = windows.CloseHandle(handle)
	if err != nil {
		return fail(err)
	}
	controlledJobs.Store(pid, job)
	// The process has not executed user code, so it owns only its primary thread.
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fail(err)
	}
	defer windows.CloseHandle(snapshot)
	thread := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &thread); err == nil; err = windows.Thread32Next(snapshot, &thread) {
		if thread.OwnerProcessID != uint32(pid) {
			continue
		}
		h, openErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, thread.ThreadID)
		if openErr != nil {
			return fail(openErr)
		}
		_, resumeErr := windows.ResumeThread(h)
		_ = windows.CloseHandle(h)
		if resumeErr != nil {
			return fail(resumeErr)
		}
		return release, nil
	}
	return fail(fmt.Errorf("provider primary thread unavailable: %w", err))
}

func ownedAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = DetachedSession()
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	Hidden(cmd)
}

// RunProviderGuard is Unix-only; Windows enforces ownership in the kernel.
func RunProviderGuard([]string) int { return 2 }

// OwnedStdinPipe needs no protocol adapter on Windows; the job owns lifetime.
func OwnedStdinPipe(cmd *exec.Cmd) (io.WriteCloser, error) { return cmd.StdinPipe() }
