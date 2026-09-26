package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"tasks/internal/agentprotocol"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
)

func TestHandleGitStatus(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action != "git_status" || op.ProjectID != "default" {
			t.Fatalf("wrong agent request: %#v", op)
		}
		return json.Marshal(models.GitStatusInfo{IsGitRepo: true, Branch: "agent-branch"})
	})
	req := httptest.NewRequest(http.MethodGet, "/api/git-status?projectId=default", nil)

	rr := httptest.NewRecorder()
	h.HandleGitStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status models.GitStatusInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if !status.IsGitRepo {
		t.Errorf("Expected isGitRepo=true for current directory")
	}

	if status.Branch == "" {
		t.Errorf("Expected non-empty branch name")
	}
}

func TestCreateTaskWithCustomTrackerSource(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	// Create a test project with issueTracker="github"
	_, _ = database.CreateProject(models.CreateProjectRequest{
		Name:         "Test Project",
		Slug:         "test-proj",
		IssueTracker: "github",
		GithubRepo:   "acme/app",
	})

	// 1. Create a task with explicitly specified source="local"
	taskBody := `{"title": "Test Local Task", "source": "local", "projectId": "test-proj"}`
	req, err := http.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(taskBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.HandleTasks(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201 Created, got %d: %s", rr.Code, rr.Body.String())
	}

	var task models.Task
	if err := json.Unmarshal(rr.Body.Bytes(), &task); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if task.Source != "local" {
		t.Errorf("Expected task.Source='local', got '%s'", task.Source)
	}

	if task.Title != "Test Local Task" {
		t.Errorf("Expected task.Title='Test Local Task', got '%s'", task.Title)
	}

	// 2. Create another task where source is omitted (should fallback to project tracker "github")
	taskBodyDefault := `{"title": "Default Project Tracker Task", "projectId": "test-proj"}`
	req2, err := http.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(taskBodyDefault))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req2.Header.Set("Content-Type", "application/json")

	rr2 := httptest.NewRecorder()
	h.HandleTasks(rr2, req2)

	if rr2.Code < 400 {
		t.Fatalf("unconfigured remote creation succeeded: %d %s", rr2.Code, rr2.Body.String())
	}
	// The request carries no session: a remote creation that names nobody is
	// refused by name before it reaches GitHub, rather than filed locally or
	// signed by the server account (#482).
	if rr2.Code != http.StatusForbidden || !strings.Contains(rr2.Body.String(), "not tied to a user") {
		t.Fatalf("remote error missing: %d %s", rr2.Code, rr2.Body.String())
	}

}

func TestHandleOpenEditor(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	// The server confirms only a successful agent response.
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		// The editor is the workstation's (#305): the server names none, even
		// when an older interface still sends one.
		if op.Action != "open_editor" || op.Editor != "" || op.ProjectID != "default" {
			t.Fatalf("wrong request: %#v", op)
		}
		return json.RawMessage(`null`), nil
	})
	body := `{"projectId":"default","editorCommand":"code"}`
	req, err := http.NewRequest(http.MethodPost, "/api/open-editor", strings.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.HandleOpenEditor(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if resp["success"] != true {
		t.Errorf("Expected success=true, got %v", resp["success"])
	}
}

func TestHandleTaskPinAndListPins(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	task, err := database.CreateTask(models.CreateTaskRequest{
		Title:    "Task to Pin",
		Status:   models.StatusToClarify,
		Priority: models.PriorityMedium,
		Source:   "local",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// 1. Pin via POST /api/tasks/{id}/pin
	req, _ := http.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/pin", nil)
	rr := httptest.NewRecorder()
	h.HandleTaskDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var pinResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &pinResp)
	if pinResp["pinned"] != true {
		t.Errorf("Expected pinned=true, got %v", pinResp["pinned"])
	}

	// 2. Fetch pins via GET /api/tasks/pins
	reqList, _ := http.NewRequest(http.MethodGet, "/api/tasks/pins", nil)
	rrList := httptest.NewRecorder()
	h.HandleTaskPins(rrList, reqList)

	if rrList.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rrList.Code, rrList.Body.String())
	}

	var pinnedTasks []models.Task
	if err := json.Unmarshal(rrList.Body.Bytes(), &pinnedTasks); err != nil {
		t.Fatalf("Failed to unmarshal pinned tasks: %v", err)
	}

	if len(pinnedTasks) != 1 || pinnedTasks[0].ID != task.ID {
		t.Errorf("Expected 1 pinned task with id %s, got %v", task.ID, pinnedTasks)
	}
	if !pinnedTasks[0].Pinned {
		t.Errorf("Expected pinnedTask.Pinned=true")
	}

	// 3. Unpin via DELETE /api/tasks/{id}/pin
	reqDel, _ := http.NewRequest(http.MethodDelete, "/api/tasks/"+task.ID+"/pin", nil)
	rrDel := httptest.NewRecorder()
	h.HandleTaskDetail(rrDel, reqDel)

	if rrDel.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rrDel.Code, rrDel.Body.String())
	}

	// Verify pins list is now empty
	rrList2 := httptest.NewRecorder()
	h.HandleTaskPins(rrList2, reqList)
	var pinnedTasks2 []models.Task
	_ = json.Unmarshal(rrList2.Body.Bytes(), &pinnedTasks2)
	if len(pinnedTasks2) != 0 {
		t.Errorf("Expected 0 pinned tasks after unpinning, got %d", len(pinnedTasks2))
	}
}

