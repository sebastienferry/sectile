package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// A launch on a ticket of two projects (#741): the web asks which project when
// it launched interactively without one, an unattended launch is refused, and
// a launch naming the board's project goes on.
func TestARunSkillOnATwoProjectTicketAsksWhichProject(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	_, alice := account(t, database, "alice@example.com")
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
	task, err := database.GetTaskByID("GODE-1")
	if err != nil || task == nil {
		t.Fatal(err)
	}
	path := "/api/tasks/" + task.ID + "/run-skill"

	status, body := call(t, server, alice, http.MethodPost, path, `{"skillId":"clarify"}`)
	var answer struct {
		Candidates []struct{ ID, Name string } `json:"candidates"`
		Unattended bool                        `json:"unattended"`
	}
	if err := json.Unmarshal([]byte(body), &answer); err != nil || status != http.StatusConflict || len(answer.Candidates) != 2 || answer.Unattended {
		t.Fatalf("an interactive launch with no project: %d %s", status, body)
	}
	if answer.Candidates[0].ID != delivery.ID || answer.Candidates[1].ID != bidder.ID || answer.Candidates[1].Name != "Bidder" {
		t.Fatalf("candidates = %+v", answer.Candidates)
	}

	status, body = call(t, server, alice, http.MethodPost, path, `{"skillId":"clarify","mode":"autonomous"}`)
	if status != http.StatusBadRequest || !strings.Contains(body, `"unattended":true`) || !strings.Contains(body, bidder.ID) {
		t.Fatalf("an autonomous launch with no project: %d %s", status, body)
	}

	status, body = call(t, server, alice, http.MethodPost, path, `{"skillId":"clarify","projectId":"`+bidder.ID+`"}`)
	if strings.Contains(body, "candidates") || !strings.Contains(body, "Connect the local agent") {
		t.Fatalf("a launch from Bidder's board: %d %s", status, body)
	}
}

