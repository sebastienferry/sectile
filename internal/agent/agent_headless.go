package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"tasks/internal/agentexec"
	"tasks/internal/agenthttp"
	"tasks/internal/runner"
	"time"

	"tasks/internal/agentconfig"
)

// headlessFlushInterval is how often captured output is posted while the CLI is
// still running. A headless run can take many minutes, and a user watching the
// activity should not have to wait for the exit to see anything.
const headlessFlushInterval = 3 * time.Second

// startHeadlessRun launches the provider CLI with no terminal at all: no PTY
// session, no foreground process group, no window. Output is read from the
// process pipes and posted onto the run activity, which is the only channel an
// autonomous run has, and the run is finished when the process exits.
//
// The run is still registered like any other, so the desktop lists it, can
// select it, and can stop it. What it cannot do is type into it.
func (d *agentDaemon) startHeadlessRun(taskRef string, payload agentconfig.Dispatch, config agentconfig.Config, workDir, branch string, envVars map[string]string, fullLine, provider, model string) error {
	cmd := exec.Command("bash", "-lc", fullLine)
	cmd.Dir = workDir
	cmd.Stdin = nil
	cmd.Env = commandEnv(envVars)
	// Its own session, so it is also its own process group leader. Cancellation
	// signals the group (stopControlledCommand kills -pid), which reaches nothing
	// when the child shares the daemon's group: the stop button would look like
	// it worked and the run would keep going. Detaching also matches what a
	// headless run is, a process with no controlling terminal at all.
	cmd.SysProcAttr = agentexec.DetachedSession()

	output, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// Both streams go into the same pipe: a provider CLI writes diagnostics to
	// either one, and the record is more useful in the order things were printed
	// than split in two.
	cmd.Stderr = cmd.Stdout

	run := d.registerHeadlessRun(taskRef, payload, config, workDir, branch, provider, model)
	if err := cmd.Start(); err != nil {
		d.finishHeadlessRun(taskRef, payload.RunID, run, "failed", err.Error())
		return err
	}

	go d.superviseHeadlessRun(taskRef, payload.RunID, run, cmd, output)
	return nil
}

// registerHeadlessRun records the run the way the PTY path does, minus the
// session: an autonomous run has no terminal to attach to, and the desktop must
// not present it as an execution whose console is missing.
func (d *agentDaemon) registerHeadlessRun(taskRef string, payload agentconfig.Dispatch, config agentconfig.Config, workDir, branch, provider, model string) *controlledRun {
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if d.queue.runs == nil {
		d.queue.runs = make(map[string]*controlledRun)
	}
	run := d.queue.runs[payload.RunID]
	if run == nil {
		run = &controlledRun{taskID: taskRef, exited: make(chan struct{})}
		d.queue.runs[payload.RunID] = run
	}
	run.taskID = taskRef
	run.desktop = desktopRun{
		CreatedAt: run.desktop.CreatedAt, Prompt: run.desktop.Prompt,
		ID: payload.RunID, TaskID: taskRef, TaskKey: payload.TaskKey, ProjectID: config.ProjectID,
		Skill: payload.SkillID, Directory: workDir, Branch: branch, Status: "running",
		Provider: provider, Model: model,
		Headless: true,
	}
	return run
}

// superviseHeadlessRun drains the process output, posts it as it comes, and
// finishes the run on exit. A cancelled run is stopped the same way an
// interactive one is, so the desktop's stop button keeps working.
func (d *agentDaemon) superviseHeadlessRun(taskRef, runID string, run *controlledRun, cmd *exec.Cmd, output io.Reader) {
	var mu sync.Mutex
	var pending bytes.Buffer

	flush := func() {
		mu.Lock()
		chunk := pending.String()
		pending.Reset()
		mu.Unlock()
		if chunk != "" {
			d.postRunOutput(taskRef, runID, chunk)
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := output.Read(buf)
			if n > 0 {
				mu.Lock()
				pending.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(headlessFlushInterval)
	defer ticker.Stop()
	signalled := false
	draining := true
	for draining {
		select {
		case <-done:
			draining = false
		case <-ticker.C:
			flush()
			if d.queue.canceled(run) && cmd.Process != nil {
				// Escalate on the second pass, as the interactive supervisor
				// does: a CLI that traps the interrupt and keeps working must
				// not turn a stop request into a run that never ends.
				agentexec.StopControlled(cmd, signalled)
				signalled = true
			}
		}
	}

	// Wait only once the pipe is drained. Wait closes the read end as soon as it
	// sees the process exit, so calling it alongside the reader would truncate
	// the tail of the output, which is exactly where a failure explains itself.
	err := cmd.Wait()
	flush()
	status, note := "completed", "Headless run finished"
	if err != nil {
		status, note = "failed", err.Error()
	}
	if d.queue.canceled(run) {
		status, note = "canceled", "Headless run canceled"
	}
	d.finishHeadlessRun(taskRef, runID, run, status, note)
}

func (d *agentDaemon) finishHeadlessRun(taskRef, runID string, run *controlledRun, status, note string) {
	d.queue.mu.Lock()
	if run != nil {
		run.desktop.Status = status
		run.once.Do(func() { close(run.exited) })
	}
	d.queue.mu.Unlock()
	if runID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := d.finishDesktopRun(ctx, taskRef, runID, status, note); err != nil {
		d.postRunOutput(taskRef, runID, "\n[agent] could not report the run result: "+err.Error()+"\n")
	}
}

// postRunOutput hands one chunk to the server. Failures are dropped on purpose:
// losing a slice of log must never take down a run that is otherwise working.
func (d *agentDaemon) postRunOutput(taskRef, runID, chunk string) {
	if strings.TrimSpace(chunk) == "" || runID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body := mustJSON(map[string]string{"taskId": taskRef, "runId": runID, "output": chunk})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.link.serverURL+"/api/v1/agent/run-output", strings.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// commandEnv turns the run environment into the form exec expects, inheriting
// the agent's own environment so the provider CLI finds its PATH and credentials.
func commandEnv(envVars map[string]string) []string {
	env := runner.SanitizedEnviron()
	for key, value := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}
