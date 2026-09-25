package db

import (
	"testing"
	"time"

	"tasks/internal/models"
)

// ownedWaitingRun gives a live agent run owned by a user, marked waiting by a
// session, which is the shape a question asked in a console leaves (#475).
func ownedWaitingRun(t *testing.T, sessionID string) (*DB, *models.TaskActivity) {
	t.Helper()
	database, run := startedRun(t)
	if _, err := database.conn.Exec("UPDATE task_activities SET user_id = 'owner' WHERE id = ?", run.ID); err != nil {
		t.Fatal(err)
	}
	if _, applied, err := database.ReportSessionRunWaitingAs(Actor{ID: "owner"}, false, sessionID, run.TaskID, run.ID, true); err != nil || !applied {
		t.Fatalf("declaring the wait: applied=%v err=%v", applied, err)
	}
	waiting, err := database.GetActivityByID(run.ID)
	if err != nil || waiting.WaitingSince == nil {
		t.Fatalf("the run is not waiting: %#v %v", waiting, err)
	}
	return database, waiting
}

func waitingSinceOf(t *testing.T, database *DB, runID string) *time.Time {
	t.Helper()
	activity, err := database.GetActivityByID(runID)
	if err != nil || activity == nil {
		t.Fatalf("reading run %s: %v", runID, err)
	}
	return activity.WaitingSince
}

// The declaring session's next call ends the wait from the database alone, so
// it does not matter which instance serves it or whether one restarted.
func TestResumeWaitsEndsTheSessionsWait(t *testing.T) {
	database, run := ownedWaitingRun(t, "instance-a.session-1")

	cleared, err := database.ResumeWaits("instance-a.session-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared) != 1 || cleared[0] != run.ID {
		t.Fatalf("cleared %v, want [%s]", cleared, run.ID)
	}
	if since := waitingSinceOf(t, database, run.ID); since != nil {
		t.Fatalf("the run still waits since %v", since)
	}
	if again, err := database.ResumeWaits("instance-a.session-1"); err != nil || len(again) != 0 {
		t.Fatalf("a second resume cleared %v (%v)", again, err)
	}
}

func TestResumeWaitsLeavesAnotherSessionsWait(t *testing.T) {
	database, run := ownedWaitingRun(t, "session-1")

	if cleared, err := database.ResumeWaits("session-2"); err != nil || len(cleared) != 0 {
		t.Fatalf("another session cleared %v (%v)", cleared, err)
	}
	if cleared, err := database.ResumeWaits(""); err != nil || len(cleared) != 0 {
		t.Fatalf("an empty session cleared %v (%v)", cleared, err)
	}
	if waitingSinceOf(t, database, run.ID) == nil {
		t.Fatal("the wait was lost to another session's call")
	}
}

// A wait set by hand belongs to no session, so no session's call ends it.
func TestAHandSetWaitIsNotResumedBySessions(t *testing.T) {
	database, run := startedRun(t)
	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	if cleared, err := database.ResumeWaits(""); err != nil || len(cleared) != 0 {
		t.Fatalf("cleared %v (%v)", cleared, err)
	}
	if waitingSinceOf(t, database, run.ID) == nil {
		t.Fatal("the hand-set wait was cleared")
	}
}

// The latest session to declare the wait is the one whose call ends it, while
// the clock keeps the first mark.
func TestTheLatestDeclaringSessionOwnsTheWait(t *testing.T) {
	database, run := ownedWaitingRun(t, "session-1")
	if _, _, err := database.ReportSessionRunWaitingAs(Actor{ID: "owner"}, false, "session-2", run.TaskID, run.ID, true); err != nil {
		t.Fatal(err)
	}
	if since := waitingSinceOf(t, database, run.ID); since == nil || !since.Equal(*run.WaitingSince) {
		t.Fatalf("the second declaration moved the mark from %v to %v", run.WaitingSince, since)
	}
	if cleared, _ := database.ResumeWaits("session-1"); len(cleared) != 0 {
		t.Fatal("the first session still ended a wait the second one declared")
	}
	if cleared, _ := database.ResumeWaits("session-2"); len(cleared) != 1 {
		t.Fatal("the second session's call did not end its wait")
	}
}

func TestResumeWaitsIgnoresAClosedRun(t *testing.T) {
	database, run := ownedWaitingRun(t, "session-1")
	if _, err := database.conn.Exec("UPDATE task_activities SET status = 'completed' WHERE id = ?", run.ID); err != nil {
		t.Fatal(err)
	}
	if cleared, err := database.ResumeWaits("session-1"); err != nil || len(cleared) != 0 {
		t.Fatalf("a closed run was touched: %v (%v)", cleared, err)
	}
}

