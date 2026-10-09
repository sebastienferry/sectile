package db

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// batchWrite is one sprint or team write a batch sent, with the tracker its
// context named.
type batchWrite struct {
	kind, tracker, value string
	keys                 []string
}

// batchWriteTracker records the sprint and team writes of the queue worker.
type batchWriteTracker struct {
	tracker.BaseTicketingSystem
	mu     sync.Mutex
	writes []batchWrite
}

func (f *batchWriteTracker) record(ctx context.Context, kind, value string, keys []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	on := ""
	if t := tracker.Tracker(ctx); t != nil {
		on = t.ID
	}
	f.writes = append(f.writes, batchWrite{kind: kind, tracker: on, value: value, keys: slices.Clone(keys)})
}

func (f *batchWriteTracker) SetSprint(ctx context.Context, sprintID string, keys []string) error {
	f.record(ctx, "sprint", sprintID, keys)
	return nil
}

func (f *batchWriteTracker) SetTeam(ctx context.Context, key string, teamID string) error {
	f.record(ctx, "team", teamID, []string{key})
	return nil
}

// waitWrites waits for the queue worker to send n writes, and answers them.
func (f *batchWriteTracker) waitWrites(t *testing.T, n int) []batchWrite {
	t.Helper()
	for i := 0; i < 100; i++ {
		f.mu.Lock()
		got := slices.Clone(f.writes)
		f.mu.Unlock()
		if len(got) >= n {
			return got
		}
		time.Sleep(30 * time.Millisecond)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t.Fatalf("waited for %d writes, got %+v", n, f.writes)
	return nil
}

// twoJiraTrackersProject opens a project selecting the GODE then the BE space
// of one Jira site, GODE being its default tracker, with two tickets on each.
func twoJiraTrackersProject(t *testing.T) (*DB, *models.Project, *batchWriteTracker, *models.Tracker, *models.Tracker) {
	t.Helper()
	d := testDB(t)
	fake := &batchWriteTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "jira", Capabilities: []tracker.Capability{tracker.CapUpdate, tracker.CapSprint, tracker.CapTeam}}}
	d.TrackerRegistry().Register("jira", fake)
	gode, be := jiraSpace(t, d, "GODE"), jiraSpace(t, d, "BE")
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Trackers: []models.ProjectTracker{{TrackerID: gode.ID}, {TrackerID: be.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, trk := range []*models.Tracker{gode, be} {
		for _, n := range []string{"1", "2"} {
			key := trk.Scope + "-" + n
			if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title, status, priority, source, tracker_id) VALUES (?, ?, ?, 'T', 'backlog', 'medium', 'jira', ?)`,
				strings.ToLower(key), trackerSentinel(trk.ID), key, trk.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	return d, p, fake, gode, be
}

// A batch of tickets of a tracker other than the default one is written on
// that tracker, not on the default one the operation's project names (#741).
func TestABatchOnASecondTrackerIsWrittenThere(t *testing.T) {
	d, p, fake, _, be := twoJiraTrackersProject(t)
	ctx := tracker.WithActingUser(context.Background(), "u-ada")

	if _, err := d.SetTasksSprint(ctx, p.ID, []string{"be-1", "be-2"}, "", ""); err != nil {
		t.Fatal(err)
	}
	writes := fake.waitWrites(t, 1)
	if writes[0].tracker != be.ID || !slices.Equal(writes[0].keys, []string{"BE-1", "BE-2"}) {
		t.Fatalf("the sprint clear went %+v, want BE-1 and BE-2 on %s", writes[0], be.ID)
	}

	if _, err := d.SetTasksTeam(ctx, p.ID, []string{"be-1", "be-2"}, "team-1", "Team"); err != nil {
		t.Fatal(err)
	}
	writes = fake.waitWrites(t, 3)
	for _, w := range writes[1:] {
		if w.kind != "team" || w.tracker != be.ID {
			t.Fatalf("a team write went %+v, want it on %s", w, be.ID)
		}
	}
}

// A batch spanning two trackers is split: one sprint call per tracker, and
// each team write on its ticket's tracker (#741).
func TestABatchOnTwoTrackersIsSplitPerTracker(t *testing.T) {
	d, p, fake, gode, be := twoJiraTrackersProject(t)
	ctx := tracker.WithActingUser(context.Background(), "u-ada")

	if _, err := d.SetTasksSprint(ctx, p.ID, []string{"gode-1", "be-1", "gode-2", "be-2"}, "", ""); err != nil {
		t.Fatal(err)
	}
	writes := fake.waitWrites(t, 2)
	want := []batchWrite{
		{kind: "sprint", tracker: gode.ID, keys: []string{"GODE-1", "GODE-2"}},
		{kind: "sprint", tracker: be.ID, keys: []string{"BE-1", "BE-2"}},
	}
	for i, w := range want {
		if writes[i].kind != w.kind || writes[i].tracker != w.tracker || !slices.Equal(writes[i].keys, w.keys) {
			t.Fatalf("sprint calls = %+v, want %+v", writes, want)
		}
	}

	if _, err := d.SetTasksTeam(ctx, p.ID, []string{"gode-1", "be-1"}, "team-1", "Team"); err != nil {
		t.Fatal(err)
	}
	writes = fake.waitWrites(t, 4)
	if writes[2].tracker != gode.ID || writes[2].keys[0] != "GODE-1" || writes[3].tracker != be.ID || writes[3].keys[0] != "BE-1" {
		t.Fatalf("team writes = %+v", writes[2:])
	}
}

// A sprint belongs to the board of the project's default tracker: setting it
// on a batch holding a ticket of another tracker is refused, and nothing is
// written, locally or on a tracker (#741).
func TestASprintOfTheDefaultBoardIsRefusedOnAnotherTrackersTicket(t *testing.T) {
	d, p, fake, _, _ := twoJiraTrackersProject(t)
	ctx := tracker.WithActingUser(context.Background(), "u-ada")

	if _, err := d.SetTasksSprint(ctx, p.ID, []string{"gode-1", "be-1"}, "42", "Sprint 42"); err == nil || !strings.Contains(err.Error(), "BE-1") {
		t.Fatalf("a default-board sprint on a BE ticket: %v", err)
	}
	for _, id := range []string{"gode-1", "be-1"} {
		if task, err := d.GetTaskByID(id); err != nil || task == nil || task.Sprint != "" {
			t.Fatalf("%s was changed locally: %+v (%v)", id, task, err)
		}
	}
	var activities int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM task_activities`).Scan(&activities); err != nil {
		t.Fatal(err)
	}
	if activities != 0 {
		t.Fatalf("%d activities were queued", activities)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.writes) != 0 {
		t.Fatalf("the tracker received %+v", fake.writes)
	}
}
