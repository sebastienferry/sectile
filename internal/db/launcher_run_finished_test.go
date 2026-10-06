package db

import (
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// finishedLauncherRun walks the sequence of #393: the board launches a run on
// the agent, the skill session adopts it with start_run(runId) and closes it
// with finish_run(completed). It returns the run as finish_run left it.
func finishedLauncherRun(t *testing.T, d *DB) *models.TaskActivity {
	t.Helper()
	seedProjectAndUser(t, d)
	seedTask(t, d)
	launched, err := d.StartAgentRun("t1", "clarify-issue", RunLaunch{UserID: "u1"})
	if err != nil {
		t.Fatalf("launching the run: %v", err)
	}
	adopted, err := d.StartRemoteRunBy("u1", "t1", "clarify-issue", launched.ID)
	if err != nil {
		t.Fatalf("adopting the launcher run: %v", err)
	}
	if adopted.ID != launched.ID || adopted.Status != "running" {
		t.Fatalf("adoption = %#v, want the launcher run running", adopted)
	}
	finished, err := d.FinishRemoteRunAs(Actor{ID: "u1"}, false, "t1", launched.ID, "completed", "Clarification done")
	if err != nil {
		t.Fatalf("finishing the run: %v", err)
	}
	if finished.Status != "completed" || finished.CompletedAt == nil {
		t.Fatalf("finish_run answered %#v, want completed with a completion time", finished)
	}
	return finished
}

// wantStillFinished reads the run back and fails unless it is exactly as
// finish_run left it: same status, same completion time, same summary.
func wantStillFinished(t *testing.T, d *DB, finished *models.TaskActivity, after string) {
	t.Helper()
	got, err := d.GetActivityByID(finished.ID)
	if err != nil || got == nil {
		t.Fatalf("reading the run back after %s: %v", after, err)
	}
	if got.Status != finished.Status {
		t.Errorf("after %s, status = %q, want %q", after, got.Status, finished.Status)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(*finished.CompletedAt) {
		t.Errorf("after %s, completedAt = %v, want %v", after, got.CompletedAt, finished.CompletedAt)
	}
	if got.Summary != finished.Summary {
		t.Errorf("after %s, summary = %q, want %q", after, got.Summary, finished.Summary)
	}
}

// TestFinishedLauncherRunIgnoresAgentReports is the acceptance test of #393:
// the agent keeps reporting a run as long as its console is open, after the
// session finished it, and those reports must not reopen it. Before #407 the
// running report rewrote the row to running and cleared its completion.
func TestFinishedLauncherRunIgnoresAgentReports(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		finished := finishedLauncherRun(t, d)
		started := time.Now().Add(-time.Hour)
		for _, status := range []string{"running", "queued"} {
			if _, err := d.SyncRemoteRunStatusFor("u1", finished.ID, "t1", "p1", "#1", "clarify-issue", status, "Execution "+status+" on agent", &started); err != nil {
				t.Fatalf("agent report %s: %v", status, err)
			}
			wantStillFinished(t, d, finished, "an agent report "+status)
		}
	})
}

// TestFinishedLauncherRunFinishIsIdempotent checks the other half of the
// ticket's expectation: finish_run never answers one outcome and keeps another.
// Repeating the same outcome is accepted, a different one is refused.
func TestFinishedLauncherRunFinishIsIdempotent(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		finished := finishedLauncherRun(t, d)
		again, err := d.FinishRemoteRunAs(Actor{ID: "u1"}, false, "t1", finished.ID, "completed", "Clarification done")
		if err != nil {
			t.Fatalf("repeating finish_run(completed): %v", err)
		}
		if again.Status != "completed" {
			t.Errorf("repeated finish_run answered %q, want completed", again.Status)
		}
		if _, err := d.FinishRemoteRunAs(Actor{ID: "u1"}, false, "t1", finished.ID, "failed", "late failure"); err == nil {
			t.Error("finish_run(failed) on a completed run was accepted")
		}
		wantStillFinished(t, d, finished, "a second finish_run")
	})
}

// TestFinishedLauncherRunSurvivesRestartPaths drives the paths that run when a
// server restarts or an agent reconnects (#393, gap 2 of the clarification):
// none of them may bring a finished run back.
func TestFinishedLauncherRunSurvivesRestartPaths(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		finished := finishedLauncherRun(t, d)

		d.recoverInterruptedRuns()
		wantStillFinished(t, d, finished, "the restart recovery")

		if _, err := d.reclaimDeadInstances(time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("reclaiming dead instances: %v", err)
		}
		wantStillFinished(t, d, finished, "the dead-instance reclaim")

		_, err := d.StartRemoteRunBy("u1", "t1", "clarify-issue", finished.ID)
		if err == nil || !strings.Contains(err.Error(), "already ended as completed") {
			t.Errorf("start_run with the finished runId = %v, want an already-ended refusal", err)
		}
		wantStillFinished(t, d, finished, "start_run with its runId")
	})
}

