package taskmcp

import (
	"fmt"
	"strings"
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

// recordingNoter records what a silence appended, which is the only thing an
// observation is allowed to do.
type recordingNoter struct {
	mu    sync.Mutex
	notes []string
}

func (n *recordingNoter) NoteRemoteRun(runID, note string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notes = append(n.notes, runID+": "+note)
	return nil
}

func (n *recordingNoter) recorded() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.notes...)
}

// silentRegistry builds a registry whose sweeper never runs on its own, so a
// test decides when time passes and what it finds.
func silentRegistry(t *testing.T, closer RunCloser, notes RunNoter, bound time.Duration) *SessionRegistry {
	t.Helper()
	registry := NewSessionRegistryWith(closer, notes, bound)
	registry.Stop()
	return registry
}

func TestSilenceMarksRunsWithoutClosingThem(t *testing.T) {
	closer, notes := &recordingCloser{}, &recordingNoter{}
	registry := silentRegistry(t, closer, notes, time.Hour)
	base := time.Unix(1700000000, 0)
	registry.now = func() time.Time { return base }
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "implement")

	registry.now = func() time.Time { return base.Add(2 * time.Hour) }
	registry.markSilentSessions()

	if calls := closer.recorded(); len(calls) != 0 {
		t.Fatalf("a silence closed runs: %v", calls)
	}
	recorded := notes.recorded()
	if len(recorded) != 1 || !strings.HasPrefix(recorded[0], "run-1: "+models.RunSilencePrefix) {
		t.Fatalf("notes = %v, want one silence sentence on run-1", recorded)
	}
	if snapshot := registry.Snapshot(); len(snapshot) != 1 {
		t.Fatalf("snapshot = %v, want the session still live", snapshot)
	}
}

func TestSilenceIsRemarkedOncePerStretch(t *testing.T) {
	notes := &recordingNoter{}
	registry := silentRegistry(t, &recordingCloser{}, notes, time.Hour)
	base := time.Unix(1700000000, 0)
	registry.now = func() time.Time { return base }
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "implement")

	registry.now = func() time.Time { return base.Add(2 * time.Hour) }
	registry.markSilentSessions()
	registry.now = func() time.Time { return base.Add(3 * time.Hour) }
	registry.markSilentSessions()
	if recorded := notes.recorded(); len(recorded) != 1 {
		t.Fatalf("notes = %v, want a single sentence for one silent stretch", recorded)
	}

	// A message rearms the observation, so the next silence earns its own
	// sentence rather than staying invisible behind the first.
	registry.Touch("session-1")
	registry.now = func() time.Time { return base.Add(5 * time.Hour) }
	registry.markSilentSessions()
	if recorded := notes.recorded(); len(recorded) != 2 {
		t.Fatalf("notes = %v, want a fresh sentence for the second stretch", recorded)
	}
}

// Silence is not an ending: only a real one closes runs, and that path is
// unchanged.
func TestRealEndingStillClosesRunsAfterASilence(t *testing.T) {
	closer, notes := &recordingCloser{}, &recordingNoter{}
	registry := silentRegistry(t, closer, notes, time.Hour)
	base := time.Unix(1700000000, 0)
	registry.now = func() time.Time { return base }
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "implement")

	registry.now = func() time.Time { return base.Add(2 * time.Hour) }
	registry.markSilentSessions()
	if closed := registry.Close("session-1"); closed != 1 {
		t.Fatalf("closed = %d, want 1", closed)
	}
	calls := closer.recorded()
	if len(calls) != 1 || calls[0] != "TASK-1/run-1/"+disconnectStatus {
		t.Fatalf("finish calls = %v, want the run canceled on the real ending", calls)
	}
}

// A silence on a session that adopted nothing, and a Touch on a session that
// does not exist, must both stay harmless.
func TestSilenceToleratesSessionsWithoutRuns(t *testing.T) {
	notes := &recordingNoter{}
	registry := silentRegistry(t, &recordingCloser{}, notes, time.Hour)
	base := time.Unix(1700000000, 0)
	registry.now = func() time.Time { return base }
	registry.open("session-1", nil)
	registry.Touch("unknown-session")

	registry.now = func() time.Time { return base.Add(2 * time.Hour) }
	registry.markSilentSessions()
	if recorded := notes.recorded(); len(recorded) != 0 {
		t.Fatalf("notes = %v, want none for a session owning no run", recorded)
	}

	var nilRegistry *SessionRegistry
	nilRegistry.Touch("session-1")
	nilRegistry.markSilentSessions()
	nilRegistry.Stop()
}

// recordingWaiter stands in for the database's waiting mark.
type recordingWaiter struct {
	mu      sync.Mutex
	cleared []string
}

func (w *recordingWaiter) SetRemoteRunWaiting(runID string, waiting bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !waiting {
		w.cleared = append(w.cleared, runID)
	}
	return nil
}

func (w *recordingWaiter) recorded() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.cleared...)
}