func TestHandleTaskStageTransition(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize db: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	// Create project and task
	proj, _ := database.CreateProject(models.CreateProjectRequest{
		Name:         "Stage Handler Test",
		Slug:         "stage-handler-test",
		IssueTracker: "local",
	})

	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: proj.ID,
		Title:     "Test transition endpoint",
		Labels:    []string{"#new"},
	})
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 1. POST /api/tasks/{id}/stage
	stageBody := `{"stage": "clarified", "note": "Questions resolved"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/stage", strings.NewReader(stageBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.HandleTaskDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var res struct {
		Success bool         `json:"success"`
		Message string       `json:"message"`
		Task    *models.Task `json:"task"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to decode json: %v", err)
	}
	if !res.Success {
		t.Errorf("Expected success=true")
	}
	if res.Task == nil || res.Task.Status != models.StatusClarified {
		t.Errorf("Expected status %s, got %v", models.StatusClarified, res.Task)
	}

	// 2. GET /api/tasks/{id}/stage
	reqGet, _ := http.NewRequest(http.MethodGet, "/api/tasks/"+task.ID+"/stage", nil)
	rrGet := httptest.NewRecorder()
	h.HandleTaskDetail(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rrGet.Code, rrGet.Body.String())
	}
	var getRes map[string]interface{}
	_ = json.Unmarshal(rrGet.Body.Bytes(), &getRes)
	if getRes["stage"] != "clarified" {
		t.Errorf("Expected stage='clarified', got %v", getRes["stage"])
	}

	// 3. POST /api/tasks/stage by key
	batchBody := `{"taskKey": "` + task.Key + `", "stage": "specified", "branch": "feat/api-stage"}`
	reqBatch, _ := http.NewRequest(http.MethodPost, "/api/tasks/stage", strings.NewReader(batchBody))
	reqBatch.Header.Set("Content-Type", "application/json")
	rrBatch := httptest.NewRecorder()
	h.HandleTasks(rrBatch, reqBatch)

	if rrBatch.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rrBatch.Code, rrBatch.Body.String())
	}

	var batchRes struct {
		Success bool         `json:"success"`
		Task    *models.Task `json:"task"`
	}
	_ = json.Unmarshal(rrBatch.Body.Bytes(), &batchRes)
	if batchRes.Task == nil || batchRes.Task.Status != models.StatusToImplement {
		t.Errorf("Expected status %s, got %v", models.StatusToImplement, batchRes.Task)
	}
}

