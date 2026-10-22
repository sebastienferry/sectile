package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

	// Calling without force while disabled should return 400
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/orchestrate", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.HandleProjectDetail(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when disabled, got %d: %s", rec.Code, rec.Body.String())
	}

	// Calling with force should succeed
	reqForce := httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/orchestrate", strings.NewReader(`{"force":true}`))
	recForce := httptest.NewRecorder()
	h.HandleProjectDetail(recForce, reqForce)
	if recForce.Code != http.StatusOK {
		t.Fatalf("expected 200 with force=true, got %d: %s", recForce.Code, recForce.Body.String())
	}

	var res models.OrchestrationResult
	if err := json.Unmarshal(recForce.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Launched) != 1 {
		t.Fatalf("expected 1 launched task, got %d", len(res.Launched))
	}
	if res.MaxConcurrent != 2 {
		t.Fatalf("expected max concurrent 2, got %d", res.MaxConcurrent)
	}
}
