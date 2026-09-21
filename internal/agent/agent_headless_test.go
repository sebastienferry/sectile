package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/agentconfig"
)

// headlessServer collects what the agent posts for a run: the output chunks and
// the finish call the MCP endpoint receives.
type headlessServer struct {
	mu      sync.Mutex
	output  strings.Builder
	server  *httptest.Server
	mcpHits int
}

func newHeadlessServer(t *testing.T) *headlessServer {
	t.Helper()
	h := &headlessServer{}
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/run-output" {
			var body struct {
				Output string `json:"output"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			h.mu.Lock()
			h.output.WriteString(body.Output)
			h.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.mu.Lock()
		h.mcpHits++
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(h.server.Close)
	return h
}

func (h *headlessServer) captured() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.output.String()
}

func runHeadless(t *testing.T, command string) (*headlessServer, *controlledRun) {
	t.Helper()
	server := newHeadlessServer(t)
	d := &agentDaemon{link: serverLink{serverURL: server.server.URL}}
	payload := agentconfig.Dispatch{RunID: "run-1", TaskKey: "#7", SkillID: "clarify"}
	if err := d.startHeadlessRun("task-a", payload, agentconfig.Config{ProjectID: "project"}, t.TempDir(), "feat/x", map[string]string{}, command, "claude", "claude-opus-5"); err != nil {
		t.Fatalf("startHeadlessRun: %v", err)
	}
	d.queue.mu.Lock()
	run := d.queue.runs["run-1"]
	d.queue.mu.Unlock()
	select {
	case <-run.exited:
	case <-time.After(20 * time.Second):
		t.Fatal("the headless run never finished")
	}
	return server, run
}

// A headless run opens no terminal, so everything it printed has to survive on
// the activity. The whole tail matters: a failing CLI explains itself last.
func TestHeadlessRunCapturesAllOutput(t *testing.T) {
	server, run := runHeadless(t, "for i in $(seq 1 500); do echo line-$i; done")
	captured := server.captured()
	for _, want := range []string{"line-1\n", "line-250\n", "line-500\n"} {
		if !strings.Contains(captured, want) {
			t.Fatalf("captured output is missing %q (got %d bytes)", want, len(captured))
		}
	}
	if run.desktop.Status != "completed" {
		t.Fatalf("status = %q, want completed", run.desktop.Status)
	}
	if !run.desktop.Headless {
		t.Fatal("a headless run must be marked as such so the desktop does not report a missing console")
	}
	if run.desktop.SessionID != "" {
		t.Fatalf("a headless run must not claim a PTY session, got %q", run.desktop.SessionID)
	}
}

// stderr belongs in the same record as stdout, in the order things were printed.
func TestHeadlessRunCapturesStderr(t *testing.T) {
	server, _ := runHeadless(t, "echo to-stdout; echo to-stderr 1>&2")
	captured := server.captured()
	if !strings.Contains(captured, "to-stdout") || !strings.Contains(captured, "to-stderr") {
		t.Fatalf("both streams should be recorded, got %q", captured)
	}
}

// A failing run is recorded as failed, with what it printed before dying.
func TestHeadlessRunReportsFailure(t *testing.T) {
	server, run := runHeadless(t, "echo before-the-failure; exit 3")
	if run.desktop.Status != "failed" {
		t.Fatalf("status = %q, want failed", run.desktop.Status)
	}
	if !strings.Contains(server.captured(), "before-the-failure") {
		t.Fatalf("the output preceding the failure was lost: %q", server.captured())
	}
}

// A provider CLI is never the direct child: the shell reaches it through a
// launcher script, and that grandchild inherits the output pipe. A stop that
// only reached the shell left the CLI running, holding the pipe open, and the
// supervisor waited for an EOF that could not come - the run stayed "running"
// with no way to stop it, and the CLI kept working on the repository.
func TestHeadlessRunStopReachesTheWholeTree(t *testing.T) {
	server := newHeadlessServer(t)
	d := &agentDaemon{link: serverLink{serverURL: server.server.URL}}
	payload := agentconfig.Dispatch{RunID: "run-tree", TaskKey: "#7", SkillID: "clarify"}
	// The inner shell is the grandchild: killing the outer one alone leaves it
	// alive with the write end of the pipe.
	if err := d.startHeadlessRun("task-a", payload, agentconfig.Config{ProjectID: "project"}, t.TempDir(), "feat/x", map[string]string{}, "sh -c 'sleep 300'", "claude", "claude-opus-5"); err != nil {
		t.Fatalf("startHeadlessRun: %v", err)
	}
	d.queue.mu.Lock()
	run := d.queue.runs["run-tree"]
	run.canceled = true
	d.queue.mu.Unlock()

	select {
	case <-run.exited:
	case <-time.After(25 * time.Second):
		t.Fatal("a canceled headless run never finished: the stop did not reach past the shell, and the supervisor waited on a pipe a survivor still held")
	}
	if run.desktop.Status != "canceled" {
		t.Fatalf("status = %q, want canceled", run.desktop.Status)
	}
}

// The desktop's stop button must actually stop an autonomous run. The supervisor
// signals the process GROUP, so the child has to be its own group leader.
func TestHeadlessRunIsStoppable(t *testing.T) {
	server := newHeadlessServer(t)
	d := &agentDaemon{link: serverLink{serverURL: server.server.URL}}
	payload := agentconfig.Dispatch{RunID: "run-stop", TaskKey: "#7", SkillID: "clarify"}
	if err := d.startHeadlessRun("task-a", payload, agentconfig.Config{ProjectID: "project"}, t.TempDir(), "feat/x", map[string]string{}, "sleep 120", "claude", "claude-opus-5"); err != nil {
		t.Fatalf("startHeadlessRun: %v", err)
	}
	d.queue.mu.Lock()
	run := d.queue.runs["run-stop"]
	run.canceled = true
	d.queue.mu.Unlock()

	select {
	case <-run.exited:
	case <-time.After(25 * time.Second):
		t.Fatal("a canceled headless run never stopped: the stop signal did not reach the process")
	}
	if run.desktop.Status != "canceled" {
		t.Fatalf("status = %q, want canceled", run.desktop.Status)
	}
}

// A pipeline reports the failure of any of its stages. Without pipefail a dying
// provider CLI behind a filter that exits zero is recorded as a run that
// completed and printed nothing, which is the silence this change removes.
func TestHeadlessRunFailsWhenAStageOfThePipelineFails(t *testing.T) {
	_, run := runHeadless(t, "echo hello; exit 3 | cat")
	if run.desktop.Status != "failed" {
		t.Fatalf("status = %q, want failed: a broken pipeline must not report success", run.desktop.Status)
	}
}

// The agent keeps its own copy of the output so the desktop can show a run that
// has no console to attach to.
func TestHeadlessRunKeepsATranscript(t *testing.T) {
	_, run := runHeadless(t, "echo transcribed")
	if !strings.Contains(run.transcript, "transcribed") {
		t.Fatalf("transcript = %q, want the run output", run.transcript)
	}
}

// jq is a prerequisite of the shipped autonomous presets. A workstation without
// it must say so, on the run, before anything is spawned.
func TestHeadlessRunRefusesAMissingPipelineTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	server := newHeadlessServer(t)
	d := &agentDaemon{link: serverLink{serverURL: server.server.URL}}
	payload := agentconfig.Dispatch{RunID: "run-jq", TaskKey: "#7", SkillID: "clarify"}
	err := d.startHeadlessRun("task-a", payload, agentconfig.Config{ProjectID: "project"}, t.TempDir(), "feat/x", map[string]string{}, `echo hi | jq .`, "claude", "claude-opus-5")
	if err == nil {
		t.Fatal("a command line needing an absent jq must not be launched")
	}
	if !strings.Contains(err.Error(), "jq") {
		t.Fatalf("the refusal must name the missing tool, got %q", err)
	}
	d.queue.mu.Lock()
	run := d.queue.runs["run-jq"]
	d.queue.mu.Unlock()
	if run == nil || run.desktop.Status != "failed" {
		t.Fatalf("the run must be recorded as failed, got %+v", run)
	}
	if !strings.Contains(server.captured(), "jq") {
		t.Fatalf("the run output must name jq, got %q", server.captured())
	}
}

// The detection is a word match, so a command line that only mentions jq inside
// its prompt is not held back by a tool it never invokes.
func TestMissingPipelineToolMatchesAnInvocationNotAWord(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := missingPipelineTool(`claude -p 'explain jqueries and jq-like tools'`); got != "" {
		t.Fatalf("missingPipelineTool = %q, want no missing tool for a prompt that merely mentions it", got)
	}
	for _, line := range []string{"echo x | jq .", "jq . file", "echo x|jq ."} {
		if got := missingPipelineTool(line); got != "jq" {
			t.Fatalf("missingPipelineTool(%q) = %q, want jq", line, got)
		}
	}
}
