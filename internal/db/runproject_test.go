package db

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// twoProjectTicket opens a database where GODE-1 carries the labels of two
// projects on the Jira space GODE, Delivery (created first) and Bidder.
func twoProjectTicket(t *testing.T) (*DB, *models.Project, *models.Project, *models.Task) {
	t.Helper()
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	bidder := spaceProject(t, d, "Bidder", "bidderAdmin")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{"GODE-1": {"delivery-admin", "bidderadmin"}, "GODE-2": {"bidderadmin"}, "GODE-3": {}})
	task, err := d.GetTaskByID("GODE-1")
	if err != nil || task == nil {
		t.Fatalf("GODE-1: %v", err)
	}
	return d, delivery, bidder, task
}

func ticketByKey(t *testing.T, d *DB, key string) *models.Task {
	t.Helper()
	task, err := d.GetTaskByID(key)
	if err != nil || task == nil {
		t.Fatalf("%s: %v", key, err)
	}
	return task
}

func TestARunFromAProjectBoardWorksForThatProject(t *testing.T) {
	d, _, bidder, task := twoProjectTicket(t)

	_, activity, err := d.EnqueueSkillOnTaskFor(task.ID, bidder.ID, "clarify", "", models.SkillModeUnset, "")
	if err != nil {
		t.Fatal(err)
	}
	if activity.RunProjectID != bidder.ID {
		t.Fatalf("the run works for %q, want the board's project %s", activity.RunProjectID, bidder.ID)
	}
	if _, _, err := d.EnqueueSkillOnTaskFor(ticketByKey(t, d, "GODE-3").ID, bidder.ID, "clarify", "", models.SkillModeUnset, ""); !errors.Is(err, ErrRunProjectNotMember) {
		t.Fatalf("a run for a project the ticket is not in: %v", err)
	}
}

func TestARunOnATicketOfOneProjectNeedsNoChoice(t *testing.T) {
	d, _, bidder, _ := twoProjectTicket(t)
	only := ticketByKey(t, d, "GODE-2")
	for _, unattended := range []bool{false, true} {
		projectID, err := d.ResolveRunProject(only, "", unattended)
		if err != nil || projectID != bidder.ID {
			t.Fatalf("unattended=%v: %q, %v; want Bidder", unattended, projectID, err)
		}
	}
}

func TestAnInteractiveRunOnATwoProjectTicketAsksWhichProject(t *testing.T) {
	d, delivery, bidder, task := twoProjectTicket(t)

	_, err := d.ResolveRunProject(task, "", false)
	var ambiguous *ErrRunProjectAmbiguous
	if !errors.As(err, &ambiguous) || ambiguous.Unattended {
		t.Fatalf("an interactive run on a two-project ticket: %v", err)
	}
	if len(ambiguous.Candidates) != 2 || ambiguous.Candidates[0].ID != delivery.ID || ambiguous.Candidates[1].ID != bidder.ID || ambiguous.Candidates[1].Name != "Bidder" {
		t.Fatalf("candidates = %+v", ambiguous.Candidates)
	}
	if _, err := d.StartRemoteRunFor("u-ada", task.ID, "clarify", "", ""); !errors.As(err, &ambiguous) {
		t.Fatalf("a session's run naming no project: %v", err)
	}
}

func TestAnAutonomousRunOnATwoProjectTicketIsRefusedWithTheCandidates(t *testing.T) {
	d, _, _, task := twoProjectTicket(t)

	_, _, err := d.EnqueueSkillOnTaskFor(task.ID, "", "clarify", "", models.SkillModeAutonomous, "")
	var ambiguous *ErrRunProjectAmbiguous
	if !errors.As(err, &ambiguous) || !ambiguous.Unattended || len(ambiguous.Candidates) != 2 {
		t.Fatalf("an autonomous run with no project: %v", err)
	}
	if !strings.Contains(err.Error(), "Delivery") || !strings.Contains(err.Error(), "Bidder") {
		t.Fatalf("the refusal names no candidate: %v", err)
	}
	if _, _, err := d.EnqueueFullChainRun(task.ID); !errors.As(err, &ambiguous) || !ambiguous.Unattended {
		t.Fatalf("a full chain with no project: %v", err)
	}
}

