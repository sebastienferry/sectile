package agent

import (
	"bufio"
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

// headlessStopGrace bounds the two waits a stop must never hang on: how long the
// output pipe is given to close after a forced stop, and how long reaping the
// shell is given after that. A descendant that outlives the kill holds the write
// end of the pipe, so its EOF may never come, and a run that waits for it stays
// "running" with no way to stop it. Both grace periods fit inside the twelve
// seconds a stop request waits for confirmation, so the stop resolves as
// canceled rather than as an unconfirmed exit.
const headlessStopGrace = 2 * time.Second

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
	output, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// Both streams go into the same pipe: a provider CLI writes diagnostics to
	// either one, and the record is more useful in the order things were printed
	// than split in two.
	cmd.Stderr = cmd.Stdout

	// Whether this run is traced is read off the command line that is about to
	// run: it is the line that decides what its standard output will be.
	run := d.registerHeadlessRun(taskRef, payload, config, workDir, branch, provider, model, commandReadsReasoning(fullLine))
	// StartDetached owns the child: its own session or process group, so a stop
	// reaches it rather than the daemon, no window on Windows, and a handle on
	// the whole tree. A stop that only reached this shell would leave the
	// provider CLI running behind it, still holding the pipe read below open.
	release, err := agentexec.StartDetached(cmd)
	if err != nil {
		release()
		d.finishHeadlessRun(taskRef, payload.RunID, run, "failed", err.Error())
		return err
	}

	go func() {
		defer release()
		d.superviseHeadlessRun(taskRef, payload.RunID, run, cmd, output)
	}()
	return nil
}

// registerHeadlessRun records the run the way the PTY path does, minus the
// session: an autonomous run has no terminal to attach to, and the desktop must
// not present it as an execution whose console is missing.
//
// traced says the engine was asked for its reasoning stream, so the run gets the
// trace the desktop attaches to in place of a console.
func (d *agentDaemon) registerHeadlessRun(taskRef string, payload agentconfig.Dispatch, config agentconfig.Config, workDir, branch, provider, model string, traced bool) *controlledRun {
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
	if traced && run.trace == nil {
		run.trace = newRunTrace()
		// The pane is attached to before the engine has said anything. One line
		// says what it is looking at, so an empty trace reads as a run starting
		// rather than as a console that failed to open.
		run.trace.write(traceLine(traceDim + "Autonomous execution · read-only trace" + traceReset))
	}
	run.desktop = desktopRun{
		CreatedAt: run.desktop.CreatedAt, Prompt: run.desktop.Prompt,
		ID: payload.RunID, TaskID: taskRef, TaskKey: payload.TaskKey, ProjectID: config.ProjectID,
		Skill: payload.SkillID, Directory: workDir, Branch: branch, Status: "running",
		Provider: provider, Model: model,
		Headless: true, Trace: run.trace != nil,
	}
	return run
}

