package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func TestProjectOrchestratorSettingsOverHTTP(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "OrchHTTP",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(database)

	// Update project orchestrator settings via PUT
	putBody := `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":3,"orchestratorPrompt":"Keep changes small"}`
	req := httptest.NewRequest(http.MethodPut, "/api/projects/"+project.ID, strings.NewReader(putBody))
	rec := httptest.NewRecorder()
	h.HandleProjectDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/projects/%s returned %d: %s", project.ID, rec.Code, rec.Body.String())
	}

	var updated models.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if !updated.OrchestratorEnabled || updated.OrchestratorMaxConcurrent != 3 || updated.OrchestratorPrompt != "Keep changes small" {
		t.Fatalf("unexpected updated project: %+v", updated)
	}
}

func TestProjectOrchestrateEndpoint(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, nil
	})

	project, err := database.CreateProject(models.CreateProjectRequest{
		Name:                      "OrchestrateEndpoint",
		IssueTracker:              "local",
		OrchestratorEnabled:       false,
		OrchestratorMaxConcurrent: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add an eligible task
	_, err = database.CreateTask(models.CreateTaskRequest{
		ProjectID:   project.ID,
		Title:       "Setup Core Metrics Pipeline",
		Description: "Configure metric collection endpoints and telemetry exporters.",
	})
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(database)

	post := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		return rec
	}
	path := "/api/projects/" + project.ID + "/orchestrate"

	// A disabled orchestrator is a conflict with the project settings.
	if rec := post(path, `{}`); rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when disabled, got %d: %s", rec.Code, rec.Body.String())
	}
	// No caller can force enablement or raise the cap: both are refused, not ignored.
	for _, body := range []string{`{"force":true}`, `{"maxConcurrent":10}`} {
		if rec := post(path, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("override %s: expected 400, got %d: %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := post("/api/projects/missing/orchestrate", `{}`); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown project, got %d: %s", rec.Code, rec.Body.String())
	}

	enabled := true
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	rec := post(path, `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 once enabled, got %d: %s", rec.Code, rec.Body.String())
	}

	var res models.OrchestrationResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Launched) != 1 {
		t.Fatalf("expected 1 launched task, got %d", len(res.Launched))
	}
	if res.MaxConcurrent != 2 {
		t.Fatalf("expected max concurrent 2, got %d", res.MaxConcurrent)
	}
}

func TestProjectOrchestrateDispatchesAsSignedInUser(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := NewHandler(database)
	userID, cookie := account(t, database, "orchestrator@example.com")
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Signed-in orchestration", IssueTracker: "local", OrchestratorEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Validate requests", Description: "Reject invalid requests with a clear error response."})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := make(chan agentprotocol.Operation, 1)
	database.SetAgentSelection(func(user, projectID, device string) (string, error) {
		if user != userID || projectID != project.ID {
			return "", fmt.Errorf("wrong caller route: %q / %q", user, projectID)
		}
		return "user-workstation", nil
	})
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action == "execute_skill" {
			dispatched <- op
		}
		return nil, nil
	})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/orchestrate", strings.NewReader(`{}`))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.HandleProjectDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("orchestrate returned %d: %s", rec.Code, rec.Body.String())
	}
	select {
	case op := <-dispatched:
		if op.UserID != userID || op.TaskID != task.ID || op.DeviceID != "user-workstation" {
			t.Fatalf("unexpected dispatch: %+v", op)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("signed-in user's workstation never received a launch")
	}
}

func TestDesktopBacklogEvaluationUsesPairedUserAndDevice(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := NewHandler(database)
	userID, _ := account(t, database, "desktop-orchestrator@example.com")
	key, device, err := database.CreateAPIKey(userID, "Desktop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Desktop backlog", IssueTracker: "local", OrchestratorEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Validate requests", Description: "Reject invalid requests with clear errors."}); err != nil {
		t.Fatal(err)
	}
	dispatched := make(chan agentprotocol.Operation, 1)
	database.SetAgentSelection(func(user, projectID, deviceID string) (string, error) {
		if user != userID || deviceID != device.ID {
			return "", fmt.Errorf("wrong route %s/%s", user, deviceID)
		}
		return deviceID, nil
	})
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action == "execute_skill" {
			dispatched <- op
		}
		return nil, nil
	})
	request := httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/orchestrate", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set(agentprotocol.DeviceIDHeader, "forged-device")
	response := httptest.NewRecorder()
	h.RequireSession(http.HandlerFunc(h.HandleProjectDetail)).ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("evaluate returned %d: %s", response.Code, response.Body.String())
	}
	select {
	case op := <-dispatched:
		if op.UserID != userID || op.DeviceID != device.ID {
			t.Fatalf("wrong dispatch: %+v", op)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("paired workstation received no launch")
	}
}

func TestProjectOrchestrateValidatesGuidelineSnapshotPayload(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Guideline payload", IssueTracker: "local", OrchestratorEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(database)
	for _, test := range []struct {
		body   string
		status int
		detail string
	}{
		{`{"guidelines":`, http.StatusBadRequest, "Invalid orchestration request"},
		{`{"guidelines":"` + strings.Repeat("x", models.OrchestratorGuidelinesMaxBytes+1) + `"}`, http.StatusBadRequest, "exceed 32 KiB"},
		// JSON escaping can make the request larger than the Markdown text.
		{`{"guidelines":"` + strings.Repeat(`\"`, models.OrchestratorGuidelinesMaxBytes) + `"}`, http.StatusOK, ""},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/orchestrate", strings.NewReader(test.body))
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, req)
		if rec.Code != test.status || !strings.Contains(rec.Body.String(), test.detail) {
			t.Fatalf("guideline request: %d %s", rec.Code, rec.Body.String())
		}
	}
}
