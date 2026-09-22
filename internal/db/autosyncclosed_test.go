package db

import (
	"testing"
	"time"

	"tasks/internal/models"
)

// What "closed" means belongs to the deployment, not to a word list. A project
// that has imported its board already says which columns land on the finished
// stage, and that answer wins: the column may be called anything at all.
func TestADeclaredFinishedColumnDecidesWhatClosedMeans(t *testing.T) {
	proj := &models.Project{
		TrackerColumns: []models.TrackerColumn{
			{Name: "En cours", Statuses: []string{"In Progress", "Dev Test"}},
			{Name: "Sorti", Statuses: []string{"Livré", "Rollback"}},
		},
		StageColumns: map[string][]string{
			"new":      {"En cours"},
			"finished": {"Sorti"},
		},
	}

	for _, status := range []string{"Livré", "livré", "Rollback"} {
		if !closedTrackerStatus(proj, status) {
			t.Fatalf("%q lands on the finished column and must read as closed", status)
		}
	}
	for _, status := range []string{"In Progress", "Dev Test"} {
		if closedTrackerStatus(proj, status) {
			t.Fatalf("%q is a working status and must not read as closed", status)
		}
	}

	// A board whose columns carry no status list still names them, and on
	// GitHub the column and the status are the same word.
	bare := &models.Project{StageColumns: map[string][]string{"finished": {"closed"}}}
	if !closedTrackerStatus(bare, "Closed") {
		t.Fatal("a column name is an answer when the column lists no status")
	}
}

// Every project imported before the column mapping existed declares nothing,
// and so does every tracker with no board. The name list is what is left.
func TestAnUndeclaredProjectFallsBackOnTheStatusName(t *testing.T) {
	proj := &models.Project{}

	for _, status := range []string{"Closed", "done", "Terminé", "Won't Do", "Annulé", "RESOLVED"} {
		if !closedTrackerStatus(proj, status) {
			t.Fatalf("%q names a closed work item", status)
		}
	}
	// The costly mistake is the other one: a status claimed wrongly freezes a
	// live ticket. "To Deploy" and "Blocked" are the two this loop used to see
	// most, and neither is an ending.
	for _, status := range []string{"Open", "In Progress", "To Deploy", "Blocked", "Dev Test", ""} {
		if closedTrackerStatus(proj, status) {
			t.Fatalf("%q is not an ending", status)
		}
	}

	// Nothing resolves a project that is not there, which is what a task with
	// no project would hand us.
	if !closedTrackerStatus(nil, "Closed") {
		t.Fatal("the name list answers even with no project to ask")
	}
}

// The loop wrote one activity per unfinished ticket per pass, on the ticket's
// own fiche, and a ticket the tracker had closed counted as unfinished for as
// long as its workflow label said anything but #finished. Re-reading it every
// few minutes bought nothing.
func TestTheBackgroundPassLeavesAClosedTicketAloneUntilTheSweep(t *testing.T) {
	fake := newFakeTracker()
	database, project := jiraTestDB(t, fake)
	database.auto = &autoSync{lastFullSync: map[string]time.Time{}, lastPassAt: map[string]time.Time{}, lastClosedSweep: map[string]time.Time{}}

	enabled := true
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{AutoSyncEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}

	seedTrackerTask(t, database, project.ID, "PE-1", "Open")
	seedTrackerTask(t, database, project.ID, "PE-2", "Closed")

	settings, _ := database.GetSettings()

	// What the pass queued, counted on the pass itself: the activity rows it
	// writes are picked up by the worker straight away, and a row the worker
	// drops for having nothing to report cannot be counted afterwards.
	queuedByAPass := func() int {
		t.Helper()
		database.auto.mu.Lock()
		// The interval gate is the project's own; it is wound back here so a
		// second pass runs at all.
		database.auto.lastPassAt[project.ID] = time.Now().Add(-time.Hour)
		database.auto.mu.Unlock()

		database.runAutoSyncPass(settings)

		database.auto.mu.Lock()
		defer database.auto.mu.Unlock()
		return database.auto.lastImported
	}

	// The first pass of a project has no sweep behind it, so it reads
	// everything: a fresh start must not inherit a blind spot.
	if got := queuedByAPass(); got != 2 {
		t.Fatalf("the first pass reads both tickets, got %d", got)
	}

	for i := 0; i < 3; i++ {
		if got := queuedByAPass(); got != 1 {
			t.Fatalf("only the open ticket is read by an ordinary pass, got %d", got)
		}
	}

	// But the closed one is not dropped: a ticket reopens, and the spaced sweep
	// is what notices. Nothing else in the loop would.
	database.auto.mu.Lock()
	database.auto.lastClosedSweep[project.ID] = time.Now().Add(-2 * autoSyncClosedSweepEvery)
	database.auto.mu.Unlock()
	if got := queuedByAPass(); got != 2 {
		t.Fatalf("the sweep must come back to the closed ticket, got %d", got)
	}
}

func seedTrackerTask(t *testing.T, database *DB, projectID, key, trackerStatus string) string {
	t.Helper()
	task, err := database.CreateTask(models.CreateTaskRequest{Title: key, Source: "local", ProjectID: projectID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.conn.Exec("UPDATE tasks SET source='jira', key=?, tracker_status=? WHERE id=?", key, trackerStatus, task.ID); err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func countSyncActivities(t *testing.T, database *DB, taskID string) int {
	t.Helper()
	activities, err := database.GetTaskActivities(taskID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, act := range activities {
		if act.SkillID == "sync_task" {
			count++
		}
	}
	return count
}