func TestABatchPickupOnATwoProjectTicketIsRefused(t *testing.T) {
	d, _, bidder, task := twoProjectTicket(t)

	_, err := d.StartAgentRun(task.ID, "pickup_issues", RunLaunch{Mode: models.SkillModeAutonomous, UserID: "u-ada"})
	var ambiguous *ErrRunProjectAmbiguous
	if !errors.As(err, &ambiguous) || !ambiguous.Unattended {
		t.Fatalf("a pickup with no project: %v", err)
	}
	if active, _, _ := d.ActiveBusyCause(task.ID); active != nil {
		t.Fatalf("the refused pickup left a run: %+v", active)
	}
	run, err := d.StartAgentRun(task.ID, "pickup_issues", RunLaunch{Mode: models.SkillModeAutonomous, UserID: "u-ada", ProjectID: bidder.ID})
	if err != nil || run.RunProjectID != bidder.ID {
		t.Fatalf("a pickup from Bidder's board: %+v, %v", run, err)
	}
}

func TestTheChosenProjectIsRecordedOnTheRun(t *testing.T) {
	d, delivery, bidder, task := twoProjectTicket(t)

	run, err := d.StartAgentRun(task.ID, "implement", RunLaunch{UserID: "u-ada", ProjectID: bidder.ID})
	if err != nil {
		t.Fatal(err)
	}
	read, err := d.GetActivityByID(run.ID)
	if err != nil || read == nil || read.RunProjectID != bidder.ID {
		t.Fatalf("read back = %+v (%v), want the run's project %s", read, err, bidder.ID)
	}
	history, err := d.GetTaskActivities(task.ID)
	if err != nil || len(history) == 0 || history[0].RunProjectID != bidder.ID {
		t.Fatalf("the task history reads %+v (%v)", history, err)
	}
	inBidder, _ := d.GetActivities(bidder.ID, "", "", "", "", 0)
	inDelivery, _ := d.GetActivities(delivery.ID, "", "", "", "", 0)
	found := func(list []models.TaskActivity) bool {
		for _, a := range list {
			if a.ID == run.ID {
				return true
			}
		}
		return false
	}
	if !found(inBidder) || found(inDelivery) {
		t.Fatalf("the run is listed in Bidder: %v, in Delivery: %v; want Bidder only", found(inBidder), found(inDelivery))
	}
	// While it runs, the ticket works for the run's project.
	if current := ticketByKey(t, d, task.ID); current.ProjectID != bidder.ID {
		t.Fatalf("the running ticket's project is %q, want %s", current.ProjectID, bidder.ID)
	}
}

func TestARunOnABacklogTicketIsRefused(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{"GODE-9": {"ops"}})
	backlog := ticketByKey(t, d, "GODE-9")

	if _, err := d.ResolveRunProject(backlog, "", false); !errors.Is(err, ErrTaskInNoProject) {
		t.Fatalf("a backlog ticket: %v", err)
	}
	if _, err := d.StartRemoteRunFor("u-ada", backlog.ID, "clarify", "", ""); !errors.Is(err, ErrTaskInNoProject) {
		t.Fatalf("a session run on a backlog ticket: %v", err)
	}
}

func TestCreatingATaskGoesToTheDefaultTrackerWithTheProjectLabel(t *testing.T) {
	d := testDB(t)
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Label: "delivery-admin"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "New"})
	if err != nil {
		t.Fatal(err)
	}
	if task.TrackerID != p.DefaultTrackerID || task.ProjectID != p.ID {
		t.Fatalf("created on tracker %q, project %q; want the default tracker %s", task.TrackerID, task.ProjectID, p.DefaultTrackerID)
	}
	if !labelCarried(task.Labels, "#new") || !labelCarried(task.Labels, "delivery-admin") {
		t.Fatalf("labels = %v, want #new and the project label", task.Labels)
	}
	var rowProject string
	if err := d.conn.QueryRow("SELECT project_id FROM tasks WHERE id = ?", task.ID).Scan(&rowProject); err != nil || rowProject != trackerSentinel(p.DefaultTrackerID) {
		t.Fatalf("project_id = %q (%v), want the tracker sentinel", rowProject, err)
	}
	second, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "Second"})
	if err != nil || second.Key == task.Key {
		t.Fatalf("the second local ticket: %+v, %v", second, err)
	}
	if got := strings.Join(listedKeys(t, d, p.ID), ","); got != strings.Join(sortedPair(task.Key, second.Key), ",") {
		t.Fatalf("the project lists %s", got)
	}
}

func sortedPair(a, b string) []string {
	if b < a {
		return []string{b, a}
	}
	return []string{a, b}
}

