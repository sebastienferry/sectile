package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"tasks/internal/agentconfig"
)

// headlessOutputCap bounds what one run records. A long skill run prints tens of
// thousands of lines; the activity is a diagnosis aid, not a log store, so the
// capture keeps the tail, which is where a failure is explained.
const headlessOutputCap = 64 << 10

// runEnv is the environment every launch shares, whether it runs in a terminal
// session or headless.
func (d *agentDaemon) runEnv(taskRef, workDir, branch string, payload agentconfig.Dispatch) map[string]string {
	env := map[string]string{
		"SECTILE_TASK_KEY":      payload.TaskKey,
		"SECTILE_TASK_BRANCH":   branch,
		"SECTILE_TASK_WORKTREE": workDir,
		"SECTILE_TASK_ID":       taskRef,
		"SECTILE_RUN_ID":        payload.RunID,
		"SECTILE_REMOTE_MODE":   "true",
		"SECTILE_AGENT_URL":     d.agentURL,
		"SECTILE_SERVER_URL":    d.serverURL,
		"SECTILE_AGENT_TOKEN":   d.loopbackToken,
	}
	if payload.ProjectID != "" {
		env["SECTILE_PROJECT_ID"] = payload.ProjectID
	}
	return env
}

// tailBuffer keeps at most cap bytes of what is written to it, dropping from the
// front. It is the size bound on a headless run's captured output.
type tailBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	cap int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf.Write(p)
	if excess := t.buf.Len() - t.cap; excess > 0 {
		t.buf.Next(excess)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String()
}

// runHeadless supervises a non-interactive launch. The process is started
// directly, not injected into a PTY session, so nothing opens a window and
// nothing waits for a human. The run is finished with the process, and its
// captured output is the note, which is the only place a headless failure is
// visible.
func (d *agentDaemon) runHeadless(taskRef string, payload agentconfig.Dispatch, workDir, branch, fullLine string) error {
	run, err := d.registerHeadlessRun(taskRef, payload, workDir, branch)
	if err != nil {
		return err
	}

	cmd := exec.Command("bash", "-lc", fullLine)
	cmd.Dir = workDir
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	for key, value := range d.runEnv(taskRef, workDir, branch, payload) {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output := &tailBuffer{cap: headlessOutputCap}
	cmd.Stdout, cmd.Stderr = output, output
	// Its own session: no controlling terminal to steal, and a process group the
	// cancel path can signal as a whole, exactly as the terminal supervisor does.
	cmd.SysProcAttr = detachedSession()
	if err := cmd.Start(); err != nil {
		return err
	}
	log.Printf("⚡ [Agent] Launching skill command headless (run: %s, workdir: %s)", payload.RunID, workDir)

	go d.awaitHeadless(taskRef, payload.RunID, run, cmd, output)
	return nil
}

// registerHeadlessRun publishes the run as running before the process starts, so
// a headless launch is visible on the board exactly like a terminal one.
func (d *agentDaemon) registerHeadlessRun(taskRef string, payload agentconfig.Dispatch, workDir, branch string) (*controlledRun, error) {
	d.runsMu.Lock()
	defer d.runsMu.Unlock()
	if d.shuttingDown {
		return nil, fmt.Errorf("agent is restarting")
	}
	run := d.runs[payload.RunID]
	if run == nil {
		return nil, fmt.Errorf("execution is not registered")
	}
	run.taskID = taskRef
	run.desktop = desktopRun{CreatedAt: run.desktop.CreatedAt, Prompt: run.desktop.Prompt, ID: payload.RunID, TaskID: taskRef,
		TaskKey: payload.TaskKey, ProjectID: payload.ProjectID, Skill: payload.SkillID, Directory: workDir,
		Branch: branch, Status: "running", StartedAt: time.Now().UTC()}
	return run, nil
}

// awaitHeadless waits for the process, then closes the run with its real outcome.
// A cancel request kills the process group the same way the terminal supervisor
// does, so stopping a headless run is not a special case.
func (d *agentDaemon) awaitHeadless(taskRef, runID string, run *controlledRun, cmd *exec.Cmd, output *tailBuffer) {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	status := "completed"
	var waitErr error
	for waiting := true; waiting; {
		select {
		case waitErr = <-done:
			waiting = false
		case <-ticker.C:
			d.runsMu.Lock()
			canceled := run.canceled
			d.runsMu.Unlock()
			if !canceled {
				continue
			}
			stopControlledCommand(cmd, false)
			select {
			case waitErr = <-done:
			case <-time.After(2 * time.Second):
				stopControlledCommand(cmd, true)
				waitErr = <-done
			}
			status = "canceled"
			waiting = false
		}
	}
	if status != "canceled" && waitErr != nil {
		status = "failed"
	}

	d.runsMu.Lock()
	if run.canceled {
		status = "canceled"
	}
	run.desktop.Status = status
	run.once.Do(func() { close(run.exited) })
	d.runsMu.Unlock()

	note := output.String()
	if waitErr != nil {
		note = waitErr.Error() + "\n" + note
	}
	if note == "" {
		note = "Headless run exited without output"
	}
	_ = d.finishDesktopRun(context.Background(), taskRef, runID, status, note)
}
