package db

import (
	"strings"
	"testing"
)

// queuedLauncherRun is the state #499 met: the launcher recorded the run, then
// the agent reported it queued while it waited for a slot, although the session
// it launched was already at work.
func queuedLauncherRun(t *testing.T, d *DB) string {
	t.Helper()
	run, err := d.StartAgentRun("t1", "handoff", RunLaunch{UserID: "u1"})
	if err != nil {
		t.Fatalf("launching: %v", err)
	}
	queued, err := d.SyncRemoteRunStatusFor("u1", run.ID, "t1", "p1", "#1", "handoff", "queued", "Execution queued on agent", nil)
	if err != nil {
		t.Fatalf("the agent's queued report: %v", err)
	}
	if queued.Status != "queued" {
		t.Fatalf("status after the agent's report = %q, want queued", queued.Status)
	}
	return run.ID
}

// The path of #499 end to end: the session adopts the queued run it was
// handed, which then shows running, and reports how it ended.
func TestALauncherRunLeftQueuedIsAdoptedThenFinished(t *testing.T) {
	d, _ := activeRunDB(t)
	runID := queuedLauncherRun(t, d)

	adopted, err := d.StartRemoteRunBy("u1", "t1", "handoff", runID)
	if err != nil {
		t.Fatalf("adopting the queued run: %v", err)
	}
	if adopted.ID != runID || adopted.Status != "running" || adopted.StartedAt == nil {
		t.Fatalf("adopted run = %s %q started %v, want %s running with a start time", adopted.ID, adopted.Status, adopted.StartedAt, runID)
	}
	// A second adoption, from a retry or a second session, changes nothing.
	again, err := d.StartRemoteRunBy("u1", "t1", "handoff", runID)
	if err != nil || again.Status != "running" || !again.StartedAt.Equal(*adopted.StartedAt) {
		t.Fatalf("second adoption = %+v (%v), want the same running run", again, err)
	}

	finished, err := d.FinishRemoteRunAs(Actor{ID: "u1"}, false, "t1", runID, "completed", "handoff done")
	if err != nil {
		t.Fatalf("finishing the adopted run: %v", err)
	}
	if finished.Status != "completed" || !strings.Contains(finished.Summary, "handoff done") {
		t.Fatalf("finished run = %q %q, want completed with the note", finished.Status, finished.Summary)
	}
}

// A session that never called start_run still records the outcome of the
// queued run it holds.
func TestAQueuedLauncherRunCanBeFinishedDirectly(t *testing.T) {
	d, _ := activeRunDB(t)
	runID := queuedLauncherRun(t, d)

	finished, err := d.FinishRemoteRunAs(Actor{ID: "u1"}, false, "t1", runID, "failed", "stopped early")
	if err != nil {
		t.Fatalf("finishing the queued run: %v", err)
	}
	if finished.Status != "failed" {
		t.Fatalf("status = %q, want failed", finished.Status)
	}
	// And ownership still holds: somebody else cannot close it.
	other := queuedLauncherRun(t, d)
	if _, err := d.FinishRemoteRunAs(Actor{ID: "u2"}, false, "t1", other, "completed", "not mine"); err != ErrRunNotYours {
		t.Fatalf("another user closing a queued run: err = %v, want ErrRunNotYours", err)
	}
}

// A refusal says which case it is and what to do instead.
func TestAdoptionRefusalsSayWhyAndWhatToDo(t *testing.T) {
	d, _ := activeRunDB(t)
	runID := queuedLauncherRun(t, d)
	if _, err := d.FinishRemoteRunAs(Actor{ID: "u1"}, false, "t1", runID, "canceled", "stopped"); err != nil {
		t.Fatalf("finishing: %v", err)
	}

	_, err := d.StartRemoteRunBy("u1", "t1", "handoff", runID)
	if err == nil || !strings.Contains(err.Error(), "already ended as canceled") || !strings.Contains(err.Error(), "without a runId") {
		t.Fatalf("adopting an ended run: err = %v, want the ended case and the way out", err)
	}
	_, err = d.StartRemoteRunBy("u1", "t1", "handoff", "no-such-run")
	if err == nil || !strings.Contains(err.Error(), "does not match an active execution on this task") || !strings.Contains(err.Error(), "without a runId") {
		t.Fatalf("adopting an unknown run: err = %v, want the unmatched case and the way out", err)
	}
}

// A queued macro run is adopted and finished the same way.
func TestAQueuedMacroRunIsAdoptedThenFinished(t *testing.T) {
	d, project := macroRunDB(t)
	run, err := d.StartMacroRun(project.ID, "M-7", "realign_macro", RunLaunch{UserID: "u1"})
	if err != nil {
		t.Fatalf("launching: %v", err)
	}
	if _, err := d.conn.Exec("UPDATE task_activities SET status='queued', started_at=NULL WHERE id=?", run.ID); err != nil {
		t.Fatalf("queueing: %v", err)
	}

	adopted, err := d.StartMacroRunBy("u1", project.ID, "M-7", "realign_macro", run.ID)
	if err != nil {
		t.Fatalf("adopting the queued macro run: %v", err)
	}
	if adopted.Status != "running" || adopted.StartedAt == nil || adopted.MacroKey != "M-7" {
		t.Fatalf("adopted = %q started %v macro %q, want running with a start time on M-7", adopted.Status, adopted.StartedAt, adopted.MacroKey)
	}
	if _, err := d.conn.Exec("UPDATE task_activities SET status='queued' WHERE id=?", run.ID); err != nil {
		t.Fatalf("queueing again: %v", err)
	}
	finished, err := d.FinishMacroRunAs(Actor{ID: "u1"}, false, project.ID, "M-7", run.ID, "completed", "done")
	if err != nil || finished.Status != "completed" {
		t.Fatalf("finishing the queued macro run = %+v (%v), want completed", finished, err)
	}
	if _, err := d.StartMacroRunBy("u1", project.ID, "M-7", "realign_macro", run.ID); err == nil || !strings.Contains(err.Error(), "already ended as completed") {
		t.Fatalf("adopting an ended macro run: err = %v, want the ended case", err)
	}
}