// trackerCreationRecorder stands in for a tracker that creates issues and records
// which tracker each creation named and with which labels.
type trackerCreationRecorder struct {
	tracker.BaseTicketingSystem
	mu     sync.Mutex
	on     string
	labels []string
	next   int
}

func (c *trackerCreationRecorder) FormatTaskID(trackerID, key, rawID string) string {
	return c.TrackerName + "-" + trackerID + "-" + key
}

func (c *trackerCreationRecorder) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	if req.Tracker != nil {
		c.on = req.Tracker.ID
	}
	c.labels = append([]string{}, req.Labels...)
	key := strings.ToUpper(req.Tracker.Scope) + "-" + string(rune('0'+c.next))
	return &models.Task{Key: key, ID: key}, nil
}

func TestCreatingATaskCanNameAnotherTrackerOfTheProject(t *testing.T) {
	d := testDB(t)
	fake := &trackerCreationRecorder{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "jira", Capabilities: []tracker.Capability{tracker.CapCreate}}}
	d.TrackerRegistry().Register("jira", fake)
	gode, be := jiraSpace(t, d, "GODE"), jiraSpace(t, d, "BE")
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Label: "delivery-admin", Trackers: []models.ProjectTracker{{TrackerID: gode.ID}, {TrackerID: be.ID}}})
	if err != nil {
		t.Fatal(err)
	}

	task, err := d.CreateTaskAs(tracker.WithActingUser(context.Background(), "u-ada"), models.CreateTaskRequest{ProjectID: p.ID, Title: "On BE", TrackerID: be.Identity, RequireRemoteCreation: true})
	if err != nil {
		t.Fatal(err)
	}
	if task.TrackerID != be.ID || fake.on != be.ID || !strings.HasPrefix(task.Key, "BE-") {
		t.Fatalf("created %s on tracker %q (adapter told %q), want BE", task.Key, task.TrackerID, fake.on)
	}
	if !labelCarried(fake.labels, "delivery-admin") || !labelCarried(fake.labels, "#new") {
		t.Fatalf("the tracker received labels %v", fake.labels)
	}
	if task.ProjectID != p.ID {
		t.Fatalf("the new ticket shows the project %q", task.ProjectID)
	}
}

func TestCreatingATaskOnATrackerOutsideTheProjectIsRefused(t *testing.T) {
	d := testDB(t)
	gode, be := jiraSpace(t, d, "GODE"), jiraSpace(t, d, "BE")
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", Trackers: []models.ProjectTracker{{TrackerID: gode.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "Elsewhere", TrackerID: be.ID}); !errors.Is(err, ErrTrackerNotInProject) {
		t.Fatalf("a creation on a tracker the project does not select: %v", err)
	}
}

func TestAgentConfigAcceptsAnyProjectTheTicketBelongsTo(t *testing.T) {
	d, delivery, bidder, task := twoProjectTicket(t)
	for _, p := range []*models.Project{delivery, bidder} {
		config, err := d.AgentConfig(p.ID, task.Key)
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		if config.ProjectID != p.ID || config.Tracker == nil || config.Tracker.ID != task.TrackerID || config.IssueTracker != "jira" || config.JiraProject != "GODE" {
			t.Fatalf("%s: config = project %q, tracker %+v, %s/%s", p.Name, config.ProjectID, config.Tracker, config.IssueTracker, config.JiraProject)
		}
		if config.Label != p.Label || len(config.Trackers) != 1 {
			t.Fatalf("%s: label %q, trackers %+v", p.Name, config.Label, config.Trackers)
		}
	}
	var ambiguous *ErrRunProjectAmbiguous
	if _, err := d.AgentConfig("", task.Key); !errors.As(err, &ambiguous) {
		t.Fatalf("no project for a two-project ticket: %v", err)
	}
	if _, err := d.StartAgentRun(task.ID, "implement", RunLaunch{ProjectID: bidder.ID}); err != nil {
		t.Fatal(err)
	}
	if config, err := d.AgentConfig("", task.Key); err != nil || config.ProjectID != bidder.ID {
		t.Fatalf("while a run works for Bidder: %+v, %v", config, err)
	}
}

func TestAgentConfigRefusesAProjectTheTicketIsNotIn(t *testing.T) {
	d, delivery, _, _ := twoProjectTicket(t)
	only := ticketByKey(t, d, "GODE-2")
	if _, err := d.AgentConfig(delivery.ID, only.Key); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("Delivery's configuration for a Bidder ticket: %v", err)
	}
}