// A session that speaks again is no longer blocked: Resume clears every wait it
// declared, once, and leaves other sessions' waits alone.
func TestResumeClearsOnlyTheSessionsOwnWaits(t *testing.T) {
	waits := &recordingWaiter{}
	registry := silentRegistry(t, &recordingCloser{}, nil, time.Hour)
	registry.SetWaiter(waits)
	registry.open("session-1", nil)
	registry.open("session-2", nil)
	registry.MarkWaiting("session-1", "run-1")
	registry.MarkWaiting("session-2", "run-2")

	if cleared := registry.Resume("session-1"); cleared != 1 {
		t.Fatalf("cleared = %d, want 1", cleared)
	}
	if cleared := registry.Resume("session-1"); cleared != 0 {
		t.Fatalf("a second resume cleared %d waits, want none", cleared)
	}
	if got := waits.recorded(); len(got) != 1 || got[0] != "run-1" {
		t.Fatalf("cleared = %v, want only run-1", got)
	}

	registry.ForgetWaiting("session-2", "run-2")
	if cleared := registry.Resume("session-2"); cleared != 0 {
		t.Fatalf("a wait the session cleared itself was cleared again: %d", cleared)
	}
}

// Closing a session clears the waits it declared on runs it does not own; the
// runs it owns are closed, and their terminal status clears their mark.
func TestCloseClearsWaitsOnRunsTheSessionDoesNotOwn(t *testing.T) {
	waits, closer := &recordingWaiter{}, &recordingCloser{}
	registry := silentRegistry(t, closer, nil, time.Hour)
	registry.SetWaiter(waits)
	registry.open("session-1", nil)
	registry.Adopt("session-1", "owned", "TASK-1", "clarify")
	registry.MarkWaiting("session-1", "owned")
	registry.MarkWaiting("session-1", "launched")

	registry.Close("session-1")
	if got := waits.recorded(); len(got) != 1 || got[0] != "launched" {
		t.Fatalf("cleared = %v, want only the run the session did not own", got)
	}
	if got := closer.recorded(); len(got) != 1 || got[0] != "TASK-1/owned/"+disconnectStatus {
		t.Fatalf("closed = %v, want the owned run", got)
	}
}

// A run closed from the board is no longer the session's to close.
func TestReleasedRunIsNotClosedByItsSession(t *testing.T) {
	closer := &recordingCloser{}
	registry := silentRegistry(t, closer, nil, time.Hour)
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "clarify")
	registry.ReleaseRun("run-1")
	registry.ReleaseRun("")

	if closed := registry.Close("session-1"); closed != 0 {
		t.Fatalf("closed = %d, want 0 for a released run", closed)
	}
	var nilRegistry *SessionRegistry
	nilRegistry.ReleaseRun("run-1")
	nilRegistry.MarkWaiting("session-1", "run-1")
	nilRegistry.Resume("session-1")
}

// A session silent past the abandon bound stands for a client that died without
// a word: its runs are closed with the disconnect note, after the silence
// sentence, and the session is forgotten (#319).
func TestAbandonedSessionClosesItsRuns(t *testing.T) {
	closer, notes := &recordingCloser{}, &recordingNoter{}
	registry := NewSessionRegistryBounded(closer, notes, time.Hour, 3*time.Hour)
	registry.Stop()
	base := time.Unix(1700000000, 0)
	registry.now = func() time.Time { return base }
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "implement")

	registry.now = func() time.Time { return base.Add(2 * time.Hour) }
	registry.markSilentSessions()
	if calls := closer.recorded(); len(calls) != 0 {
		t.Fatalf("a session below the abandon bound was closed: %v", calls)
	}

	registry.now = func() time.Time { return base.Add(3 * time.Hour) }
	registry.markSilentSessions()
	if calls := closer.recorded(); len(calls) != 1 || calls[0] != "TASK-1/run-1/"+disconnectStatus {
		t.Fatalf("finish calls = %v, want the run canceled on abandonment", calls)
	}
	if len(notes.recorded()) != 1 {
		t.Fatalf("notes = %v, want the one silence sentence before the closure", notes.recorded())
	}
	if snapshot := registry.Snapshot(); len(snapshot) != 0 {
		t.Fatalf("snapshot = %v, want the abandoned session gone", snapshot)
	}
}

// A message resets the clock, so a session that speaks now and then is never
// abandoned, however long it lives.
func TestSpeakingSessionIsNeverAbandoned(t *testing.T) {
	closer := &recordingCloser{}
	registry := NewSessionRegistryBounded(closer, &recordingNoter{}, time.Hour, 3*time.Hour)
	registry.Stop()
	base := time.Unix(1700000000, 0)
	registry.now = func() time.Time { return base }
	registry.open("session-1", nil)
	registry.Adopt("session-1", "run-1", "TASK-1", "implement")
	for hour := 2; hour <= 20; hour += 2 {
		registry.now = func() time.Time { return base.Add(time.Duration(hour) * time.Hour) }
		registry.Touch("session-1")
		registry.markSilentSessions()
	}
	if calls := closer.recorded(); len(calls) != 0 {
		t.Fatalf("a session that kept speaking was abandoned: %v", calls)
	}
}

// No session is closed before its silence was remarked upon: an abandon bound
// below the silence bound is raised to it, and invalid values take defaults.
func TestAbandonBoundIsNeverBelowTheSilenceBound(t *testing.T) {
	for _, tc := range []struct {
		silence, abandon, want time.Duration
	}{
		{time.Hour, 30 * time.Minute, time.Hour},
		{time.Hour, 0, defaultAbandonAfter},
		{0, -time.Minute, defaultAbandonAfter},
		{10 * time.Hour, 0, 10 * time.Hour},
		{time.Hour, 2 * time.Hour, 2 * time.Hour},
	} {
		registry := NewSessionRegistryBounded(nil, nil, tc.silence, tc.abandon)
		registry.Stop()
		if registry.abandon != tc.want {
			t.Errorf("silence %s, abandon %s: got %s, want %s", tc.silence, tc.abandon, registry.abandon, tc.want)
		}
	}
}
