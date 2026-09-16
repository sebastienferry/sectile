package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

func startControlledCommand(cmd *exec.Cmd) (func(), error) {
	return func() {}, fmt.Errorf("supervised native execution is not supported on Windows")
}
func stopControlledCommand(cmd *exec.Cmd, force bool) {}

// Windows has no controlling terminal to detach from.
func detachedSession() *syscall.SysProcAttr {
	return nil
}
