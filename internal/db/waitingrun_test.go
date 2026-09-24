package db

import (
	"errors"
	"testing"
	"time"

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

// A second waiting report must not restart the clock: the board shows how long
// the owner has been waited for, and that began with the first report.
func TestFirstWaitingMarkWins(t *testing.T) {
	database, run := startedRun(t)
	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	first, _ := database.GetActivityByID(run.ID)
	time.Sleep(20 * time.Millisecond)
	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	second, _ := database.GetActivityByID(run.ID)
	if first.WaitingSince == nil || second.WaitingSince == nil || !second.WaitingSince.Equal(*first.WaitingSince) {
		t.Fatalf("waitingSince moved from %v to %v on a repeated report", first.WaitingSince, second.WaitingSince)
	}
}

// Declaring a wait follows the ownership rule of finish_run: the owner, an
// admin, or anyone on a run nobody owns.
func TestReportWaitingFollowsRunOwnership(t *testing.T) {
	d := openRolesDB(t)
	owner, _ := d.SignInLocal("alice@example.com")
	other, _ := d.SignInLocal("bob@example.com")
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Asks a question", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartRemoteRunBy(owner.ID, task.ID, "clarify", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.ReportRemoteRunWaitingAs(Actor{ID: other.ID}, false, task.ID, run.ID, true); !errors.Is(err, ErrRunNotYours) {
		t.Fatalf("a colleague marked another user's run waiting: %v", err)
	}
	if still, _ := d.GetActivityByID(run.ID); still.WaitingSince != nil {
		t.Fatal("the refused report marked the run anyway")
	}
	activity, applied, err := d.ReportRemoteRunWaitingAs(Actor{ID: owner.ID}, false, task.ID, run.ID, true)
	if err != nil || !applied || activity.WaitingSince == nil {
		t.Fatalf("the owner could not mark their run: %v, applied=%v", err, applied)
	}
	if _, applied, err := d.ReportRemoteRunWaitingAs(Actor{ID: other.ID}, true, task.ID, run.ID, false); err != nil || !applied {
		t.Fatalf("an admin could not clear a wait: %v", err)
	}
	legacy, _ := d.StartRemoteRun(task.ID, "clarify", "")
	if _, applied, err := d.ReportRemoteRunWaitingAs(Actor{ID: other.ID}, false, task.ID, legacy.ID, true); err != nil || !applied {
		t.Fatalf("an ownerless run refused a wait: %v", err)
	}
}

// A headless run has nobody to answer it: the report is accepted, so a skill
// written for both modes does not fail, but nothing is shown.
func TestReportWaitingIgnoresAHeadlessRun(t *testing.T) {
	database := testDB(t)
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Headless", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartAgentRun(task.ID, "clarify", RunLaunch{Mode: models.SkillModeAutonomous})
	if err != nil {
		t.Fatal(err)
	}
	activity, applied, err := database.ReportRemoteRunWaitingAs(Actor{}, false, task.ID, run.ID, true)
	if err != nil || applied {
		t.Fatalf("a headless wait: err=%v applied=%v", err, applied)
	}
	if activity.WaitingSince != nil {
		t.Fatal("a headless run was shown as waiting")
	}
	if stored, _ := database.GetActivityByID(run.ID); stored.WaitingSince != nil {
		t.Fatal("a headless run was stored as waiting")
	}
}

func TestReportWaitingRefusesAnotherTaskOrAFinishedRun(t *testing.T) {
	database, run := startedRun(t)
	other, err := database.CreateTask(models.CreateTaskRequest{Title: "Unrelated", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := database.ReportRemoteRunWaitingAs(Actor{}, true, other.ID, run.ID, true); err == nil {
		t.Fatal("a run was marked waiting through a task it does not belong to")
	}
	if _, err := database.FinishRemoteRun(run.TaskID, run.ID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := database.ReportRemoteRunWaitingAs(Actor{}, true, run.TaskID, run.ID, true); err == nil {
		t.Fatal("a finished run accepted a wait")
	}
}
