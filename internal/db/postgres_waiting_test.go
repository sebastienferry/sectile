package db

import (
	"encoding/json"
	"testing"
	"time"

	"tasks/internal/models"
)

// A wait declared through one instance is ended by the declaring session's
// call on another, and an answer naming the instant the agent was pushed,
// after a JSON round trip, matches it on PostgreSQL's precision (#475).
func TestPostgresWaitIsEndedFromAnotherInstance(t *testing.T) {
	first, second := openPostgresPair(t)
	project, err := first.CreateProject(models.CreateProjectRequest{Name: "Waiting"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := first.CreateTask(models.CreateTaskRequest{Title: "Blocked on a prompt", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	run, err := first.StartAgentRemoteRun(task.ID, "implement")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.conn.Exec("UPDATE task_activities SET user_id = 'owner' WHERE id = ?", run.ID); err != nil {
		t.Fatal(err)
	}
	declare := func() time.Time {
		t.Helper()
		if _, applied, err := first.ReportSessionRunWaitingAs(Actor{ID: "owner"}, false, "instance-a.session-1", task.ID, run.ID, true); err != nil || !applied {
			t.Fatalf("declaring the wait: applied=%v err=%v", applied, err)
		}
		activity, err := second.GetActivityByID(run.ID)
		if err != nil || activity.WaitingSince == nil {
			t.Fatalf("the other instance does not see the wait: %v", err)
		}
		// What the agent sends back is what it was pushed, through JSON.
		raw, _ := json.Marshal(activity.WaitingSince)
		var echoed time.Time
		if err := json.Unmarshal(raw, &echoed); err != nil {
			t.Fatal(err)
		}
		return echoed
	}

	declare()
	if cleared, err := second.ResumeWaits("instance-a.session-1"); err != nil || len(cleared) != 1 {
		t.Fatalf("the other instance did not end the session's wait: %v (%v)", cleared, err)
	}

	echoed := declare()
	if cleared, err := second.AnswerRemoteRunWait("owner", run.ID, echoed); err != nil || !cleared {
		t.Fatalf("the echoed instant did not match the wait: %v (%v)", cleared, err)
	}
	if activity, _ := first.GetActivityByID(run.ID); activity.WaitingSince != nil {
		t.Fatal("the answered wait is still recorded")
	}
}

// One instance may be answering the old question while another commits a new
// one. Hold the new mark in a transaction until the answer reaches its update:
// the old implementation reads the old mark first, then clears the new one.
func TestPostgresAnswerDoesNotClearANewerConcurrentWait(t *testing.T) {
	first, second := openPostgresPair(t)
	project, err := first.CreateProject(models.CreateProjectRequest{Name: "Waiting"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := first.CreateTask(models.CreateTaskRequest{Title: "Two questions", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	run, err := first.StartAgentRemoteRun(task.ID, "implement")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.conn.Exec("UPDATE task_activities SET user_id = 'owner' WHERE id = ?", run.ID); err != nil {
		t.Fatal(err)
	}
	if _, applied, err := first.ReportSessionRunWaitingAs(Actor{ID: "owner"}, false, "session-1", task.ID, run.ID, true); err != nil || !applied {
		t.Fatalf("declaring the first wait: applied=%v err=%v", applied, err)
	}
	current, err := first.GetActivityByID(run.ID)
	if err != nil || current.WaitingSince == nil {
		t.Fatalf("reading the first wait: %v", err)
	}
	oldMark := *current.WaitingSince
	newMark := oldMark.Add(time.Second)

	// This row update represents the old wait ending and a new declaration.
	// Until commit, a plain SELECT on the other instance still sees oldMark.
	tx, err := first.conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE task_activities SET waiting_since = ?, waiting_session = 'session-2', waiting_reason = '' WHERE id = ?`, newMark, run.ID); err != nil {
		t.Fatal(err)
	}
	type answerResult struct {
		cleared bool
		err     error
	}
	answered := make(chan answerResult, 1)
	go func() {
		cleared, err := second.AnswerRemoteRunWait("owner", run.ID, oldMark)
		answered <- answerResult{cleared, err}
	}()

	// Both implementations block at the UPDATE while the new mark is held.
	// Waiting for that lock makes this interleaving deterministic.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked int
		err := first.conn.QueryRow(`SELECT COUNT(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND query LIKE 'UPDATE task_activities SET waiting_since=NULL, waiting_session=%'`).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("answer did not reach the locked update")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-answered:
		if result.err != nil || result.cleared {
			t.Fatalf("old answer cleared the new wait: cleared=%v err=%v", result.cleared, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("answer did not finish after the new wait committed")
	}
	activity, err := first.GetActivityByID(run.ID)
	if err != nil || activity.WaitingSince == nil || !activity.WaitingSince.Equal(newMark) {
		t.Fatalf("new wait did not survive the old answer: activity=%+v err=%v", activity, err)
	}
}
