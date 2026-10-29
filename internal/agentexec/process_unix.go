//go:build !windows

package agentexec

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// The native client owns a separate foreground process group in the terminal.
// StartControlled starts cmd in its own foreground process group and returns a
// function that restores the caller's terminal.
func StartControlled(cmd *exec.Cmd) (func(), error) {
	fd := int(os.Stdin.Fd())
	group, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	attrs := &syscall.SysProcAttr{Setpgid: true}
	restore := func() {}
	if err == nil {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTTOU)
		attrs.Foreground = true
		attrs.Ctty = fd
		// The shell can change the PTY foreground group as this process starts.
		// An external stty child may then be suspended by SIGTTIN before the
		// provider launches. Capture and restore settings in this process.
		var terminalState *term.State
		if group == syscall.Getpgrp() {
			terminalState, _ = term.GetState(fd)
		}
		restore = func() {
			_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, group)
			if terminalState != nil {
				_ = term.Restore(fd, terminalState)
			}
			signal.Stop(signals)
		}
	}
	cmd.SysProcAttr = attrs
	if err := cmd.Start(); err != nil {
		restore()
		return func() {}, err
	}
	return restore, nil
}

// StartDetached starts a child the daemon supervises itself rather than one a
// terminal owns. Setsid makes it a session and process group leader, which is
// all StopControlled needs to signal the whole tree; the Windows side has to
// build a Job Object for the same guarantee.
func StartDetached(cmd *exec.Cmd) (func(), error) {
	cmd.SysProcAttr = DetachedSession()
	return func() {}, cmd.Start()
}

// StopControlled signals the whole process group cmd owns.
func StopControlled(cmd *exec.Cmd, force bool) {
	sig := syscall.SIGINT
	if force {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}

// detachedSession runs a child in its own session, with no controlling
// terminal. Interactive shells started by tests otherwise contend for the
// caller's terminal and can block until they are signalled.
// DetachedSession runs a child in its own session, with no controlling terminal.
func DetachedSession() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// Hidden is the Windows console-window suppression described in its Windows
// counterpart. A POSIX child inherits the terminal it was started from and opens
// no window of its own, so there is nothing to suppress here.
func Hidden(cmd *exec.Cmd) *exec.Cmd { return cmd }

// terminationSignals keeps the supervisor alive long enough to stop its child
// and report the actual exit when its owning terminal disappears.
func terminationSignals() (<-chan os.Signal, func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	return signals, func() { signal.Stop(signals) }
}
