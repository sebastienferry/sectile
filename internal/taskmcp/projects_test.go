package taskmcp

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	sectiletracker "tasks/internal/tracker"
)

func openDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// sharedTicket opens a database where GODE-1 carries the labels of two projects
// on the Jira space GODE (#741).
func sharedTicket(t *testing.T) (*db.DB, *models.Project, *models.Project) {
	t.Helper()
	database := openDB(t)
	delivery, err := database.CreateProject(models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "GODE", Label: "delivery-admin"})
	if err != nil {
		t.Fatal(err)
	}
	bidder, err := database.CreateProject(models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "GODE", Label: "bidderAdmin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.ImportOrUpdateTasks(delivery.DefaultTrackerID, []models.Task{{Key: "GODE-1", Title: "Shared", Status: models.StatusToClarify, Priority: models.PriorityMedium,
		Labels: []string{"delivery-admin", "bidderadmin"}, Source: "jira", CreatedAt: time.Now(), UpdatedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	return database, delivery, bidder
}

func TestTheMcpStartRunTakesAProjectId(t *testing.T) {
	database, _, bidder := sharedTicket(t)

	_, err := call(t, database, "start_run", map[string]any{"taskKey": "GODE-1", "skill": "clarify"})
	if err == nil || !strings.Contains(err.Error(), "several projects") || !strings.Contains(err.Error(), bidder.ID) {
		t.Fatalf("a run on a two-project task naming no project: %v", err)
	}
	if strings.Contains(err.Error(), "plusieurs") {
		t.Fatalf("the refusal reached the session in French: %v", err)
	}
	activity, err := call(t, database, "start_run", map[string]any{"taskKey": "GODE-1", "skill": "clarify", "projectId": bidder.ID})
	if err != nil {
		t.Fatal(err)
	}
	if activity["runProjectId"] != bidder.ID {
		t.Fatalf("the run works for %v, want %s", activity["runProjectId"], bidder.ID)
	}
}

func TestGetProjectContextListsTheProjectsTrackersAndLabel(t *testing.T) {
	database, delivery, _ := sharedTicket(t)

	projectContext, err := call(t, database, "get_project_context", map[string]any{"projectId": delivery.ID})
	if err != nil {
		t.Fatal(err)
	}
	trackers, _ := projectContext["trackers"].([]any)
	if len(trackers) != 1 || projectContext["label"] != "delivery-admin" || projectContext["jiraProject"] != "GODE" {
		t.Fatalf("context = trackers %v, label %v, jiraProject %v", projectContext["trackers"], projectContext["label"], projectContext["jiraProject"])
	}
	first, _ := trackers[0].(map[string]any)
	if first["id"] != delivery.DefaultTrackerID || first["scope"] != "GODE" {
		t.Fatalf("tracker = %v", first)
	}
	if _, ok := projectContext["tracker"]; ok {
		t.Fatal("a project's context names no task tracker")
	}
	withTask, err := call(t, database, "get_project_context", map[string]any{"projectId": delivery.ID, "taskKey": "GODE-1"})
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := withTask["tracker"].(map[string]any); task == nil || task["id"] != delivery.DefaultTrackerID {
		t.Fatalf("the task's tracker = %v", withTask["tracker"])
	}
}

// issueCreator stands in for a Jira tracker that creates issues and records
// which tracker each one was created on.
type issueCreator struct {
	sectiletracker.BaseTicketingSystem
	mu sync.Mutex
	on string
}

func (c *issueCreator) FormatTaskID(trackerID, key, rawID string) string {
	return "jira-" + trackerID + "-" + key
}

func (c *issueCreator) CreateIssue(ctx context.Context, req sectiletracker.CreateIssueRequest) (*models.Task, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.on = req.Tracker.ID
	key := req.Tracker.Scope + "-7"
	return &models.Task{ID: key, Key: key}, nil
}

