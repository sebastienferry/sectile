//go:build !windows

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// The native client owns a separate foreground process group in the terminal.
func startControlledCommand(cmd *exec.Cmd) (func(), error) {
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

func stopControlledCommand(cmd *exec.Cmd, force bool) {
	sig := syscall.SIGINT
	if force {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}