func TestAnAnswerEndsTheWaitItAnswered(t *testing.T) {
	database, run := ownedWaitingRun(t, "session-1")

	// The agent sends back the instant it was pushed, after a JSON round trip.
	answered := run.WaitingSince.UTC()
	cleared, err := database.AnswerRemoteRunWait("owner", run.ID, answered)
	if err != nil || !cleared {
		t.Fatalf("the answer was not applied: cleared=%v err=%v", cleared, err)
	}
	if since := waitingSinceOf(t, database, run.ID); since != nil {
		t.Fatalf("the run still waits since %v", since)
	}
	if cleared, err := database.AnswerRemoteRunWait("owner", run.ID, answered); err != nil || cleared {
		t.Fatalf("a repeated answer reported a clear: %v (%v)", cleared, err)
	}
}

// A question asked after the answer was typed is a new wait: the answer to the
// previous one must not end it.
func TestAnAnswerToAnOlderWaitKeepsTheNewerOne(t *testing.T) {
	database, run := ownedWaitingRun(t, "session-1")
	older := *run.WaitingSince
	if cleared, _ := database.ResumeWaits("session-1"); len(cleared) != 1 {
		t.Fatal("setup: the wait was not resumed")
	}
	time.Sleep(5 * time.Millisecond)
	if _, _, err := database.ReportSessionRunWaitingAs(Actor{ID: "owner"}, false, "session-1", run.TaskID, run.ID, true); err != nil {
		t.Fatal(err)
	}
	newer := waitingSinceOf(t, database, run.ID)
	if newer == nil || !newer.After(older) {
		t.Fatalf("a re-mark after a clear kept the old instant: %v then %v", older, newer)
	}

	if cleared, err := database.AnswerRemoteRunWait("owner", run.ID, older); err != nil || cleared {
		t.Fatalf("the old answer ended the new wait: %v (%v)", cleared, err)
	}
	if waitingSinceOf(t, database, run.ID) == nil {
		t.Fatal("the new wait is gone")
	}
}

func TestAnAnswerFromAnotherUserIsRefused(t *testing.T) {
	database, run := ownedWaitingRun(t, "session-1")
	if cleared, err := database.AnswerRemoteRunWait("intruder", run.ID, *run.WaitingSince); err != nil || cleared {
		t.Fatalf("another user's answer was applied: %v (%v)", cleared, err)
	}
	if waitingSinceOf(t, database, run.ID) == nil {
		t.Fatal("the wait was cleared by another user")
	}
}

// A launch parked on a repository is answered by a pin on the ticket, never by
// a key in the console.
func TestAnAnswerLeavesARepositoryWait(t *testing.T) {
	database, run := startedRun(t)
	if _, err := database.conn.Exec("UPDATE task_activities SET user_id = 'owner' WHERE id = ?", run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.MarkRunAwaitingRepository(Actor{ID: "owner"}, false, run.ID, true); err != nil {
		t.Fatal(err)
	}
	since := waitingSinceOf(t, database, run.ID)
	if since == nil {
		t.Fatal("setup: the run is not parked")
	}
	if cleared, err := database.AnswerRemoteRunWait("owner", run.ID, *since); err != nil || cleared {
		t.Fatalf("a console answer released a repository wait: %v (%v)", cleared, err)
	}
}

// The wait listeners hear the changes of the mark, and nothing else: a
// repeated declaration keeps the mark and is not relayed.
func TestWaitListenersHearOnlyRealChanges(t *testing.T) {
	database, run := startedRun(t)
	heard := make(chan *time.Time, 8)
	database.RegisterWaitListener(func(_ *models.Task, activity *models.TaskActivity) {
		heard <- activity.WaitingSince
	})
	expect := func(waiting bool) {
		t.Helper()
		select {
		case since := <-heard:
			if (since != nil) != waiting {
				t.Fatalf("heard waitingSince=%v, want waiting=%v", since, waiting)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no wait change heard, want waiting=%v", waiting)
		}
	}
	quiet := func() {
		t.Helper()
		select {
		case since := <-heard:
			t.Fatalf("an unchanged mark was relayed: %v", since)
		case <-time.After(100 * time.Millisecond):
		}
	}

	if _, _, err := database.ReportSessionRunWaitingAs(Actor{}, true, "session-1", run.TaskID, run.ID, true); err != nil {
		t.Fatal(err)
	}
	expect(true)
	if _, _, err := database.ReportSessionRunWaitingAs(Actor{}, true, "session-1", run.TaskID, run.ID, true); err != nil {
		t.Fatal(err)
	}
	quiet()
	if _, err := database.ResumeWaits("session-1"); err != nil {
		t.Fatal(err)
	}
	expect(false)
	if err := database.SetRemoteRunWaiting(run.ID, false); err != nil {
		t.Fatalf("clearing a run that no longer waits: %v", err)
	}
	quiet()
}
