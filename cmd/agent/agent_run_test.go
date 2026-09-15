package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRunControlAuthentication(t *testing.T) {
	d := &agentDaemon{}
	if _, err := d.wrapRun("task", "run", "true"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/control/runs/run", nil)
	rec := httptest.NewRecorder()
	d.handleRunControl(rec, req)
	if rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	select {
	case <-d.runs["run"].exited:
		t.Fatal("unauthorized exit acknowledged")
	default:
	}
	req.Header.Set("Authorization", "Bearer "+d.runs["run"].token)
	rec = httptest.NewRecorder()
	d.handleRunControl(rec, req)
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	select {
	case <-d.runs["run"].exited:
	default:
		t.Fatal("exit not acknowledged")
	}
}

func TestSupervisedRunCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process groups")
	}
	d := &agentDaemon{}
	if _, err := d.wrapRun("task", "run", ""); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(d.handleRunControl))
	defer server.Close()
	marker := filepath.Join(t.TempDir(), "started")
	command := "touch " + quoteShell(marker) + "; exec sleep 60"
	done := make(chan error, 1)
	go func() {
		done <- runAgentExec([]string{"--url", server.URL + "/control/runs/run", "--token", d.runs["run"].token, "--command", command})
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("command never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	d.runsMu.Lock()
	d.runs["run"].canceled = true
	d.runsMu.Unlock()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled command succeeded")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("command did not stop")
	}
	select {
	case <-d.runs["run"].exited:
	default:
		t.Fatal("exit not confirmed")
	}
}

func TestControlledCommandHasIsolatedGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process groups")
	}
	// startControlledCommand reads os.Stdin to hand the terminal to the child.
	// Pointed at the caller's terminal, that puts `go test` in the background
	// and kills the run; a detached stdin exercises the same process group
	// logic without borrowing anything.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = devNull.Close() }()
	realStdin := os.Stdin
	os.Stdin = devNull
	defer func() { os.Stdin = realStdin }()

	cmd := exec.Command("sleep", "60")
	restore, err := startControlledCommand(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	stopControlledCommand(cmd, true)
	if err := cmd.Wait(); err == nil {
		t.Fatal("expected killed child")
	}
}
