package main

import (
	"fmt"
	"os/exec"
)

func startControlledCommand(cmd *exec.Cmd) (func(), error) {
	return func() {}, fmt.Errorf("supervised native execution is not supported on Windows")
}
func stopControlledCommand(cmd *exec.Cmd, force bool) {}
