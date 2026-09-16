package taskmcp

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
)

// recordingCloser stands in for the database so session ownership can be
// exercised on its own terms: what matters here is which runs get closed and
// how, not how an activity is persisted.
type recordingCloser struct {
	mu     sync.Mutex
	calls  []string
	failOn string
}

func (c *recordingCloser) FinishRemoteRun(taskKey, runID, status, note string) (*models.TaskActivity, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if runID == c.failOn {
		return nil, fmt.Errorf("already finished")
	}
	c.calls = append(c.calls, fmt.Sprintf("%s/%s/%s", taskKey, runID, status))
	return &models.TaskActivity{ID: runID, Status: status, Summary: note}, nil
}

func (c *recordingCloser) recorded() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

func TestSessionClosesAdoptedRuns(t *testing.T) {
	closer := &recordingCloser{}
	registry := NewSessionRegistry(closer)
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "implement")
	registry.Adopt("session-1", "run-2", "TASK-2", "specify")

	if closed := registry.Close("session-1"); closed != 2 {
		t.Fatalf("closed = %d, want 2", closed)
	}
	calls := closer.recorded()
	if len(calls) != 2 {
		t.Fatalf("finish calls = %v, want two", calls)
	}
	for _, call := range calls {
		if got := call[len(call)-len(disconnectStatus):]; got != disconnectStatus {
			t.Fatalf("call %q did not close as %s", call, disconnectStatus)
		}
	}
	if snapshot := registry.Snapshot(); len(snapshot) != 0 {
		t.Fatalf("snapshot after close = %v, want empty", snapshot)
	}
}

func TestReleasedRunSurvivesSessionEnd(t *testing.T) {
	closer := &recordingCloser{}
	registry := NewSessionRegistry(closer)
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "implement")
	registry.Release("session-1", "run-1")

	if closed := registry.Close("session-1"); closed != 0 {
		t.Fatalf("closed = %d, want 0", closed)
	}
	if calls := closer.recorded(); len(calls) != 0 {
		t.Fatalf("a run the client finished was closed again: %v", calls)
	}
}

// A run somebody else already finished is an ordinary race, and it must not
// cost the session's remaining runs their closure.
func TestCloseContinuesAfterAFailedRun(t *testing.T) {
	closer := &recordingCloser{failOn: "run-1"}
	registry := NewSessionRegistry(closer)
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "implement")
	registry.Adopt("session-1", "run-2", "TASK-2", "specify")

	if closed := registry.Close("session-1"); closed != 1 {
		t.Fatalf("closed = %d, want 1", closed)
	}
	calls := closer.recorded()
	if len(calls) != 1 || calls[0] != "TASK-2/run-2/"+disconnectStatus {
		t.Fatalf("finish calls = %v, want only the second run", calls)
	}
}

func TestSnapshotDescribesLiveSessions(t *testing.T) {
	registry := NewSessionRegistry(&recordingCloser{})
	registry.now = func() time.Time { return time.Unix(1700000000, 0) }
	registry.open("session-1", nil)
	registry.now = func() time.Time { return time.Unix(1700000060, 0) }
	registry.open("session-2", nil)
	registry.Adopt("session-2", "run-9", "TASK-9", "implement")

	snapshot := registry.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot = %v, want two sessions", snapshot)
	}
	// Most recent first, so the newest connection is the one an operator reads.
	if snapshot[0].ID != "session-2" || len(snapshot[0].Runs) != 1 || snapshot[0].Runs[0] != "run-9" {
		t.Fatalf("first session = %+v, want session-2 owning run-9", snapshot[0])
	}
	if snapshot[1].ID != "session-1" || len(snapshot[1].Runs) != 0 {
		t.Fatalf("second session = %+v, want session-1 without runs", snapshot[1])
	}
	if snapshot[0].Client != "unknown" {
		t.Fatalf("client = %q, want the unknown fallback", snapshot[0].Client)
	}
}

// A transport without sessions, and a server built without a registry, both
// keep the previous client-owned behaviour rather than failing.
func TestRegistryToleratesMissingSessions(t *testing.T) {
	var registry *SessionRegistry
	registry.Adopt("", "run-1", "TASK-1", "implement")
	registry.Release("", "run-1")
	if closed := registry.Close("session-1"); closed != 0 {
		t.Fatalf("closed = %d on a nil registry, want 0", closed)
	}
	if snapshot := registry.Snapshot(); len(snapshot) != 0 {
		t.Fatalf("snapshot = %v on a nil registry, want empty", snapshot)
	}

	live := NewSessionRegistry(&recordingCloser{})
	live.open("session-1", nil)
	live.Adopt("", "run-1", "TASK-1", "implement")
	if closed := live.Close("session-1"); closed != 0 {
		t.Fatalf("closed = %d, want 0 for a run adopted without a session", closed)
	}
}
