package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/models"
)

// runSkillServer serves the launch route alone: no agent is registered, so a
// launch that passes the duplicate guard ends on the handler's own "Connect the
// local agent" refusal. That answer is what "the guard let it through" looks
// like here, and it is textually distinct from the guard's own refusal.
func runSkillServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(h.HandleTaskDetail))
	t.Cleanup(server.Close)
	return server
}

func guardTask(t *testing.T, database *db.DB, title string) *models.Task {
	t.Helper()
	task, err := database.CreateTask(models.CreateTaskRequest{Title: title, ProjectID: "default", Priority: "high"})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func activityCount(t *testing.T, database *db.DB, taskID string) int {
	t.Helper()
	activities, err := database.GetTaskActivities(taskID)
	if err != nil {
		t.Fatal(err)
	}
	return len(activities)
}

// A task already carrying a run is busy, whatever declared that run: the whole
// point of the guard is to see the runs a Claude Code session declares over
// MCP, which the desktop's own queue cannot.
func TestLaunchIsRefusedWhileARunIsActive(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	_, alice := account(t, database, "alice@example.com")

	busy := guardTask(t, database, "Busy task")
	run, err := database.StartAgentRun(busy.ID, "implement", db.RunLaunch{Mode: "autonomous"})
	if err != nil {
		t.Fatal(err)
	}
	before := activityCount(t, database, busy.ID)

	status, body := call(t, server, alice, http.MethodPost, "/api/tasks/"+busy.ID+"/run-skill", `{"skillId":"clarify"}`)
	if status != http.StatusConflict || !strings.Contains(body, `"activeRunId":"`+run.ID+`"`) {
		t.Fatalf("a launch on a busy task: %d %s", status, body)
	}
	// NFR1: a rejected launch leaves no trace at all.
	if after := activityCount(t, database, busy.ID); after != before {
		t.Fatalf("the refusal recorded %d activities", after-before)
	}

	// Waiting for the user is being active: the session is still there.
	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	if status, body = call(t, server, alice, http.MethodPost, "/api/tasks/"+busy.ID+"/run-skill", `{"skillId":"clarify"}`); status != http.StatusConflict || !strings.Contains(body, "waiting for user input") {
		t.Fatalf("a launch on a waiting run: %d %s", status, body)
	}
}

// The three shapes that must not refuse: a closed run, a launch record, and
// nothing at all. agent_launch is a launch, not a run.
func TestLaunchProceedsWithoutAnActiveRun(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	_, alice := account(t, database, "alice@example.com")

	free := guardTask(t, database, "Free task")
	finished, err := database.StartAgentRun(free.ID, "implement", db.RunLaunch{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.FinishRemoteRun(free.ID, finished.ID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := database.AddTaskActivity(models.TaskActivity{
		ID: "launch-1", TaskID: free.ID, SkillID: "agent_launch", SkillName: "clarify",
		Action: "Exécution de agent_launch sur l'agent local", Status: "running", CreatedAt: now, StartedAt: &now,
	}); err != nil {
		t.Fatal(err)
	}

	status, body := call(t, server, alice, http.MethodPost, "/api/tasks/"+free.ID+"/run-skill", `{"skillId":"clarify"}`)
	if status != http.StatusConflict || !strings.Contains(body, "Connect the local agent") {
		t.Fatalf("a launch on a free task was stopped by the guard: %d %s", status, body)
	}

	// A workflow skill running under its own id is a run, and does refuse.
	if err := database.AddTaskActivity(models.TaskActivity{
		ID: "clarify-1", TaskID: free.ID, SkillID: "clarify", SkillName: "clarify",
		Action: "Clarification", Status: "running", CreatedAt: now, StartedAt: &now,
	}); err != nil {
		t.Fatal(err)
	}
	if status, body = call(t, server, alice, http.MethodPost, "/api/tasks/"+free.ID+"/run-skill", `{"skillId":"clarify"}`); status != http.StatusConflict || !strings.Contains(body, `"activeRunId":"clarify-1"`) {
		t.Fatalf("a running workflow skill did not refuse: %d %s", status, body)
	}
}

// force waives the duplicate check and nothing else, for the owner of the run
// or an admin, and it never closes the run it steps over.
func TestForceWaivesTheDuplicateCheckOnly(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	// The first account is the deployment's admin, the others are members.
	_, alice := account(t, database, "alice@example.com")
	bobID, bob := account(t, database, "bob@example.com")
	_, carol := account(t, database, "carol@example.com")

	task := guardTask(t, database, "Contested task")
	run, err := database.StartAgentRun(task.ID, "implement", db.RunLaunch{UserID: bobID})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name   string
		cookie *http.Cookie
		body   string
		status int
		expect string
	}{
		{"the owner forces through", bob, `{"skillId":"clarify","force":true}`, http.StatusConflict, "Connect the local agent"},
		{"an admin forces through", alice, `{"skillId":"clarify","force":true}`, http.StatusConflict, "Connect the local agent"},
		{"a third party is refused", carol, `{"skillId":"clarify","force":true}`, http.StatusForbidden, msgNotOwner},
		{"force does not waive the mode check", bob, `{"skillId":"clarify","force":true,"mode":"telepathic"}`, http.StatusBadRequest, ""},
	} {
		status, body := call(t, server, c.cookie, http.MethodPost, "/api/tasks/"+task.ID+"/run-skill", c.body)
		if status != c.status || !strings.Contains(body, c.expect) {
			t.Fatalf("%s: %d %s", c.name, status, body)
		}
	}

	// force is not a cancellation: the run it stepped over is untouched.
	active, err := database.ActiveRunOnTask(task.ID)
	if err != nil || active == nil || active.ID != run.ID || active.Status != "running" {
		t.Fatalf("the active run was disturbed: %+v %v", active, err)
	}
}

// A queued run makes the task busy too (#407), and every path that records a
// run answers the same 409 once the database refuses a second one: the queued
// launch, the next step, the full chain and a retry. None records anything.
func TestEveryLaunchPathRefusesABusyTask(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	activities := httptest.NewServer(http.HandlerFunc(h.HandleActivityDetail))
	defer activities.Close()
	_, alice := account(t, database, "alice@example.com")

	task := guardTask(t, database, "Queued task")
	now := time.Now()
	if err := database.AddTaskActivity(models.TaskActivity{
		ID: "queued-1", TaskID: task.ID, SkillID: "clarify", SkillName: "clarify",
		Action: "Clarification", Status: "queued", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.AddTaskActivity(models.TaskActivity{
		ID: "failed-1", TaskID: task.ID, SkillID: "clarify", SkillName: "clarify",
		Action: "Clarification", Status: "failed", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	before := activityCount(t, database, task.ID)

	for _, tc := range []struct {
		name, method, url, body string
		server                  *httptest.Server
	}{
		{"launch", http.MethodPost, "/api/tasks/" + task.ID + "/run-skill", `{"skillId":"clarify"}`, server},
		{"next step", http.MethodPost, "/api/tasks/" + task.ID + "/advance", `{}`, server},
		{"full chain", http.MethodPost, "/api/tasks/" + task.ID + "/advance", `{"auto":true}`, server},
		{"retry", http.MethodPost, "/api/activities/failed-1/retry", ``, activities},
	} {
		status, body := call(t, tc.server, alice, tc.method, tc.url, tc.body)
		if status != http.StatusConflict || !strings.Contains(body, `"activeRunId":"queued-1"`) || !strings.Contains(body, "is queued on this task") {
			t.Fatalf("%s on a task with a queued run: %d %s", tc.name, status, body)
		}
	}
	if after := activityCount(t, database, task.ID); after != before {
		t.Fatalf("the refusals recorded %d activities", after-before)
	}
}