func TestPrepareRepositoryWorktreeUsesTheRunsProject(t *testing.T) {
	d := testDB(t)
	delivery, err := d.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE", Label: "delivery-admin",
		Repositories: []string{"git@github.com:o/delivery.git"}})
	if err != nil {
		t.Fatal(err)
	}
	bidder, err := d.CreateProject(models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "GODE", Label: "bidderAdmin",
		Repositories: []string{"git@github.com:o/bidder.git", "git@github.com:o/bidder-ui.git"}})
	if err != nil {
		t.Fatal(err)
	}
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{"GODE-1": {"delivery-admin", "bidderadmin"}})
	task := ticketByKey(t, d, "GODE-1")
	if _, err := d.conn.Exec("UPDATE tasks SET branch_name = 'feat/GODE-1' WHERE id = ?", task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.StartAgentRun(task.ID, "implement", RunLaunch{ProjectID: bidder.ID}); err != nil {
		t.Fatal(err)
	}
	agent := &fakeWorktreeAgent{}
	d.SetAgentOperations(agent.call)

	if _, err := d.PrepareRepositoryWorktree(context.Background(), "", task.Key, "github.com/o/bidder-ui", ""); err != nil {
		t.Fatal(err)
	}
	if len(agent.calls) != 1 || agent.calls[0].ProjectID != bidder.ID {
		t.Fatalf("the worktree was asked for %+v, want the run's project %s", agent.calls, bidder.ID)
	}
}

// localTicket writes a ticket with a given key on a project's local tracker.
func localTicket(t *testing.T, d *DB, p *models.Project, id, key string) {
	t.Helper()
	if err := d.ImportOrUpdateTasks(p.DefaultTrackerID, []models.Task{{
		ID:        id,
		Key:       key,
		Title:     key,
		Status:    models.StatusToClarify,
		Priority:  models.PriorityMedium,
		Source:    "local",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}}); err != nil {
		t.Fatal(err)
	}
}

func TestAKeyResolvesWithinTheCallersProject(t *testing.T) {
	d := testDB(t)
	alpha, err := d.CreateProject(models.CreateProjectRequest{Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	beta, err := d.CreateProject(models.CreateProjectRequest{Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	localTicket(t, d, alpha, "alpha-task-1", "TASK-1")
	localTicket(t, d, beta, "beta-task-1", "TASK-1")

	if _, err := d.GetTaskByID("TASK-1"); !errors.Is(err, ErrTaskKeyAmbiguous) {
		t.Fatalf("TASK-1 with no project: %v, want the ambiguity", err)
	}
	for _, p := range []*models.Project{alpha, beta} {
		task, err := d.GetTaskByIDIn(p.ID, "TASK-1")
		if err != nil || task == nil || task.TrackerID != p.DefaultTrackerID || task.ProjectID != p.ID {
			t.Fatalf("TASK-1 in %s: %+v, %v", p.Name, task, err)
		}
	}
	if config, err := d.AgentConfig(beta.ID, "TASK-1"); err != nil || config.ProjectID != beta.ID {
		t.Fatalf("Beta's configuration for its TASK-1: %+v, %v", config, err)
	}
}

func TestAKeyTwoTrackersOfOneProjectCarryIsAmbiguous(t *testing.T) {
	d := testDB(t)
	api, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "github", Scope: "acme/api"})
	if err != nil {
		t.Fatal(err)
	}
	web, err := d.CreateTrackerAs("admin", models.Tracker{Provider: "github", Scope: "acme/web"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Acme", Trackers: []models.ProjectTracker{{TrackerID: api.ID}, {TrackerID: web.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	solo, err := d.CreateProject(models.CreateProjectRequest{Name: "Api", Trackers: []models.ProjectTracker{{TrackerID: api.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, trk := range []*models.Tracker{api, web} {
		if err := d.ImportOrUpdateTasks(trk.ID, []models.Task{{
			ID:        "12",
			Key:       "#12",
			Title:     trk.Scope,
			Status:    models.StatusToClarify,
			Priority:  models.PriorityMedium,
			Source:    "github",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}}); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := d.GetTaskByIDIn(p.ID, "#12"); !errors.Is(err, ErrTaskKeyAmbiguous) {
		t.Fatalf("#12 in a project of both repositories: %v, want the ambiguity", err)
	}
	task, err := d.GetTaskByIDIn(solo.ID, "#12")
	if err != nil || task == nil || task.TrackerID != api.ID {
		t.Fatalf("#12 in the project of acme/api only: %+v, %v", task, err)
	}
}