func TestHandleGitBranchesAndCheckoutWithAll(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.ProjectID != "default" {
			t.Fatalf("wrong project: %#v", op)
		}
		switch op.Action {
		case "git_branches":
			return json.Marshal(models.GitBranchesInfo{CurrentBranch: "agent-branch"})
		case "git_checkout":
			if op.Branch != "agent-branch" || op.Create {
				t.Fatalf("wrong checkout: %#v", op)
			}
			return json.RawMessage(`{}`), nil
		default:
			t.Fatalf("wrong action: %#v", op)
			return nil, nil
		}
	})

	// 1. GET /api/git/branches?projectId=all should resolve the default project and use its agent
	reqBranches, err := http.NewRequest(http.MethodGet, "/api/git/branches?projectId=all", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rrBranches := httptest.NewRecorder()
	h.HandleGitBranches(rrBranches, reqBranches)

	if rrBranches.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for branches, got %d: %s", rrBranches.Code, rrBranches.Body.String())
	}

	var info models.GitBranchesInfo
	if err := json.Unmarshal(rrBranches.Body.Bytes(), &info); err != nil {
		t.Fatalf("Failed to decode branches JSON: %v", err)
	}
	if info.CurrentBranch == "" {
		t.Errorf("Expected non-empty current branch")
	}

	// 2. POST /api/git/checkout with projectId="all" and current branch should succeed cleanly
	checkoutBody := `{"projectId": "all", "branch": "` + info.CurrentBranch + `", "create": false}`
	reqCheckout, err := http.NewRequest(http.MethodPost, "/api/git/checkout", strings.NewReader(checkoutBody))
	if err != nil {
		t.Fatalf("Failed to create checkout request: %v", err)
	}
	reqCheckout.Header.Set("Content-Type", "application/json")
	rrCheckout := httptest.NewRecorder()
	h.HandleGitCheckout(rrCheckout, reqCheckout)

	if rrCheckout.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for checkout, got %d: %s", rrCheckout.Code, rrCheckout.Body.String())
	}
}

func TestCloneTaskHandler(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	// 1. Create a base task
	baseTask, err := database.CreateTask(models.CreateTaskRequest{
		Title:       "Original Story Title",
		Description: "Story detailed description",
		Priority:    models.PriorityHigh,
		Labels:      []string{"Feature", "Backend"},
		Assignee:    "John Doe",
		Sprint:      "Sprint 42",
		Source:      "local",
	})
	if err != nil {
		t.Fatalf("Failed to create base task: %v", err)
	}

	// 2. Clone via POST /api/tasks/{id}/clone with custom title
	cloneBody := `{"title": "Cloned Custom Story", "sprint": "Sprint 43"}`
	reqClone, err := http.NewRequest(http.MethodPost, "/api/tasks/"+baseTask.ID+"/clone", strings.NewReader(cloneBody))
	if err != nil {
		t.Fatalf("Failed to create clone request: %v", err)
	}
	reqClone.Header.Set("Content-Type", "application/json")
	rrClone := httptest.NewRecorder()
	h.HandleTaskDetail(rrClone, reqClone)

	if rrClone.Code != http.StatusCreated {
		t.Fatalf("Expected status 201 Created for clone, got %d: %s", rrClone.Code, rrClone.Body.String())
	}

	var clonedTask models.Task
	if err := json.Unmarshal(rrClone.Body.Bytes(), &clonedTask); err != nil {
		t.Fatalf("Failed to decode cloned task JSON: %v", err)
	}

	if clonedTask.ID == baseTask.ID {
		t.Errorf("Cloned task must have a distinct ID, got same: %s", clonedTask.ID)
	}
	if clonedTask.Key == baseTask.Key {
		t.Errorf("Cloned task must have a distinct Key, got same: %s", clonedTask.Key)
	}
	if clonedTask.Title != "Cloned Custom Story" {
		t.Errorf("Expected title 'Cloned Custom Story', got '%s'", clonedTask.Title)
	}
	if clonedTask.Description != "Story detailed description" {
		t.Errorf("Expected description to be preserved, got '%s'", clonedTask.Description)
	}
	if clonedTask.Priority != models.PriorityHigh {
		t.Errorf("Expected priority High, got '%s'", clonedTask.Priority)
	}
	if clonedTask.Sprint != "Sprint 43" {
		t.Errorf("Expected sprint 'Sprint 43', got '%s'", clonedTask.Sprint)
	}
	if clonedTask.Assignee != "John Doe" {
		t.Errorf("Expected assignee 'John Doe', got '%s'", clonedTask.Assignee)
	}
	if clonedTask.Status != models.StatusToClarify {
		t.Errorf("Expected initial status 'to_clarify', got '%s'", clonedTask.Status)
	}

	// 3. Clone with default parameters (no body)
	reqCloneDefault, _ := http.NewRequest(http.MethodPost, "/api/tasks/"+baseTask.ID+"/clone", nil)
	rrCloneDefault := httptest.NewRecorder()
	h.HandleTaskDetail(rrCloneDefault, reqCloneDefault)

	if rrCloneDefault.Code != http.StatusCreated {
		t.Fatalf("Expected status 201 Created for default clone, got %d: %s", rrCloneDefault.Code, rrCloneDefault.Body.String())
	}

	var defaultCloned models.Task
	_ = json.Unmarshal(rrCloneDefault.Body.Bytes(), &defaultCloned)
	if defaultCloned.Title != "Original Story Title (Copie)" {
		t.Errorf("Expected default title 'Original Story Title (Copie)', got '%s'", defaultCloned.Title)
	}
}

func TestHealthEndpointReturnsSectileAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)
	req, err := http.NewRequest(http.MethodGet, "/api/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	h.HandleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var healthRes map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &healthRes); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if healthRes["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %q", healthRes["status"])
	}
	if healthRes["service"] != "sectile-api" {
		t.Errorf("Expected service 'sectile-api' for backward compatibility, got %q", healthRes["service"])
	}
}

func TestCreateTaskPopulatesCreatorFromAuthenticatedPrincipal(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Local Test Project",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	user, err := database.SignInLocal("alice@example.com")
	if err != nil {
		t.Fatalf("Failed to sign in user: %v", err)
	}
	token, _, err := database.CreateWebSession(user.ID)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionCookie := &http.Cookie{Name: "sectile_session", Value: token}

	avatarURL := "https://example.com/alice.png"
	_, err = database.UpdateUserSettings(user.ID, models.Settings{UserAvatar: avatarURL})
	if err != nil {
		t.Fatalf("Failed to update user settings: %v", err)
	}

	h := handlers.NewHandler(database)

	// 1. Authenticated task creation
	taskBody := `{"title": "Alice Task", "source": "local", "projectId": "` + proj.ID + `"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(taskBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rr := httptest.NewRecorder()
	h.HandleTasks(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var created models.Task
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if created.Creator != "alice@example.com" {
		t.Errorf("Expected Creator 'alice@example.com', got %q", created.Creator)
	}
	if created.CreatorAvatar != avatarURL {
		t.Errorf("Expected CreatorAvatar %q, got %q", avatarURL, created.CreatorAvatar)
	}

	// 2. Anonymous task creation
	anonBody := `{"title": "Anonymous Task", "source": "local", "projectId": "` + proj.ID + `"}`
	anonReq, _ := http.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(anonBody))
	anonReq.Header.Set("Content-Type", "application/json")
	anonRR := httptest.NewRecorder()
	h.HandleTasks(anonRR, anonReq)

	if anonRR.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", anonRR.Code, anonRR.Body.String())
	}
	var anonCreated models.Task
	if err := json.Unmarshal(anonRR.Body.Bytes(), &anonCreated); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if anonCreated.Creator != "" {
		t.Errorf("Expected empty Creator for anonymous task, got %q", anonCreated.Creator)
	}
	if anonCreated.CreatorAvatar != "" {
		t.Errorf("Expected empty CreatorAvatar for anonymous task, got %q", anonCreated.CreatorAvatar)
	}

	// 3. Batch creation with authenticated session
	batchBody := `[{"title": "Batch Task 1", "source": "local", "projectId": "` + proj.ID + `"}, {"title": "Batch Task 2", "source": "local", "projectId": "` + proj.ID + `"}]`
	batchReq, _ := http.NewRequest(http.MethodPost, "/api/tasks/batch", strings.NewReader(batchBody))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.AddCookie(sessionCookie)
	batchRR := httptest.NewRecorder()
	h.HandleTasks(batchRR, batchReq)

	if batchRR.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", batchRR.Code, batchRR.Body.String())
	}
	var batchCreated []models.Task
	if err := json.Unmarshal(batchRR.Body.Bytes(), &batchCreated); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(batchCreated) != 2 {
		t.Fatalf("Expected 2 tasks created, got %d", len(batchCreated))
	}
	for i, task := range batchCreated {
		if task.Creator != "alice@example.com" {
			t.Errorf("Batch task [%d] expected Creator 'alice@example.com', got %q", i, task.Creator)
		}
		if task.CreatorAvatar != avatarURL {
			t.Errorf("Batch task [%d] expected CreatorAvatar %q, got %q", i, avatarURL, task.CreatorAvatar)
		}
	}
}
