package db

import (
	"context"
	"errors"
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
	// refusal, when set, is what the tracker answers every label write with.
	refusal error
	// remote, when set, are the labels the tracker answers a read of the
	// ticket with; the recorder then reads tickets.
	remote []string
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
	return l.refusal
}

func (l *labelRecorder) GetIssue(ctx context.Context, req tracker.GetIssueRequest) (*models.Task, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.remote == nil {
		return l.BaseTicketingSystem.GetIssue(ctx, req)
	}
	return &models.Task{Key: req.Key, Labels: append([]string{}, l.remote...)}, nil
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

// The backlog lists the open tickets only: a finished one, by its workflow
// label or by its tracker status, stays out. A ticket of no project reads its
// stage through the tracker's own mapping, never a project's.
func TestABacklogLeavesTheFinishedTicketsOut(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery")
	trackerID := delivery.DefaultTrackerID
	if _, err := d.UpdateTrackerMirror(trackerID, func(trk *models.Tracker) {
		trk.TrackerColumns = []models.TrackerColumn{{Name: "Doing", Statuses: []string{"In Progress"}}, {Name: "Review", Statuses: []string{"In Review"}}, {Name: "Shipped", Statuses: []string{"Released"}}}
		trk.StageColumns = map[string][]string{"implemented": {"Doing"}, "finished": {"Shipped"}}
	}); err != nil {
		t.Fatal(err)
	}
	// Delivery's own mapping finishes Review: it does not apply out of the project.
	if err := setOwnMapping(d, delivery.ID, trackerID, map[string][]string{"finished": {"Review"}}); err != nil {
		t.Fatal(err)
	}
	ticket := func(key, trackerStatus string, labels ...string) models.Task {
		return models.Task{Key: key, Title: key, Status: models.StatusToClarify, Priority: models.PriorityMedium, Labels: labels,
			TrackerStatus: trackerStatus, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}
	if err := d.ImportOrUpdateTasks(trackerID, []models.Task{
		ticket("GODE-1", "In Progress"),
		ticket("GODE-2", "In Review", "ops"),
		ticket("GODE-3", "Released"),
		ticket("GODE-4", "Done"),
		ticket("GODE-5", "In Progress", "#finished"),
		ticket("GODE-6", "In Progress", "delivery"),
	}); err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(backlogKeys(t, d, trackerID), ","); got != "GODE-1,GODE-2" {
		t.Fatalf("backlog = %s, want the open tickets of no project only", got)
	}
}

// A backlog ticket carries the link to its ticket on the tracker, built from
// the tracker's own site or repository when the import recorded none, so the
// view can open it.
func TestABacklogTicketLinksToItsTracker(t *testing.T) {
	for _, tc := range []struct {
		provider, site, scope, key, want string
	}{
		{"jira", "https://acme.atlassian.net", "GODE", "GODE-7", "https://acme.atlassian.net/browse/GODE-7"},
		{"github", "", "acme/sectile", "#7", "https://github.com/acme/sectile/issues/7"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			d := testDB(t)
			trk, err := d.CreateTrackerAs("admin", models.Tracker{Provider: tc.provider, Site: tc.site, Scope: tc.scope})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Label: "delivery-admin", Trackers: []models.ProjectTracker{{TrackerID: trk.ID}}}); err != nil {
				t.Fatal(err)
			}
			if err := d.ImportOrUpdateTasks(trk.ID, []models.Task{{
				ID:        "7",
				Key:       tc.key,
				Title:     "Backlog",
				Status:    models.StatusToClarify,
				Priority:  models.PriorityMedium,
				Source:    tc.provider,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}}); err != nil {
				t.Fatal(err)
			}
			backlog, err := d.GetTrackerBacklog(trk.ID)
			if err != nil || len(backlog) != 1 {
				t.Fatalf("backlog = %d tickets (%v)", len(backlog), err)
			}
			if got := backlog[0].ExternalURL; got == nil || *got != tc.want {
				t.Fatalf("backlog ticket link = %v, want %s", got, tc.want)
			}
		})
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
	site := ""
	if provider == "jira" {
		site = "https://acme.atlassian.net"
	}
	trk, err := d.CreateTrackerAs("admin", models.Tracker{Provider: provider, Site: site, Scope: scope})
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

func TestARefusedLabelWriteTakesTheTicketBackOutOfTheProject(t *testing.T) {
	d := testDB(t)
	recorder := newLabelRecorder("jira")
	recorder.refusal = errors.New("label refused")
	d.TrackerRegistry().Register("jira", recorder)
	trk, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Site: "https://acme.atlassian.net", Scope: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Label: "delivery-admin", Trackers: []models.ProjectTracker{{TrackerID: trk.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ImportOrUpdateTasks(trk.ID, []models.Task{
		{ID: "5", Key: "GODE-5", Title: "Backlog", Labels: []string{"ops"}, Status: models.StatusToClarify, Priority: models.PriorityMedium, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "6", Key: "GODE-6", Title: "Already in", Labels: []string{"Delivery-Admin"}, Status: models.StatusToClarify, Priority: models.PriorityMedium, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	backlog, err := d.GetTrackerBacklog(trk.ID)
	if err != nil || len(backlog) != 1 {
		t.Fatalf("backlog = %d tickets (%v)", len(backlog), err)
	}
	task, activity, err := d.AddTaskToProjectAs(tracker.WithActingUser(context.Background(), "u-ada"), backlog[0].ID, project.ID)
	if err != nil || activity == nil || !labelCarried(task.Labels, "delivery-admin") {
		t.Fatalf("labelling: task %+v, activity %v, %v", task, activity, err)
	}
	recorder.written(t)

	var reverted *models.Task
	var act *models.TaskActivity
	for i := 0; i < 200; i++ {
		reverted, _ = d.GetTaskByID(task.ID)
		act, _ = d.GetActivityByID(activity.ID)
		if reverted != nil && !labelCarried(reverted.Labels, "delivery-admin") && act != nil && act.Status == string(models.ActivityStatusFailed) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if reverted == nil || labelCarried(reverted.Labels, "delivery-admin") || strings.Join(reverted.Labels, ",") != "ops" {
		t.Fatalf("after the refused write the ticket keeps labels %v, want ops alone", reverted)
	}
	if act == nil || act.Status != string(models.ActivityStatusFailed) {
		t.Fatalf("the refused write's activity = %+v, want failed", act)
	}
	if got := strings.Join(backlogKeys(t, d, trk.ID), ","); got != "GODE-5" {
		t.Fatalf("backlog = %s, want the ticket back in it", got)
	}

	again, activity, err := d.AddTaskToProjectAs(tracker.WithActingUser(context.Background(), "u-ada"), trk.Provider+"-"+trk.ID+"-GODE-6", project.ID)
	if err != nil || activity != nil || again == nil || strings.Join(again.Labels, ",") != "Delivery-Admin" {
		t.Fatalf("a ticket already carrying the label queues no write: task %+v, activity %v, %v", again, activity, err)
	}
}

// labelOpTicket opens a Jira space GODE that the project Delivery selects with
// its label, a recorder standing in for Jira, and the ticket GODE-5 carrying
// labels locally, with a failed or completed task_labels activity to report
// on. It returns the ticket's id and the operation adding Delivery's label.
func labelOpTicket(t *testing.T, recorder *labelRecorder, labels []string) (*DB, *models.Tracker, string, SkillJob) {
	t.Helper()
	d := testDB(t)
	d.TrackerRegistry().Register("jira", recorder)
	trk, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Site: "https://acme.atlassian.net", Scope: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Label: "delivery-admin", Trackers: []models.ProjectTracker{{TrackerID: trk.ID}}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ImportOrUpdateTasks(trk.ID, []models.Task{{ID: "5", Key: "GODE-5", Title: "Backlog", Labels: labels, Status: models.StatusToClarify,
		Priority: models.PriorityMedium, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	task, err := d.GetTaskByID("GODE-5")
	if err != nil || task == nil {
		t.Fatalf("GODE-5: %v", err)
	}
	activity := models.TaskActivity{ID: "labels-op", TaskID: task.ID, TaskKey: task.Key, SkillID: "tracker_op", Status: string(models.ActivityStatusQueued), CreatedAt: time.Now()}
	if err := d.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	op := TrackerOp{Kind: TrackerOpTaskLabels, TaskID: task.ID, TaskKey: task.Key, TrackerID: trk.ID, Labels: []string{"delivery-admin"}, UserID: "u-ada"}
	return d, trk, task.ID, SkillJob{ActivityID: activity.ID, TaskID: task.ID, SkillID: "tracker_op", Op: &op}
}

// A refused label write does not take back a label the tracker carries: a sync
// may have brought it in the meantime, or the write landed before its answer
// was lost. The tracker is read before the label is withdrawn.
func TestARefusedLabelWriteKeepsALabelTheTrackerCarries(t *testing.T) {
	recorder := newLabelRecorder("jira")
	recorder.Capabilities = append(recorder.Capabilities, tracker.CapGet)
	recorder.refusal = errors.New("label refused")
	recorder.remote = []string{"ops", "delivery-admin"}
	d, trk, taskID, job := labelOpTicket(t, recorder, []string{"ops", "delivery-admin"})

	d.processTrackerOpJob(context.Background(), job)

	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil || strings.Join(task.Labels, ",") != "ops,delivery-admin" {
		t.Fatalf("after a refused write of a label Jira carries, the ticket holds %+v (%v), want ops,delivery-admin", task, err)
	}
	if got := backlogKeys(t, d, trk.ID); len(got) != 0 {
		t.Fatalf("the ticket fell back into the backlog: %v", got)
	}

	recorder.remote = []string{"ops"}
	d.processTrackerOpJob(context.Background(), job)
	if task, _ = d.GetTaskByID(taskID); task == nil || strings.Join(task.Labels, ",") != "ops" {
		t.Fatalf("after a refused write of a label Jira lacks, the ticket holds %+v, want ops alone", task)
	}
}

// A label write that succeeds puts the label on the local ticket too, whatever
// took it away since it was queued, a revert of an earlier refusal or a sync
// that read the tracker before the write: the ticket joins its project at once
// rather than at the next sync.
func TestASuccessfulLabelWriteShowsTheLabelLocally(t *testing.T) {
	recorder := newLabelRecorder("jira")
	d, trk, taskID, job := labelOpTicket(t, recorder, []string{"ops"})

	d.processTrackerOpJob(context.Background(), job)

	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil || strings.Join(task.Labels, ",") != "ops,delivery-admin" {
		t.Fatalf("after the label was written on Jira, the ticket holds %+v (%v), want ops,delivery-admin", task, err)
	}
	if got := backlogKeys(t, d, trk.ID); len(got) != 0 {
		t.Fatalf("the ticket stays in the backlog: %v", got)
	}
	act, _ := d.GetActivityByID(job.ActivityID)
	if act == nil || act.Status != string(models.ActivityStatusCompleted) {
		t.Fatalf("the write's activity = %+v, want completed", act)
	}
}
