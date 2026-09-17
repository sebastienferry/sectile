package db

import (
	"testing"

	"tasks/internal/models"
)

// startedRun gives each test a task carrying one live remote run, which is the
// only shape the waiting state is ever written on.
func startedRun(t *testing.T) (*DB, *models.TaskActivity) {
	t.Helper()
	database := testDB(t)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Waiting"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Blocked on a prompt", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartAgentRemoteRun(task.ID, "implement")
	if err != nil {
		t.Fatal(err)
	}
	return database, run
}

func TestWaitingIsSetAndCleared(t *testing.T) {
	database, run := startedRun(t)

	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	waiting, err := database.GetActivityByID(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if waiting.WaitingSince == nil {
		t.Fatal("the run does not report waiting, so the UI would still show it running")
	}
	if waiting.Status != "running" {
		t.Fatalf("waiting changed the run status to %q; waiting is a phase of a running run", waiting.Status)
	}

	if err := database.SetRemoteRunWaiting(run.ID, false); err != nil {
		t.Fatal(err)
	}
	resumed, err := database.GetActivityByID(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.WaitingSince != nil {
		t.Fatal("the run still reports waiting after resuming")
	}
}

func TestCompletionClearsTheWaitingState(t *testing.T) {
	database, run := startedRun(t)
	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	task, err := database.GetTaskByID(run.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.FinishRemoteRun(task.ID, run.ID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	finished, err := database.GetActivityByID(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.WaitingSince != nil {
		t.Fatal("a finished run is still marked as waiting, which would never clear")
	}
}

func TestCancellationClearsTheWaitingState(t *testing.T) {
	database, run := startedRun(t)
	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := database.CancelActivity(run.ID); err != nil {
		t.Fatal(err)
	}
	canceled, err := database.GetActivityByID(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canceled.WaitingSince != nil {
		t.Fatal("a cancelled run is still marked as waiting")
	}
}

// A hook reports late, after its session already ended, often enough that it is
// the normal case rather than an edge one: nothing may be written back onto a
// run that is no longer running, nor onto one that never existed.
func TestWaitingIsRefusedOnAnUnknownOrFinishedRun(t *testing.T) {
	database, run := startedRun(t)
	if err := database.SetRemoteRunWaiting("no-such-run", true); err == nil {
		t.Fatal("an unknown run accepted a waiting report")
	}
	task, err := database.GetTaskByID(run.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.FinishRemoteRun(task.ID, run.ID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if err := database.SetRemoteRunWaiting(run.ID, true); err == nil {
		t.Fatal("a finished run accepted a waiting report")
	}
	finished, err := database.GetActivityByID(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.WaitingSince != nil || finished.Status != "completed" {
		t.Fatalf("the finished run was modified: %#v", finished)
	}
}
