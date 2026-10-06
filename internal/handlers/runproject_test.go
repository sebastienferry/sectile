package handlers

import (
	"encoding/json"
	"net/http"
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