func TestTheMcpCreateTaskAcceptsATracker(t *testing.T) {
	database := openDB(t)
	creator := &issueCreator{BaseTicketingSystem: sectiletracker.BaseTicketingSystem{TrackerName: "jira", Capabilities: []sectiletracker.Capability{sectiletracker.CapCreate}}}
	database.TrackerRegistry().Register("jira", creator)
	gode, err := database.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Scope: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	be, err := database.CreateTrackerAs("admin", models.Tracker{Provider: "jira", Scope: "BE"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Delivery", Trackers: []models.ProjectTracker{{TrackerID: gode.ID}, {TrackerID: be.ID}}})
	if err != nil {
		t.Fatal(err)
	}

	result, err := call(t, database, "create_task", map[string]any{"projectId": project.ID, "title": "On BE", "tracker": be.Identity})
	if err != nil {
		t.Fatal(err)
	}
	task, _ := result["task"].(map[string]any)
	if task == nil || task["trackerId"] != be.ID || creator.on != be.ID || task["key"] != "BE-7" {
		t.Fatalf("created %v on %q, want BE", task, creator.on)
	}
	if _, err := call(
		t,
		database,
		"create_task",
		map[string]any{"projectId": project.ID, "title": "Nowhere", "tracker": "jira|elsewhere|OPS"},
	); err == nil || !strings.Contains(err.Error(), "unknown tracker") {
		t.Fatalf("an unknown tracker: %v", err)
	}
}

func TestTheMcpResolvesAKeyWithinItsProjectAndRefusesAnAmbiguousOneInEnglish(t *testing.T) {
	database := openDB(t)
	var projects []*models.Project
	for _, name := range []string{"Alpha", "Beta"} {
		p, err := database.CreateProject(models.CreateProjectRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		if err := database.ImportOrUpdateTasks(p.DefaultTrackerID, []models.Task{{ID: strings.ToLower(name) + "-1", Key: "TASK-1", Title: name, Status: models.StatusToClarify,
			Priority: models.PriorityMedium, Source: "local", CreatedAt: time.Now(), UpdatedAt: time.Now()}}); err != nil {
			t.Fatal(err)
		}
		projects = append(projects, p)
	}

	_, err := call(t, database, "get_task", map[string]any{"taskKey": "TASK-1"})
	if err == nil || !strings.Contains(err.Error(), "several trackers") || strings.Contains(err.Error(), "plusieurs") {
		t.Fatalf("an ambiguous key: %v", err)
	}
	projectContext, err := call(t, database, "get_project_context", map[string]any{"projectId": projects[1].ID, "taskKey": "TASK-1"})
	if err != nil || projectContext["projectId"] != projects[1].ID {
		t.Fatalf("TASK-1 within Beta: %v, %v", projectContext, err)
	}
	activity, err := call(t, database, "start_run", map[string]any{"taskKey": "TASK-1", "skill": "clarify", "projectId": projects[0].ID})
	if err != nil || activity["taskId"] != "alpha-1" {
		t.Fatalf("a run on Alpha's TASK-1: %v, %v", activity, err)
	}
}

func TestARunFindsTheKeyOfAnotherProjectsTicketAndRefusesOneTwoOtherTrackersCarry(t *testing.T) {
	database := openDB(t)
	tickets := map[string][]string{"Alpha": {"TASK-1"}, "Beta": {"ONLY-B", "SHARED-2"}, "Gamma": {"SHARED-2"}}
	var alpha *models.Project
	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		p, err := database.CreateProject(models.CreateProjectRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		if alpha == nil {
			alpha = p
		}
		for _, key := range tickets[name] {
			if err := database.ImportOrUpdateTasks(p.DefaultTrackerID, []models.Task{{ID: strings.ToLower(name + "-" + key), Key: key, Title: name, Status: models.StatusToClarify,
				Priority: models.PriorityMedium, Source: "local", CreatedAt: time.Now(), UpdatedAt: time.Now()}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	run, err := call(t, database, "start_run", map[string]any{"taskKey": "TASK-1", "skill": "clarify", "projectId": alpha.ID})
	if err != nil {
		t.Fatal(err)
	}
	runID, _ := run["id"].(string)

	result, err := callAsInRun(t, database, &tester, runID, "get_task", map[string]any{"taskKey": "ONLY-B"})
	if err != nil {
		t.Fatalf("Beta's ONLY-B within Alpha's run: %v", err)
	}
	if task, _ := result["task"].(map[string]any); task == nil || task["id"] != "beta-only-b" {
		t.Fatalf("Beta's ONLY-B within Alpha's run read %v", result["task"])
	}
	if _, err := callAsInRun(t, database, &tester, runID, "add_comment", map[string]any{"taskKey": "ONLY-B", "body": "Seen from Alpha"}); err != nil {
		t.Fatalf("a comment on Beta's ONLY-B within Alpha's run: %v", err)
	}
	_, err = callAsInRun(t, database, &tester, runID, "get_task", map[string]any{"taskKey": "SHARED-2"})
	if err == nil || !strings.Contains(err.Error(), "several trackers") {
		t.Fatalf("a key Beta and Gamma carry within Alpha's run: %v", err)
	}
}
