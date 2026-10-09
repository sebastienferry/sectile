package db

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// The loop used to queue one job per unfinished work item, every few minutes,
// for ever: four hundred cards meant four hundred reads a pass, four hundred
// activity rows, and a tracker asked four hundred times what one search
// answers. A pass now files one synchronisation per tracker (#741), and the
// tracker decides what has moved.
func TestABackgroundPassQueuesOneSynchronisationPerTrackerNotOnePerTicket(t *testing.T) {
	fake := newFakeTracker()
	database, project := autoSyncTestDB(t, fake)

	for _, key := range []string{"PE-1", "PE-2", "PE-3", "PE-4"} {
		seedTrackerTask(t, database, project.ID, key, "Open")
	}

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)

	activities, err := database.activitiesAttachedTo("tracker_id", defaultTrackerID(t, database, project.ID))
	if err != nil {
		t.Fatal(err)
	}
	queued := 0
	for _, act := range activities {
		if act.SkillID == "sync_jira" {
			queued++
		}
	}
	if queued != 1 {
		t.Fatalf("a pass files one synchronisation for the tracker, got %d", queued)
	}
}

// The first pass of a project has nothing to read back from, so it reads all of
// it. The next one only asks for what the tracker has touched since, with a
// margin: JQL reasons to the minute and clocks drift.
func TestTheFirstPassIsFullAndTheNextIsBoundedOnWhatMoved(t *testing.T) {
	fake := newFakeTracker()
	database, project := autoSyncTestDB(t, fake)
	seedTrackerTask(t, database, project.ID, "PE-1", "Open")

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)

	if window, ran := fake.syncedWithin(t); !ran || window != 0 {
		t.Fatalf("the first pass reads the whole project, got window %d (ran: %v)", window, ran)
	}
	// The full read is dated when it comes back, not when it is queued, so the
	// second pass has to wait for the first one to have finished.
	syncActivities(t, database, project.ID, false)

	// Wind the interval gate back so a second pass runs at all, and date the
	// previous one far enough for the window to be worth asserting on.
	setAutoSyncLastPass(t, database, project.ID, time.Now().UTC().Add(-10*time.Minute))

	fake.mu.Lock()
	fake.syncs = 0
	fake.mu.Unlock()

	database.runAutoSyncPass(settings)

	window, ran := fake.syncedWithin(t)
	if !ran {
		t.Fatal("the second pass never reached the tracker")
	}
	if window != 10+autoSyncOverlap {
		t.Fatalf("the second pass reads back to the previous one plus the overlap, got %d", window)
	}
}

// A window only ever narrows a read the tracker can narrow. One that cannot is
// asked for the whole project, which is still one request per hundred work
// items where the unit re-read cost one per work item.
func TestATrackerThatCannotNarrowASearchIsAskedForEverything(t *testing.T) {
	fake := newFakeTracker()
	fake.Capabilities = withoutCapability(fake.Capabilities, "incremental_sync")
	database, project := autoSyncTestDB(t, fake)
	seedTrackerTask(t, database, project.ID, "PE-1", "Open")

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)
	if _, ran := fake.syncedWithin(t); !ran {
		t.Fatal("the first pass never reached the tracker")
	}
	syncActivities(t, database, project.ID, false)

	setAutoSyncLastPass(t, database, project.ID, time.Now().UTC().Add(-10*time.Minute))
	fake.mu.Lock()
	fake.syncs = 0
	fake.mu.Unlock()

	database.runAutoSyncPass(settings)
	if window, ran := fake.syncedWithin(t); !ran || window != 0 {
		t.Fatalf("a tracker without the capability is asked for everything, got %d", window)
	}
}

// A synchronisation somebody asked for is never narrowed, whatever the loop
// last read: they clicked the button because they want the project as it is.
func TestASynchronisationSomebodyAskedForReadsTheWholeProject(t *testing.T) {
	fake := newFakeTracker()
	database, project := autoSyncTestDB(t, fake)

	if _, err := database.EnqueueSyncAs("u-ada", "jira", "", project.ID); err != nil {
		t.Fatal(err)
	}
	if window, ran := fake.syncedWithin(t); !ran || window != 0 {
		t.Fatalf("an explicit synchronisation reads everything, got %d (ran: %v)", window, ran)
	}
}

