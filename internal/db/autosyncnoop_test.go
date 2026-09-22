package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"tasks/internal/models"
)

// runBackgroundSync queues the activity the way the pass does, then runs the
// job here rather than through the worker: the worker would run it in parallel
// and race whatever the test is watching.
func runBackgroundSync(t *testing.T, database *DB, taskID string) {
	t.Helper()
	task, err := database.GetTaskByID(taskID)
	if err != nil || task == nil {
		t.Fatalf("seeded task unreadable: %v", err)
	}
	activity, err := database.EnqueueSingleTaskSyncAs("u-ada", task)
	if err != nil {
		t.Fatal(err)
	}
	database.processSyncTaskJob(context.Background(), SkillJob{
		SkillID: "sync_task", ActivityID: activity.ID, TaskID: task.ID, ProjectID: task.ProjectID, ActingUser: "u-ada",
	})
}

// The background pass re-reads every unfinished work item every few minutes,
// and almost every read finds the ticket exactly as it left it. One row per
// read, on the ticket's own card, buried every real entry: on a four hundred
// ticket project, ninety-six percent of a card's history said nothing but
// "synchronised successfully".
func TestABackgroundReadThatChangedNothingLeavesNoTrace(t *testing.T) {
	fake := newFakeTracker()
	database, project := jiraTestDB(t, fake)
	taskID := seedTrackerTask(t, database, project.ID, "PE-1", "Open")

	// The tracker answers with the work item as Sectile already holds it.
	stored, err := database.GetTaskByID(taskID)
	if err != nil {
		t.Fatal(err)
	}
	fake.tasks = []models.Task{*stored}

	runBackgroundSync(t, database, taskID)

	if got := countSyncActivities(t, database, taskID); got != 0 {
		t.Fatalf("a read that changed nothing must leave nothing behind, got %d activity(ies)", got)
	}
}

// What it must not do is go quiet about a read that found something. That is
// the entry the card exists for.
func TestABackgroundReadThatChangedTheTicketIsKept(t *testing.T) {
	fake := newFakeTracker()
	database, project := jiraTestDB(t, fake)
	taskID := seedTrackerTask(t, database, project.ID, "PE-1", "Open")

	stored, err := database.GetTaskByID(taskID)
	if err != nil {
		t.Fatal(err)
	}
	moved := *stored
	moved.TrackerStatus = "In Progress"
	moved.Assignee = "Ada"
	fake.tasks = []models.Task{moved}

	runBackgroundSync(t, database, taskID)

	activities, err := database.GetTaskActivities(taskID)
	if err != nil {
		t.Fatal(err)
	}
	kept := 0
	for _, act := range activities {
		if act.SkillID != "sync_task" {
			continue
		}
		kept++
		if act.Status != string(models.ActivityStatusCompleted) {
			t.Fatalf("a read that found a change completes, got %q", act.Status)
		}
	}
	if kept != 1 {
		t.Fatalf("the change must be recorded once, got %d activity(ies)", kept)
	}
}

// And a refusal is never dropped. It is the one outcome nobody can reconstruct
// afterwards, and a credential the tracker refuses is exactly what these rows
// get read for.
func TestAFailedBackgroundReadIsAlwaysKept(t *testing.T) {
	fake := newFakeTracker()
	database, project := jiraTestDB(t, fake)
	taskID := seedTrackerTask(t, database, project.ID, "PE-1", "Open")
	fake.getErr = errors.New("configure the Jira account e-mail")

	runBackgroundSync(t, database, taskID)

	activities, err := database.GetTaskActivities(taskID)
	if err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, act := range activities {
		if act.SkillID != "sync_task" {
			continue
		}
		failed++
		if act.Status != string(models.ActivityStatusFailed) {
			t.Fatalf("a refused read fails, got %q", act.Status)
		}
		if act.UserID != "u-ada" {
			t.Fatalf("the failure must say whose credential was refused, got %q", act.UserID)
		}
	}
	if failed != 1 {
		t.Fatalf("the refusal must be kept, got %d activity(ies)", failed)
	}
}

// The comparison reads the tracker's own fields, and only those. A reordering
// of the labels is not a change; a timestamp Sectile bumps on every write is
// not one either, which is what would have made the whole thing pointless.
func TestTrackerFactsIgnoreWhatTheTrackerDoesNotOwn(t *testing.T) {
	updated := time.Now().UTC()
	base := models.Task{
		Key: "PE-1", Title: "Titre", TrackerStatus: "Open", Priority: models.PriorityHigh,
		Labels: []string{"#new", "backend"}, TrackerUpdatedAt: &updated,
	}

	reordered := base
	reordered.Labels = []string{"backend", "#new"}
	reordered.UpdatedAt = time.Now().Add(time.Hour)
	reordered.Position = base.Position + 10
	if trackerFacts(&base) != trackerFacts(&reordered) {
		t.Fatal("neither the label order nor a local write is a change")
	}

	for _, changed := range []func(*models.Task){
		func(task *models.Task) { task.Title = "Autre" },
		func(task *models.Task) { task.TrackerStatus = "Closed" },
		func(task *models.Task) { task.Priority = models.PriorityLow },
		func(task *models.Task) { task.Labels = append(task.Labels, "#clarified") },
		func(task *models.Task) { task.Assignee = "Ada" },
		func(task *models.Task) { later := updated.Add(time.Minute); task.TrackerUpdatedAt = &later },
	} {
		moved := base
		moved.Labels = append([]string(nil), base.Labels...)
		changed(&moved)
		if trackerFacts(&base) == trackerFacts(&moved) {
			t.Fatalf("a tracker field that moved must read as a change: %+v", moved)
		}
	}
}
