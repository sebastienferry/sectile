//go:build !windows

package agent

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSupervisorSignalsConfirmExit(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGHUP, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			d := &agentDaemon{}
			if _, err := d.wrapRun("", "run", ""); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(d.handleRunControl))
			defer server.Close()
			marker := filepath.Join(t.TempDir(), "child-pid")
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "agent-exec", "--url", server.URL+"/control/runs/run", "--token", d.queue.runs["run"].token, "--command", "echo $$ > "+quoteShell(marker)+"; exec sleep 60")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			defer cmd.Process.Kill()
			deadline := time.Now().Add(5 * time.Second)
			var pid int
			for time.Now().Before(deadline) {
				data, _ := os.ReadFile(marker)
				pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
				if pid > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid == 0 {
				t.Fatal("supervised child never started")
			}
			defer syscall.Kill(-pid, syscall.SIGKILL)
			d.queue.read("run", func(run *controlledRun) { run.canceled = true })
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				t.Fatal("supervisor did not stop")
			}
			select {
			case <-d.queue.runs["run"].exited:
			default:
				t.Fatal("supervisor disappeared without confirming process exit")
			}
			if err := syscall.Kill(pid, 0); err == nil {
				t.Fatal("supervised child survived termination")
			}
		})
	}
}
