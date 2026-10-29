//go:build darwin

package companion

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tasks/internal/agentexec"
)

func syncDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func managedListener(port int, expected string) (int, error) {
	if !filepath.IsAbs(expected) {
		return 0, errors.New("legacy executable identity is required")
	}
	raw, err := agentexec.Hidden(exec.Command("/usr/sbin/lsof", "-nP", "-t", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN")).Output()
	if err != nil {
		return 0, err
	}
	ids := strings.Fields(string(raw))
	if len(ids) != 1 {
		return 0, errors.New("ambiguous source listener")
	}
	pid, err := strconv.Atoi(ids[0])
	if err != nil {
		return 0, err
	}
	if err = checkExecutable(pid, expected); err != nil {
		return 0, err
	}
	return pid, nil
}
func checkExecutable(pid int, expected string) error {
	raw, err := agentexec.Hidden(exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "comm=")).Output()
	if err != nil {
		return err
	}
	actual := strings.TrimSpace(string(raw))
	actualPath, err := filepath.EvalSymlinks(actual)
	if err != nil {
		return err
	}
	expectedPath, err := filepath.EvalSymlinks(expected)
	if err != nil {
		return err
	}
	if actualPath != expectedPath {
		return errors.New("source executable does not match the managed desktop binary")
	}
	return nil
}
func stopManaged(ctx context.Context, pid int, expected string) error {
	if err := checkExecutable(pid, expected); err != nil {
		return err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err = process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("source service did not stop before deadline")
		case <-ticker.C:
			if err := process.Signal(syscall.Signal(0)); err != nil {
				return nil
			}
			raw, err := agentexec.Hidden(exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "stat=")).Output()
			if err != nil || strings.HasPrefix(strings.TrimSpace(string(raw)), "Z") {
				return nil
			}
		}
	}
}
