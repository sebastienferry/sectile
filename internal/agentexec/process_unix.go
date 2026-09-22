//go:build !windows

package agentexec

import (
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
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
		snapshot := exec.Command("stty", "-g")
		snapshot.Stdin = os.Stdin
		terminalState, _ := snapshot.Output()
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTTOU)
		attrs.Foreground = true
		attrs.Ctty = fd
		restore = func() {
			_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, group)
			if state := strings.TrimSpace(string(terminalState)); state != "" {
				reset := exec.Command("stty", state)
				reset.Stdin = os.Stdin
				_ = reset.Run()
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
