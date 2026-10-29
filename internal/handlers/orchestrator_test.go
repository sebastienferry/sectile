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

	h := NewHandler(database)

	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}
	path := "/api/projects/" + project.ID + "/orchestrate"

	// A disabled orchestrator is a conflict with the project settings.
	if rec := do(http.MethodPost, path, `{}`); rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when disabled, got %d: %s", rec.Code, rec.Body.String())
	}
	// No caller can force enablement or raise the cap: both are refused, not ignored.
	for _, body := range []string{`{"force":true}`, `{"maxConcurrent":10}`} {
		if rec := do(http.MethodPost, path, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("override %s: expected 400, got %d: %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := do(http.MethodPost, "/api/projects/missing/orchestrate", `{}`); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown project, got %d: %s", rec.Code, rec.Body.String())
	}

	enabled := true
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{OrchestratorEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	// Without a connected workstation nothing can run the session.
	if rec := do(http.MethodPost, path, `{}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "no workstation") {
		t.Fatalf("expected 409 without a workstation, got %d: %s", rec.Code, rec.Body.String())
	}
	// The handler installs the real agent hub; the test stands in for a
	// connected workstation after it.
	database.SetAgentSelection(func(user, projectID, device string) (string, error) { return "workstation", nil })
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, nil
	})
	rec := do(http.MethodPost, path, `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 once enabled, got %d: %s", rec.Code, rec.Body.String())
	}
	var started struct {
		Session models.TaskActivity `json:"session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil || started.Session.ID == "" || started.Session.RunScope != "orchestrator" {
		t.Fatalf("start returned no orchestrator session: %s %v", rec.Body.String(), err)
	}

	// A second start while the session runs names the running one.
	if rec := do(http.MethodPost, path, `{}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), started.Session.ID) {
		t.Fatalf("second start: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(http.MethodGet, path, "")
	var state models.OrchestrationState
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &state) != nil {
		t.Fatalf("state: %d %s", rec.Code, rec.Body.String())
	}
	if state.MaxConcurrent != 2 || state.Armed == nil || state.Session == nil || state.Session.ID != started.Session.ID {
		t.Fatalf("unexpected state: %+v", state)
	}

	rec = do(http.MethodPost, path+"/stop", "")
	var stopped models.OrchestrationState
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &stopped) != nil || stopped.Armed != nil {
		t.Fatalf("stop: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodDelete, path, ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE: expected 405, got %d", rec.Code)
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
	if rec.Code != http.StatusAccepted {
		t.Fatalf("orchestrate returned %d: %s", rec.Code, rec.Body.String())
	}
	select {
	case op := <-dispatched:
		if op.UserID != userID || op.TaskID != "" || op.Scope != agentprotocol.ScopeOrchestrator || op.SkillID != db.OrchestratorSkillID || op.DeviceID != "user-workstation" {
			t.Fatalf("unexpected dispatch: %+v", op)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("signed-in user's workstation never received the orchestrator session")
	}
}

func TestDesktopOrchestrationStartsOnPairedUserAndDevice(t *testing.T) {
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
	if response.Code != http.StatusAccepted {
		t.Fatalf("start returned %d: %s", response.Code, response.Body.String())
	}
	select {
	case op := <-dispatched:
		if op.UserID != userID || op.DeviceID != device.ID || op.Scope != agentprotocol.ScopeOrchestrator {
			t.Fatalf("wrong dispatch: %+v", op)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("paired workstation received no orchestrator session")
	}
}

// A desktop that predates ADR 0064 sends its policy text and expects the
// server to evaluate the backlog. It is told to update rather than having its
// policy silently ignored.
func TestProjectOrchestrateRefusesALegacyPolicySnapshot(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Legacy payload", IssueTracker: "local", OrchestratorEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(database)
	for _, test := range []struct {
		body   string
		detail string
	}{
		{`{"guidelines":`, "Invalid orchestration request"},
		{`{"guidelines":"Only small fixes."}`, "update Sectile Desktop"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/orchestrate", strings.NewReader(test.body))
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), test.detail) {
			t.Fatalf("legacy request %s: %d %s", test.body, rec.Code, rec.Body.String())
		}
	}
	if state, err := database.OrchestrationState(project.ID); err != nil || state.Armed != nil {
		t.Fatalf("a refused request armed the project: %+v %v", state, err)
	}
}
