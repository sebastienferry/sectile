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