// wantNoActivity fails unless no activity carries the id.
func wantNoActivity(t *testing.T, d *DB, id, after string) {
	t.Helper()
	got, err := d.GetActivityByID(id)
	if err != nil {
		t.Fatalf("reading the run back after %s: %v", after, err)
	}
	if got != nil {
		t.Errorf("after %s, the run exists again as %q (%q)", after, got.Status, got.Summary)
	}
}

// reportRunning applies one agent report of the run, as ApplyAgentRunningTasksFor does.
func reportRunning(t *testing.T, d *DB, id, status string) *models.TaskActivity {
	t.Helper()
	started := time.Now().Add(-time.Hour)
	act, err := d.SyncRemoteRunStatusFor("u1", id, "t1", "p1", "#1", "clarify-issue", status, "Execution "+status+" on agent", &started)
	if err != nil {
		t.Fatalf("agent report %s: %v", status, err)
	}
	return act
}

// TestDeletedFinishedRunIsNotRecreatedByAgentReports is the acceptance test of
// #675: a finished run deleted from the activities view while its console is
// still open must not come back from the agent's next reports. Before the fix
// the insert branch recreated it as running with a fresh createdAt.
func TestDeletedFinishedRunIsNotRecreatedByAgentReports(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		finished := finishedLauncherRun(t, d)
		if err := d.DeleteActivity(finished.ID); err != nil {
			t.Fatalf("deleting the finished run: %v", err)
		}
		for _, status := range []string{"running", "queued"} {
			if act := reportRunning(t, d, finished.ID, status); act != nil {
				t.Errorf("agent report %s answered %#v, want no activity", status, act)
			}
			wantNoActivity(t, d, finished.ID, "an agent report "+status)
		}
	})
}

// TestClearedFinishedRunIsNotRecreatedByAgentReports is the same sequence
// through "clear completed activities", which removes the same finished runs.
func TestClearedFinishedRunIsNotRecreatedByAgentReports(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		finished := finishedLauncherRun(t, d)
		if n, err := d.ClearCompletedActivities(); err != nil || n == 0 {
			t.Fatalf("clearing completed activities = %d, %v, want the finished run removed", n, err)
		}
		reportRunning(t, d, finished.ID, "running")
		wantNoActivity(t, d, finished.ID, "an agent report after the clear")
	})
}

// TestUnknownAgentRunIsStillInserted keeps the reason the insert branch exists:
// a run that started on the agent, which the server never created, is recorded.
func TestUnknownAgentRunIsStillInserted(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		seedProjectAndUser(t, d)
		seedTask(t, d)
		act := reportRunning(t, d, "agent-run-675", "running")
		if act == nil || act.Status != "running" || act.SkillID != "remote_run" || act.UserID != "u1" {
			t.Fatalf("agent report of an unknown run answered %#v, want a running remote_run owned by u1", act)
		}
	})
}

// TestDeletedRunningRunIsRecreatedByAgentReports covers a deletion that does
// not end the run: the agent is still running it, so its report records it again.
func TestDeletedRunningRunIsRecreatedByAgentReports(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		seedProjectAndUser(t, d)
		seedTask(t, d)
		reportRunning(t, d, "agent-run-675", "running")
		if err := d.DeleteActivity("agent-run-675"); err != nil {
			t.Fatalf("deleting the running run: %v", err)
		}
		if act := reportRunning(t, d, "agent-run-675", "running"); act == nil || act.Status != "running" {
			t.Fatalf("agent report after deleting a running run answered %#v, want it running again", act)
		}
	})
}

// TestDeletedRemoteRunRecordsExpire checks that the record of a deletion is
// kept for a bounded time: the next deletion purges one older than 30 days.
func TestDeletedRemoteRunRecordsExpire(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		finished := finishedLauncherRun(t, d)
		if err := d.DeleteActivity(finished.ID); err != nil {
			t.Fatalf("deleting the finished run: %v", err)
		}
		if _, err := d.conn.Exec("UPDATE deleted_remote_runs SET deleted_at = ? WHERE id = ?", time.Now().Add(-31*24*time.Hour), finished.ID); err != nil {
			t.Fatalf("ageing the record: %v", err)
		}
		if err := d.DeleteActivity("no-such-activity"); err != nil {
			t.Fatalf("a later deletion: %v", err)
		}
		if act := reportRunning(t, d, finished.ID, "running"); act == nil {
			t.Fatal("an agent report after the record expired was ignored, want it inserted")
		}
	})
}
