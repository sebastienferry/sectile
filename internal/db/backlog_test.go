package db

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// labelRecorder stands in for a tracker whose label writes are recorded: which
// ticket, which labels, on which tracker and as whom.
type labelRecorder struct {
	tracker.BaseTicketingSystem
	mu     sync.Mutex
	key    string
	added  []string
	on, as string
}

func newLabelRecorder(provider string) *labelRecorder {
	return &labelRecorder{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: provider,
		Capabilities: []tracker.Capability{tracker.CapSync, tracker.CapUpdate, tracker.CapLabels}}}
}

func (l *labelRecorder) FormatTaskID(trackerID, key, rawID string) string {
	return l.TrackerName + "-" + trackerID + "-" + key
}

func (l *labelRecorder) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.key, l.added, l.as = key, append([]string{}, add...), tracker.ActingUser(ctx)
	if trk := tracker.Tracker(ctx); trk != nil {
		l.on = trk.ID
	}
	return nil
}

// written waits for the queued label write and returns what it recorded.
func (l *labelRecorder) written(t *testing.T) (key string, added []string, on, as string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		l.mu.Lock()
		key, added, on, as = l.key, l.added, l.on, l.as
		l.mu.Unlock()
		if key != "" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the label was never written on the tracker")
	return
}

// backlogKeys lists the keys of a tracker's backlog, sorted.
func backlogKeys(t *testing.T, d *DB, trackerID string) []string {
	t.Helper()
	tasks, err := d.GetTrackerBacklog(trackerID)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, task := range tasks {
		keys = append(keys, task.Key)
	}
	sort.Strings(keys)
	return keys
}

func TestABacklogListsTheTicketsOfNoProject(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	spaceProject(t, d, "Bidder", "bidderAdmin")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{
		"GODE-1": {"delivery-admin"},
		"GODE-2": {"BIDDERADMIN"},
		"GODE-3": {},
		"GODE-4": {"ops"},
	})

	if got := strings.Join(backlogKeys(t, d, delivery.DefaultTrackerID), ","); got != "GODE-3,GODE-4" {
		t.Fatalf("backlog = %s, want the tickets carrying no project label", got)
	}
}

func TestAnUnlabelledProjectLeavesItsTrackerBacklogEmpty(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	spaceProject(t, d, "Everything", "")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{"GODE-1": {}, "GODE-2": {"ops"}})

	if got := backlogKeys(t, d, delivery.DefaultTrackerID); len(got) != 0 {
		t.Fatalf("backlog = %v, want none: the unlabelled project shows every ticket", got)
	}
}

func TestLabellingABacklogTicketShowsItInTheProjectAtOnce(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{"GODE-1": {"ops"}})
	backlog, err := d.GetTrackerBacklog(delivery.DefaultTrackerID)
	if err != nil || len(backlog) != 1 {
		t.Fatalf("backlog = %d tickets (%v)", len(backlog), err)
	}

	task, _, err := d.AddTaskToProjectAs(tracker.WithActingUser(context.Background(), "u-ada"), backlog[0].ID, delivery.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.ProjectID != delivery.ID || !labelCarried(task.Labels, "delivery-admin") {
		t.Fatalf("the labelled ticket shows project %q, labels %v", task.ProjectID, task.Labels)
	}
	if got := strings.Join(listedKeys(t, d, delivery.ID), ","); got != "GODE-1" {
		t.Fatalf("the project lists %s right after the labelling", got)
	}
	if got := backlogKeys(t, d, delivery.DefaultTrackerID); len(got) != 0 {
		t.Fatalf("the ticket stays in the backlog: %v", got)
	}

	everything := spaceProject(t, d, "Everything", "")
	if _, _, err := d.AddTaskToProjectAs(context.Background(), backlog[0].ID, everything.ID); err != ErrProjectWithoutLabel {
		t.Fatalf("labelling into an unlabelled project: %v", err)
	}
	other, err := d.CreateProject(models.CreateProjectRequest{Name: "Other", IssueTracker: "jira", JiraProject: "BE", Label: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.AddTaskToProjectAs(context.Background(), backlog[0].ID, other.ID); err != ErrProjectNotOnTracker {
		t.Fatalf("labelling into a project off the tracker: %v", err)
	}
}

// labelsWrittenOn labels a backlog ticket of a provider's tracker into a
// project, and checks the label went to that tracker, on that ticket, as the
// acting user.
func labelsWrittenOn(t *testing.T, provider, scope, key string) {
	t.Helper()
	d := testDB(t)
	recorder := newLabelRecorder(provider)
	d.TrackerRegistry().Register(provider, recorder)
	trk, err := d.CreateTrackerAs("admin", models.Tracker{Provider: provider, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Label: "delivery-admin", Trackers: []models.ProjectTracker{{TrackerID: trk.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ImportOrUpdateTasks(trk.ID, []models.Task{{
		ID:        "5",
		Key:       key,
		Title:     "Backlog",
		Status:    models.StatusToClarify,
		Priority:  models.PriorityMedium,
		Source:    provider,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}}); err != nil {
		t.Fatal(err)
	}
	backlog, err := d.GetTrackerBacklog(trk.ID)
	if err != nil || len(backlog) != 1 {
		t.Fatalf("backlog = %d tickets (%v)", len(backlog), err)
	}
	if _, activity, err := d.AddTaskToProjectAs(tracker.WithActingUser(context.Background(), "u-ada"), backlog[0].ID, project.ID); err != nil || activity == nil {
		t.Fatalf("labelling: activity %v, %v", activity, err)
	}
	gotKey, added, on, as := recorder.written(t)
	if gotKey != key || strings.Join(added, ",") != "delivery-admin" || on != trk.ID || as != "u-ada" {
		t.Fatalf("label write = key %q, added %v, tracker %q, as %q; want %s, delivery-admin, %s, u-ada", gotKey, added, on, as, key, trk.ID)
	}
}

func TestLabellingABacklogTicketWritesTheLabelOnJira(t *testing.T) {
	labelsWrittenOn(t, "jira", "GODE", "GODE-5")
}

func TestLabellingABacklogTicketWritesTheLabelOnGithub(t *testing.T) {
	labelsWrittenOn(t, "github", "acme/api", "#5")
}

func TestLabellingABacklogTicketWritesTheLabelOnGitlab(t *testing.T) {
	labelsWrittenOn(t, "gitlab", "acme/api", "#5")
}
