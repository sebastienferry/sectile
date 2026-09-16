package db

import (
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

func chainTestTask(t *testing.T, d *DB, project *models.Project) *models.Task {
	t.Helper()
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "chained"})
	if err != nil {
		t.Fatal(err)
	}
	if stage := d.StageOfTask(task); stage != "new" {
		t.Fatalf("a new task starts at stage %q, want new", stage)
	}
	return task
}

func runSummary(t *testing.T, d *DB, runID string) string {
	t.Helper()
	activity, err := d.GetActivityByID(runID)
	if err != nil || activity == nil {
		t.Fatalf("run %s not found: %v", runID, err)
	}
	return activity.Summary
}

// awaitSkillActivity waits for an activity of that skill to appear, since a
// chained step goes through the job queue before anything else happens to it.
func awaitSkillActivity(t *testing.T, d *DB, taskID, skillName string) bool {
	t.Helper()
	for i := 0; i < 50; i++ {
		activities, err := d.GetTaskActivities(taskID)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range activities {
			if a.SkillName == skillName {
				return true
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func countSkillActivities(t *testing.T, d *DB, taskID, name string) int {
	t.Helper()
	count := 0
	for _, a := range mustActivities(t, d, taskID) {
		if a.SkillName == name {
			count++
		}
	}
	return count
}

func skillName(t *testing.T, d *DB, skillID string) string {
	t.Helper()
	for _, s := range d.GetAvailableSkills() {
		if s.ID == skillID {
			return s.Name
		}
	}
	t.Fatalf("skill %s not found", skillID)
	return ""
}

// The chain's own contract: each step enqueues the next until the stop stage.
// Only the first step used to be enqueued, so a full chain run advanced the
// board exactly once and then stopped without saying anything.
func TestAFinishedChainStepEnqueuesTheNextOne(t *testing.T) {
	d, project := modeTestDB(t)
	task := chainTestTask(t, d, project)

	run, err := d.StartAgentRun(task.ID, "clarify", RunLaunch{Mode: models.SkillModeAutonomous, ChainStop: "reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "clarified", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(task.ID, run.ID, "completed", "Headless run finished"); err != nil {
		t.Fatal(err)
	}
	if !awaitSkillActivity(t, d, task.ID, skillName(t, d, "specify")) {
		t.Fatal("the step following clarified was never enqueued")
	}
}

// A run that ends having moved nothing is where a chain silently died. It now
// stops on a recorded reason instead, and the same reason is what tells a lone
// autonomous run apart from one that handed the workflow back.
func TestARunThatMovesNothingStopsTheChainAndSaysSo(t *testing.T) {
	d, project := modeTestDB(t)
	task := chainTestTask(t, d, project)

	run, err := d.StartAgentRun(task.ID, "clarify", RunLaunch{Mode: models.SkillModeAutonomous, ChainStop: "reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(task.ID, run.ID, "completed", "Headless run finished"); err != nil {
		t.Fatal(err)
	}
	summary := runSummary(t, d, run.ID)
	if !strings.Contains(summary, "Headless run finished") {
		t.Fatalf("the reason the process gave was dropped: %q", summary)
	}
	if !strings.Contains(summary, "without moving the task") || !strings.Contains(summary, "full chain stops here") {
		t.Fatalf("the run does not say it handed nothing back: %q", summary)
	}
	if awaitSkillActivity(t, d, task.ID, skillName(t, d, "specify")) {
		t.Fatal("a chain step was enqueued although the stage never moved")
	}
}

// The chain stops where it was told to, and says which stage that was.
func TestTheChainStopsAtItsStopStage(t *testing.T) {
	d, project := modeTestDB(t)
	task := chainTestTask(t, d, project)

	run, err := d.StartAgentRun(task.ID, "clarify", RunLaunch{Mode: models.SkillModeAutonomous, ChainStop: "clarified"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "clarified", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(task.ID, run.ID, "completed", "Headless run finished"); err != nil {
		t.Fatal(err)
	}
	if summary := runSummary(t, d, run.ID); !strings.Contains(summary, "stop stage") {
		t.Fatalf("the run does not say the chain reached its stop stage: %q", summary)
	}
	if awaitSkillActivity(t, d, task.ID, skillName(t, d, "specify")) {
		t.Fatal("the chain ran past its stop stage")
	}
}

// An interactive run is handed back by the user closing the session, so it is
// judged on nothing here. Neither is a skill whose whole point is not to move
// the task, nor a run recorded before any of this existed.
func TestOnlyAutonomousStageRunsAreJudged(t *testing.T) {
	d, project := modeTestDB(t)

	interactive := chainTestTask(t, d, project)
	run, err := d.StartAgentRun(interactive.ID, "clarify", RunLaunch{Mode: models.SkillModeInteractive})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(interactive.ID, run.ID, "completed", "Local console process exited"); err != nil {
		t.Fatal(err)
	}
	if summary := runSummary(t, d, run.ID); summary != "Local console process exited" {
		t.Fatalf("an interactive run was judged on a stage it never owned: %q", summary)
	}

	rewrite := chainTestTask(t, d, project)
	story, err := d.StartAgentRun(rewrite.ID, "rewrite_story", RunLaunch{Mode: models.SkillModeAutonomous})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(rewrite.ID, story.ID, "completed", "Headless run finished"); err != nil {
		t.Fatal(err)
	}
	if summary := runSummary(t, d, story.ID); summary != "Headless run finished" {
		t.Fatalf("a skill that owns no stage was reported as having moved nothing: %q", summary)
	}

	legacy := chainTestTask(t, d, project)
	old, err := d.StartAgentRemoteRun(legacy.ID, "clarify")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(legacy.ID, old.ID, "completed", "Headless run finished"); err != nil {
		t.Fatal(err)
	}
	if summary := runSummary(t, d, old.ID); summary != "Headless run finished" {
		t.Fatalf("a run launched before the mode was recorded was judged anyway: %q", summary)
	}
}

// finish_run is idempotent. A second report must not enqueue a second step.
func TestASecondFinishDoesNotChainTwice(t *testing.T) {
	d, project := modeTestDB(t)
	task := chainTestTask(t, d, project)

	run, err := d.StartAgentRun(task.ID, "clarify", RunLaunch{Mode: models.SkillModeAutonomous, ChainStop: "reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "clarified", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(task.ID, run.ID, "completed", "Headless run finished"); err != nil {
		t.Fatal(err)
	}
	if !awaitSkillActivity(t, d, task.ID, skillName(t, d, "specify")) {
		t.Fatal("the step following clarified was never enqueued")
	}
	// Counting the step by name, since the worker keeps adding records of its own
	// to the one step that was legitimately enqueued.
	before := countSkillActivities(t, d, task.ID, skillName(t, d, "specify"))
	if _, err := d.FinishRemoteRun(task.ID, run.ID, "completed", "Headless run finished"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if after := countSkillActivities(t, d, task.ID, skillName(t, d, "specify")); after != before {
		t.Fatalf("a repeated finish_run enqueued %d more step(s)", after-before)
	}
}