// A key two trackers carry names no one ticket (#741): the web is answered 409,
// so it asks for the ticket's id, never 404 nor 500.
func TestATaskReadByAKeyTwoTrackersCarryIsAConflict(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	_, alice := account(t, database, "alice@example.com")
	for _, name := range []string{"Alpha", "Beta"} {
		p, err := database.CreateProject(models.CreateProjectRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		if err := database.ImportOrUpdateTasks(p.DefaultTrackerID, []models.Task{{ID: strings.ToLower(name) + "-1", Key: "TASK-1", Title: name, Status: models.StatusToClarify,
			Priority: models.PriorityMedium, Source: "local", CreatedAt: time.Now(), UpdatedAt: time.Now()}}); err != nil {
			t.Fatal(err)
		}
	}

	if status, body := call(t, server, alice, http.MethodGet, "/api/tasks/TASK-1", ""); status != http.StatusConflict {
		t.Fatalf("GET by an ambiguous key: %d %s", status, body)
	}
	if status, body := call(t, server, alice, http.MethodPost, "/api/tasks/TASK-1/run-skill", `{"skillId":"clarify"}`); status != http.StatusConflict {
		t.Fatalf("a launch by an ambiguous key: %d %s", status, body)
	}
	if status, body := call(t, server, alice, http.MethodGet, "/api/tasks/beta-1", ""); status != http.StatusOK {
		t.Fatalf("GET by id: %d %s", status, body)
	}
}

// An activity recorded before #741 names no project: retrying it on a ticket of
// two projects asks which one, as a launch does, instead of failing with a 500.
func TestRetryingAnActivityOfNoProjectOnATwoProjectTicketAsksWhichProject(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := httptest.NewServer(http.HandlerFunc(h.HandleActivityDetail))
	t.Cleanup(server.Close)
	_, alice := account(t, database, "alice@example.com")
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
	task, err := database.GetTaskByID("GODE-1")
	if err != nil || task == nil {
		t.Fatal(err)
	}
	old := models.TaskActivity{ID: "old-run", TaskID: task.ID, TaskKey: task.Key, SkillID: "clarify", SkillName: "Clarify", Status: string(models.ActivityStatusFailed), CreatedAt: time.Now()}
	if err := database.AddTaskActivity(old); err != nil {
		t.Fatal(err)
	}

	status, body := call(t, server, alice, http.MethodPost, "/api/activities/old-run/retry", "")
	var answer struct {
		Candidates []struct{ ID string } `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(body), &answer); err != nil || status != http.StatusConflict || len(answer.Candidates) != 2 {
		t.Fatalf("retrying an activity of no project: %d %s", status, body)
	}
	if answer.Candidates[0].ID != delivery.ID || answer.Candidates[1].ID != bidder.ID {
		t.Fatalf("candidates = %+v", answer.Candidates)
	}

	// The retry naming one of the candidates goes through, for that project.
	status, body = call(t, server, alice, http.MethodPost, "/api/activities/old-run/retry", `{"projectId":"`+bidder.ID+`"}`)
	var retried models.TaskActivity
	if err := json.Unmarshal([]byte(body), &retried); err != nil || status != http.StatusOK || retried.ID == "" || retried.ID == old.ID {
		t.Fatalf("retrying for Bidder: %d %s", status, body)
	}
	if stored, err := database.GetActivityByID(retried.ID); err != nil || stored == nil || stored.RunProjectID != bidder.ID {
		t.Fatalf("the retried activity = %+v (%v), want it run for Bidder %s", stored, err, bidder.ID)
	}

	// A project the ticket is not shown in is refused as a launch's is.
	notes, err := database.CreateProject(models.CreateProjectRequest{Name: "Notes"})
	if err != nil {
		t.Fatal(err)
	}
	if status, body := call(t, server, alice, http.MethodPost, "/api/activities/old-run/retry", `{"projectId":"`+notes.ID+`"}`); status != http.StatusBadRequest {
		t.Fatalf("retrying for a project the ticket is not in: %d %s", status, body)
	}
}

// The other lookups of a ticket by a key two trackers carry say so too (#741),
// rather than answering that the ticket does not exist: typing a skill into the
// ticket's terminal, and a batch member named by that key.
func TestTheTerminalSkillAndABatchMemberByAKeyTwoTrackersCarryAreRefusedAsAmbiguous(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	_, alice := account(t, database, "alice@example.com")
	var alpha *models.Project
	for _, name := range []string{"Alpha", "Beta"} {
		p, err := database.CreateProject(models.CreateProjectRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		if alpha == nil {
			alpha = p
		}
		tickets := []models.Task{{ID: strings.ToLower(name) + "-1", Key: "TASK-1", Title: name, Status: models.StatusToClarify,
			Priority: models.PriorityMedium, Source: "local", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
		if name == "Alpha" {
			tickets = append(tickets, models.Task{ID: "alpha-2", Key: "TASK-2", Title: "Alpha 2", Status: models.StatusToClarify,
				Priority: models.PriorityMedium, Source: "local", CreatedAt: time.Now(), UpdatedAt: time.Now()})
		}
		if err := database.ImportOrUpdateTasks(p.DefaultTrackerID, tickets); err != nil {
			t.Fatal(err)
		}
	}
	ambiguity := "plusieurs trackers"

	if status, body := call(t, server, alice, http.MethodPost, "/api/tasks/TASK-1/tty-skill", `{"skillId":"clarify"}`); status != http.StatusConflict || !strings.Contains(body, ambiguity) {
		t.Fatalf("a terminal skill by an ambiguous key: %d %s", status, body)
	}
	status, body := call(t, server, alice, http.MethodPost, "/api/tasks/alpha-2/run-skill",
		`{"skillId":"pickup_issues","projectId":"`+alpha.ID+`","batchTaskIds":["alpha-2","TASK-1"]}`)
	if status != http.StatusBadRequest || !strings.Contains(body, ambiguity) {
		t.Fatalf("a batch member by an ambiguous key: %d %s", status, body)
	}
}
