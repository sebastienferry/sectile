//go:build !windows

package agentexec

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// StopTree freezes the supervised provider before discovering descendants.
// Tools can create their own sessions, so signalling only its group misses them.
// Frozen descendants cannot fork while the tree is collected and terminated.
func StopTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return fmt.Errorf("provider has no process")
	}
	return StopProcessTree(cmd.Process.Pid)
}

// StopProcessTree terminates a process and every descendant, including tools
// that leave the process group their parent started in.
func StopProcessTree(root int) error {
	if root <= 0 {
		return fmt.Errorf("process has no valid ID")
	}
	if err := syscall.Kill(root, syscall.SIGSTOP); err != nil {
		return err
	}
	frozen := map[int]bool{root: true}
	order := []int{root}
	defer func() {
		for _, pid := range order {
			_ = syscall.Kill(pid, syscall.SIGCONT)
		}
	}()
	for pass := 0; pass < 64; pass++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		output, err := Hidden(exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=")).Output()
		cancel()
		if err != nil {
			return fmt.Errorf("inspect provider descendants: %w", err)
		}
		changed := false
		for _, line := range strings.Split(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			pid, e1 := strconv.Atoi(fields[0])
			parent, e2 := strconv.Atoi(fields[1])
			if e1 != nil || e2 != nil || !frozen[parent] || frozen[pid] {
				continue
			}
			if err := syscall.Kill(pid, syscall.SIGSTOP); err == nil {
				frozen[pid] = true
				order = append(order, pid)
				changed = true
			}
		}
		if !changed {
			for i := len(order) - 1; i >= 0; i-- {
				if err := syscall.Kill(order[i], syscall.SIGKILL); err != nil && err != syscall.ESRCH {
					return err
				}
			}
			// Do not signal PIDs again after killing them: the OS can reuse them.
			order = nil
			return nil
		}
	}
	return fmt.Errorf("provider process tree did not stabilize")
}