// superviseHeadlessRun drains the process output, posts it as it comes, and
// finishes the run on exit. A cancelled run is stopped the same way an
// interactive one is, so the desktop's stop button keeps working.
func (d *agentDaemon) superviseHeadlessRun(taskRef, runID string, run *controlledRun, cmd *exec.Cmd, output io.Reader) {
	var mu sync.Mutex
	var pending bytes.Buffer

	// The run's trace, read once under the queue lock like every other field of
	// the run. Nil means the engine was not asked for its reasoning, and this
	// run's output is read byte by byte as it always was.
	d.queue.mu.Lock()
	var trace *runTrace
	if run != nil {
		trace = run.trace
	}
	d.queue.mu.Unlock()

	flush := func() {
		mu.Lock()
		chunk := pending.String()
		pending.Reset()
		mu.Unlock()
		if chunk != "" {
			d.postRunOutput(taskRef, runID, chunk)
		}
	}

	record := func(text string) {
		mu.Lock()
		pending.WriteString(text)
		mu.Unlock()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if trace != nil {
			readTracedOutput(trace, output, record)
			return
		}
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
	// abandoned fires once a forced stop has been issued and the pipe still has
	// not closed. Waiting past that point is waiting on something that escaped
	// the kill, and the tail of the output is worth less than a stop that lands.
	var abandoned <-chan time.Time
	for draining {
		select {
		case <-done:
			draining = false
		case <-abandoned:
			draining = false
		case <-ticker.C:
			flush()
			if d.queue.canceled(run) && cmd.Process != nil {
				// Escalate on the second pass, as the interactive supervisor
				// does: a CLI that traps the interrupt and keeps working must
				// not turn a stop request into a run that never ends.
				force := signalled
				agentexec.StopControlled(cmd, force)
				signalled = true
				if force && abandoned == nil {
					abandoned = time.After(headlessStopGrace)
				}
			}
		}
	}

	// Wait only once the pipe is drained. Wait closes the read end as soon as it
	// sees the process exit, so calling it alongside the reader would truncate
	// the tail of the output, which is exactly where a failure explains itself.
	// It is bounded for the same reason the drain is: the run has to end even
	// when the shell itself survived the stop.
	err := waitForExit(cmd, headlessStopGrace)
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

// readTracedOutput reads the output of a run whose engine was asked for its
// reasoning stream, and sends each line where it belongs: the events to the
// trace the desktop watches, and to the task activity exactly what the activity
// received before the stream existed.
//
// Lines are read with a bufio.Reader and not a bufio.Scanner: one object of the
// stream can carry a whole tool result and go past the scanner's 64 KB token
// ceiling, which would end the reading for the size of what the engine said
// rather than for anything wrong.
func readTracedOutput(trace *runTrace, output io.Reader, record func(string)) {
	reader := bufio.NewReader(output)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			routeTracedLine(trace, strings.TrimRight(line, "\r\n"), record)
		}
		if err != nil {
			return
		}
	}
}

// routeTracedLine places one line of a traced run.
//
// The result message carries the answer, which is what the engine printed on its
// own before it was asked for a stream, so that is what the activity receives.
// Anything the parser showed belongs to the trace alone. What is left is the
// interesting case: standard error shares this pipe, so a missing binary, a
// crash or a shell error arrives here as plain text, and that is where a failed
// run explains itself — it goes to the activity untouched.
func routeTracedLine(trace *runTrace, line string, record func(string)) {
	events, result, done := runner.ParseReasoningLine(line)
	for _, event := range events {
		trace.publish(event)
	}
	switch {
	case done:
		if strings.TrimSpace(result) != "" {
			record(result + "\n")
		}
	case len(events) > 0 || isStreamFrame(line):
		// Shown in the trace, or protocol the reader had nothing to show for:
		// either way it is not the activity's business.
	case strings.TrimSpace(line) == "":
	default:
		record(line + "\n")
	}
}

// isStreamFrame says whether a line the parser showed nothing for is still part
// of the engine's protocol — the session banner, a user message carrying a tool
// result, the tail of a stream cut mid-object — as opposed to a diagnostic
// printed beside it. Only the second kind is worth recording on the task.
func isStreamFrame(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"type":`)
}

// waitForExit reaps the child, giving up after grace. cmd.Wait is left running
// in its own goroutine when it does: it owns the pipe teardown, and the reader
// it unblocks is what lets that goroutine finish on its own.
func waitForExit(cmd *exec.Cmd, grace time.Duration) error {
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err := <-waited:
		return err
	case <-time.After(grace):
		return fmt.Errorf("the process did not exit within %s of the stop request", grace)
	}
}

func (d *agentDaemon) finishHeadlessRun(taskRef, runID string, run *controlledRun, status, note string) {
	d.queue.mu.Lock()
	var trace *runTrace
	if run != nil {
		run.desktop.Status = status
		run.once.Do(func() { close(run.exited) })
		// Read under the lock, like every other field of the run, and closed
		// outside it: closing wakes the watchers, and they have no business
		// waiting on the queue.
		trace = run.trace
	}
	d.queue.mu.Unlock()
	// The run is over: a watcher sees the trace end rather than a socket left
	// open on a process that exited. What it showed stays readable.
	trace.close()
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
