package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestHeadlessIdentityIsDurableBeforeTheFirstChildEffect(t *testing.T) {
	server := newHeadlessServer(t)
	directory := t.TempDir()
	store := openRunStore(filepath.Join(directory, "runs"))
	daemon := &agentDaemon{store: store, link: serverLink{serverURL: server.server.URL}}
	command := "if [ -f " + quoteShell(store.path("durable-headless")) + " ]; then printf CHECKPOINT_PRESENT; else printf CHECKPOINT_MISSING; exit 71; fi"
	err := daemon.startHeadlessRun("fixture", agentconfig.Dispatch{RunID: "durable-headless"}, agentconfig.Config{ProjectID: "fixture"}, directory, "feat/fixture", nil, command, "claude", "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	daemon.queue.mu.Lock()
	run := daemon.queue.runs["durable-headless"]
	daemon.queue.mu.Unlock()
	select {
	case <-run.exited:
	case <-time.After(10 * time.Second):
		t.Fatal("headless child did not finish")
	}
	if server.captured() != "CHECKPOINT_PRESENT" {
		t.Fatalf("child ran without its durable identity: %q", server.captured())
	}
	data, err := os.ReadFile(store.path("durable-headless"))
	if err != nil {
		t.Fatal(err)
	}
	var record storedRun
	if json.Unmarshal(data, &record) != nil || record.Run.Provider != "claude" || record.Run.Model != "fixture-model" || record.Run.ProjectID != "fixture" {
		t.Fatal("initial checkpoint lost run identity")
	}
	daemon.persistRun("durable-headless", run)
}

func TestHeadlessStorageFailureRefusesToStartTheProvider(t *testing.T) {
	server := newHeadlessServer(t)
	directory := t.TempDir()
	store := openRunStore(filepath.Join(directory, "runs"))
	broken := filepath.Join(directory, "not-a-directory")
	if err := os.WriteFile(broken, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	store.dir = broken
	daemon := &agentDaemon{store: store, link: serverLink{serverURL: server.server.URL}}
	marker := filepath.Join(directory, "unexpected-provider-effect")
	err := daemon.startHeadlessRun("fixture", agentconfig.Dispatch{RunID: "storage-failure"}, agentconfig.Config{ProjectID: "fixture"}, directory, "feat/fixture", nil, "printf unexpected > "+quoteShell(marker), "codex", "fixture-model")
	if err == nil {
		t.Fatal("provider launch accepted a failed initial checkpoint")
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatal("provider performed work despite a failed initial checkpoint")
	}
	daemon.queue.mu.Lock()
	status := daemon.queue.runs["storage-failure"].desktop.Status
	daemon.queue.mu.Unlock()
	if status != "failed" {
		t.Fatalf("refused launch status = %q", status)
	}
}