// autoSyncTestDB gives a database whose jira adapter is the fake one, on a
// project that opted into the background loop, with the loop's own state reset
// so a test starts from a project that has never been read.
func autoSyncTestDB(t *testing.T, fake *fakeTracker) (*DB, *models.Project) {
	t.Helper()
	database, project := jiraTestDB(t, fake)
	database.auto = &autoSync{}

	enabled := true
	project, err := database.UpdateProjectAs("u-ada", project.ID, models.UpdateProjectRequest{AutoSyncEnabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	return database, project
}

// setAutoSyncLastPass dates the previous pass of a project's tracker, as
// another pass or another instance would have (#741).
func setAutoSyncLastPass(t *testing.T, database *DB, projectID string, at time.Time) {
	t.Helper()
	if _, err := database.conn.Exec(`INSERT INTO auto_sync_trackers (tracker_id, last_pass_at) VALUES (?, ?)
		ON CONFLICT (tracker_id) DO UPDATE SET last_pass_at = excluded.last_pass_at`, defaultTrackerID(t, database, projectID), at); err != nil {
		t.Fatalf("dating the previous pass: %v", err)
	}
}

// autoSyncLastFull is when a project's tracker was last read in full, if ever.
func autoSyncLastFull(t *testing.T, database *DB, projectID string) (time.Time, bool) {
	t.Helper()
	var at sql.NullTime
	err := database.conn.QueryRow(`SELECT last_full_sync_at FROM auto_sync_trackers WHERE tracker_id = ?`, defaultTrackerID(t, database, projectID)).Scan(&at)
	if err == sql.ErrNoRows {
		return time.Time{}, false
	}
	if err != nil {
		t.Fatalf("reading the full-read date: %v", err)
	}
	return at.Time, at.Valid
}

func withoutCapability(caps []tracker.Capability, drop tracker.Capability) []tracker.Capability {
	kept := make([]tracker.Capability, 0, len(caps))
	for _, c := range caps {
		if c != drop {
			kept = append(kept, c)
		}
	}
	return kept
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

// syncActivities answers what the loop left on the feed of a project's
// tracker, once the worker has finished with it: a background pass that had
// nothing to report deletes its own row, so what is left can only be read
// after the job ran. wantOne keeps waiting until there is one, for the cases
// that expect a row to survive. A background pass is the tracker's, not the
// project's (#741).
func syncActivities(t *testing.T, database *DB, projectID string, wantOne bool) []models.TaskActivity {
	t.Helper()
	var last []models.TaskActivity
	trackerID := defaultTrackerID(t, database, projectID)
	for i := 0; i < 100; i++ {
		activities, err := database.activitiesAttachedTo("tracker_id", trackerID)
		if err != nil {
			t.Fatal(err)
		}
		last = nil
		pending := false
		for _, act := range activities {
			if act.SkillID != "sync_jira" {
				continue
			}
			if act.Status == string(models.ActivityStatusQueued) || act.Status == string(models.ActivityStatusRunning) {
				pending = true
			}
			last = append(last, act)
		}
		if !pending && (!wantOne || len(last) > 0) {
			return last
		}
		time.Sleep(30 * time.Millisecond)
	}
	return last
}

// A full read that the tracker refused is not a full read. Dating it would
// narrow every pass for the next half hour, on a project whose copy the
// failure just left incomplete.
func TestAFullPassThatFailedIsNotDated(t *testing.T) {
	fake := newFakeTracker()
	fake.syncErr = errors.New("configure the Jira account e-mail")
	database, project := autoSyncTestDB(t, fake)

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)
	if _, ran := fake.syncedWithin(t); !ran {
		t.Fatal("the first pass never reached the tracker")
	}
	syncActivities(t, database, project.ID, true)

	setAutoSyncLastPass(t, database, project.ID, time.Now().UTC().Add(-10*time.Minute))
	_, dated := autoSyncLastFull(t, database, project.ID)
	if dated {
		t.Fatal("a refused read must not count as the full pass it never was")
	}

	fake.mu.Lock()
	fake.syncs = 0
	fake.syncErr = nil
	fake.mu.Unlock()

	database.runAutoSyncPass(settings)
	if window, ran := fake.syncedWithin(t); !ran || window != 0 {
		t.Fatalf("the loop must keep asking for the whole project, got %d", window)
	}
}

// Two projects selecting one tracker are one source to read: the loop files
// one synchronisation for it, not one per project (#741).
func TestTwoProjectsOnOneTrackerQueueOneSynchronisation(t *testing.T) {
	fake := newFakeTracker()
	database, project := autoSyncTestDB(t, fake)
	enabled := true
	second, err := database.CreateProject(models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "PE", AutoSyncEnabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	if defaultTrackerID(t, database, second.ID) != defaultTrackerID(t, database, project.ID) {
		t.Fatal("the two projects do not share their tracker")
	}

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)
	database.jobs.drain(5 * time.Second)

	fake.mu.Lock()
	syncs := fake.syncs
	fake.mu.Unlock()
	if syncs != 1 {
		t.Fatalf("the shared tracker was read %d times, want once", syncs)
	}
}
