package db

import (
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

// twoInstances opens two stores on one database, as two server instances
// sharing it would, both reaching the same fake tracker, on a project that
// opted into the background loop.
func twoInstances(t *testing.T, fake *fakeTracker) (*DB, *DB, *models.Project) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.db")
	first, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Close() })
	first.TrackerRegistry().Register("jira", fake)
	project, err := first.CreateProject(models.CreateProjectRequest{Name: "Platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	if project, err = first.UpdateProject(project.ID, models.UpdateProjectRequest{AutoSyncEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}

	second, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { second.Close() })
	second.TrackerRegistry().Register("jira", fake)
	return first, second, project
}

// Two instances running their pass on the same due project queue one
// synchronisation between them, not one each.
func TestTwoInstancesQueueOneSynchronisationPerDueProject(t *testing.T) {
	fake := newFakeTracker()
	first, second, _ := twoInstances(t, fake)

	settings, _ := first.GetSettings()
	first.runAutoSyncPass(settings)
	second.runAutoSyncPass(settings)
	first.jobs.drain(5 * time.Second)
	second.jobs.drain(5 * time.Second)

	fake.mu.Lock()
	syncs := fake.syncs
	fake.mu.Unlock()
	if syncs != 1 {
		t.Fatalf("two instances synchronised the project %d times, want once", syncs)
	}
	if status := second.AutoSyncStatus(); status.Passes != 2 || status.LastRunAt == "" {
		t.Errorf("the status counts both passes wherever it is read: %+v", status)
	}
}

// A claim is refused while the project's interval has not elapsed since the
// previous one, whoever made it, and granted afterwards with the pacing as it
// stood.
func TestAClaimIsRefusedWithinTheIntervalAndGrantedAfter(t *testing.T) {
	first, second, project := twoInstances(t, newFakeTracker())
	interval := 5 * time.Minute
	start := time.Now().UTC()

	pacing, claimed, err := first.claimAutoSyncPass(project.ID, interval, start)
	if err != nil || !claimed {
		t.Fatalf("the first claim of a project never read must succeed: %v %v", claimed, err)
	}
	if !pacing.lastPass.IsZero() || !pacing.lastFull.IsZero() {
		t.Fatalf("a project never read has no pacing: %+v", pacing)
	}

	if _, claimed, err := second.claimAutoSyncPass(project.ID, interval, start.Add(time.Minute)); err != nil || claimed {
		t.Fatalf("a claim within the interval must be refused: %v %v", claimed, err)
	}

	pacing, claimed, err = second.claimAutoSyncPass(project.ID, interval, start.Add(6*time.Minute))
	if err != nil || !claimed {
		t.Fatalf("a claim after the interval must succeed: %v %v", claimed, err)
	}
	if pacing.lastPass.Sub(start).Abs() > time.Second {
		t.Errorf("the pacing read is the previous claim's: got %v, want %v", pacing.lastPass, start)
	}
}

// A tracker that answered 429 to one instance is the same tracker for all of
// them: every loop steps back, and every status says so.
func TestABackoffEnteredByOneInstanceHoldsForAll(t *testing.T) {
	first, second, _ := twoInstances(t, newFakeTracker())

	first.enterAutoSyncBackoff()

	if until := second.autoSyncBackoffUntil(); !time.Now().UTC().Before(until) {
		t.Fatalf("the other instance does not see the backoff: %v", until)
	}
	if status := second.AutoSyncStatus(); status.BackoffUntil == "" {
		t.Errorf("the other instance's status omits the backoff: %+v", status)
	}
}

// A full read one instance recorded narrows the next pass another instance
// makes, and what it imported shows in every status.
func TestAFullReadDatedByOneInstanceNarrowsTheOthersNextWindow(t *testing.T) {
	first, second, project := twoInstances(t, newFakeTracker())

	first.recordAutoSyncPass(project.ID, 0, 3, false, "")
	if _, dated := autoSyncLastFull(t, second, project.ID); !dated {
		t.Fatal("the full read is not dated for the other instance")
	}
	setAutoSyncLastPass(t, first, project.ID, time.Now().UTC().Add(-10*time.Minute))

	now := time.Now().UTC()
	pacing, claimed, err := second.claimAutoSyncPass(project.ID, 5*time.Minute, now)
	if err != nil || !claimed {
		t.Fatalf("claiming: %v %v", claimed, err)
	}
	if window := autoSyncWindowFrom(pacing, now); window != 10+autoSyncOverlap {
		t.Errorf("window = %d, want %d", window, 10+autoSyncOverlap)
	}
	if status := second.AutoSyncStatus(); status.LastImported != 3 || status.Imported != 3 {
		t.Errorf("the other instance's status misses the import: %+v", status)
	}
}

func TestAutoSyncWindowFrom(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name   string
		pacing autoSyncPacing
		want   int
	}{
		{"never read", autoSyncPacing{}, 0},
		{"never read in full", autoSyncPacing{lastPass: now.Add(-5 * time.Minute)}, 0},
		{"full read too old", autoSyncPacing{lastPass: now.Add(-5 * time.Minute), lastFull: now.Add(-autoSyncFullEvery - time.Minute)}, 0},
		{"no previous pass", autoSyncPacing{lastFull: now.Add(-time.Minute)}, 0},
		{"incremental", autoSyncPacing{lastPass: now.Add(-7 * time.Minute), lastFull: now.Add(-10 * time.Minute)}, 7 + autoSyncOverlap},
		{"beyond the maximum window", autoSyncPacing{lastPass: now.Add(-25 * time.Hour), lastFull: now.Add(-time.Minute)}, 0},
	} {
		if got := autoSyncWindowFrom(tc.pacing, now); got != tc.want {
			t.Errorf("%s: window = %d, want %d", tc.name, got, tc.want)
		}
	}
}
