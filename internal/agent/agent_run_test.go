package agent

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"tasks/internal/agentexec"
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
	case <-d.queue.runs["run"].exited:
		t.Fatal("unauthorized exit acknowledged")
	default:
	}
	req.Header.Set("Authorization", "Bearer "+d.queue.runs["run"].token)
	rec = httptest.NewRecorder()
	d.handleRunControl(rec, req)
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	select {
	case <-d.queue.runs["run"].exited:
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
		done <- agentexec.Run([]string{"--url", server.URL + "/control/runs/run", "--token", d.queue.runs["run"].token, "--command", command})
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
	d.queue.mu.Lock()
	d.queue.runs["run"].canceled = true
	d.queue.mu.Unlock()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled command succeeded")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("command did not stop")
	}
	select {
	case <-d.queue.runs["run"].exited:
	default:
		t.Fatal("exit not confirmed")
	}
}

// A stop overrides the exit a supervised command reports, except that a
// discussion the user ends completes, and a discussion that fails on its own
// still fails.
func TestSupervisedRunExitStatus(t *testing.T) {
	for _, tc := range []struct {
		name, skill, reported string
		canceled              bool
		want                  string
	}{
		{"stopped skill run", "implement", "failed", true, "canceled"},
		{"stopped discussion", "discuss", "failed", true, "completed"},
		{"discussion exiting non-zero", "discuss", "failed", false, "failed"},
		{"discussion exiting zero", "discuss", "completed", false, "completed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &agentDaemon{}
			if _, err := d.wrapRun("", "run", ""); err != nil {
				t.Fatal(err)
			}
			run := d.queue.runs["run"]
			run.desktop.Skill = tc.skill
			run.canceled = tc.canceled
			req := httptest.NewRequest(http.MethodPost, "/control/runs/run", strings.NewReader(`{"status":"`+tc.reported+`"}`))
			req.Header.Set("Authorization", "Bearer "+run.token)
			rec := httptest.NewRecorder()
			d.handleRunControl(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("control returned %d: %s", rec.Code, rec.Body.String())
			}
			if run.desktop.Status != tc.want {
				t.Fatalf("status = %q, want %q", run.desktop.Status, tc.want)
			}
		})
	}
}

func TestStoppedNote(t *testing.T) {
	if got := stoppedNote("discuss", true); got != "Discussion ended after its local terminal closed" {
		t.Fatalf("discussion note = %q", got)
	}
	if got := stoppedNote("clarify", false); got != "Execution canceled" {
		t.Fatalf("skill note = %q", got)
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
	restore, err := agentexec.StartControlled(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	agentexec.StopControlled(cmd, true)
	if err := cmd.Wait(); err == nil {
		t.Fatal("expected killed child")
	}
}

func TestSupervisedRunRetriesExitConfirmation(t *testing.T) {
	d := &agentDaemon{}
	if _, err := d.wrapRun("", "run", ""); err != nil {
		t.Fatal(err)
	}
	var reports atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && reports.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		d.handleRunControl(w, r)
	}))
	defer server.Close()
	if err := agentexec.Run([]string{"--url", server.URL + "/control/runs/run", "--token", d.queue.runs["run"].token, "--command", "exit 0"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-d.queue.runs["run"].exited:
	default:
		t.Fatal("exit was not confirmed after a transient report failure")
	}
	if reports.Load() != 2 {
		t.Fatalf("got %d exit reports, want 2", reports.Load())
	}
}
