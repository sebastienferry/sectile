package db

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func orchestratorTestDB(t *testing.T) (*DB, *models.Project) {
	t.Helper()
	d, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, nil
	})
	project, err := d.CreateProject(models.CreateProjectRequest{
		Name:                      "Orchestrated",
		IssueTracker:              "local",
		OrchestratorEnabled:       true,
		OrchestratorMaxConcurrent: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d, project
}

func TestEvaluateTaskEligibility(t *testing.T) {
	d, _ := orchestratorTestDB(t)

	// Short title
	ok, reason := d.EvaluateTaskEligibility(&models.Task{Title: "hi", Description: "Valid technical description here."})
	if ok || reason == "" {
		t.Fatalf("expected ineligible for short title, got %v (%s)", ok, reason)
	}

	// Short description
	ok, reason = d.EvaluateTaskEligibility(&models.Task{Title: "Valid Title", Description: "short"})
	if ok || reason == "" {
		t.Fatalf("expected ineligible for short description, got %v (%s)", ok, reason)
	}

	// Blocker label
	ok, reason = d.EvaluateTaskEligibility(&models.Task{
		Title:       "Valid Title",
		Description: "Detailed technical description for autonomous implementation.",
		Labels:      []string{"feature", "blocked"},
	})
	if ok || reason == "" {
		t.Fatalf("expected ineligible for blocked label, got %v (%s)", ok, reason)
	}

	// Eligible task
	ok, reason = d.EvaluateTaskEligibility(&models.Task{
		Title:       "Implement User Profile Screen",
		Description: "Build the user profile screen according to design specifications and integrate REST endpoints.",
		Labels:      []string{"frontend"},
	})
	if !ok || reason != "" {
		t.Fatalf("expected eligible task, got %v (%s)", ok, reason)
	}
}

func TestEvaluateAndOrchestrateBacklog(t *testing.T) {
	ctx := context.Background()
	d, project := orchestratorTestDB(t)
	actor := Actor{ID: "admin", Name: "Admin"}

	// Create 3 tasks: 2 eligible, 1 blocked
	t1, err := d.CreateTask(models.CreateTaskRequest{
		ProjectID:   project.ID,
		Title:       "Task One Architecture",
		Description: "First task description with ample technical details for clarification and spec.",
	})
	if err != nil {
		t.Fatal(err)
	}

	t2, err := d.CreateTask(models.CreateTaskRequest{
		ProjectID:   project.ID,
		Title:       "Task Two Implementation",
		Description: "Second task description with ample technical details for clarification and spec.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = d.CreateTask(models.CreateTaskRequest{
		ProjectID:   project.ID,
		Title:       "Task Three Blocked",
		Description: "Third task description which is currently blocked by dependency.",
		Labels:      []string{"blocked"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Initial active runs count should be 0
	count, err := d.ActiveRunsCountForProject(project.ID)
	if err != nil || count != 0 {
		t.Fatalf("expected 0 active runs, got %d (err: %v)", count, err)
	}

	// Run orchestrator
	res, err := d.EvaluateAndOrchestrateBacklog(ctx, project.ID, actor, models.OrchestrateProjectRequest{})
	if err != nil {
		t.Fatalf("orchestrate failed: %v", err)
	}

	if res.MaxConcurrent != 2 {
		t.Fatalf("expected MaxConcurrent 2, got %d", res.MaxConcurrent)
	}
	if len(res.Launched) != 2 {
		t.Fatalf("expected 2 launched tasks, got %d", len(res.Launched))
	}
	if res.Launched[0].ID != t1.ID || res.Launched[1].ID != t2.ID {
		t.Fatalf("unexpected launched tasks: %+v", res.Launched)
	}
	if res.AvailableSlots != 0 {
		t.Fatalf("expected 0 available slots remaining, got %d", res.AvailableSlots)
	}

	// Running orchestrator again should yield 0 launches since capacity is full (2 active runs)
	res2, err := d.EvaluateAndOrchestrateBacklog(ctx, project.ID, actor, models.OrchestrateProjectRequest{})
	if err != nil {
		t.Fatalf("second orchestrate failed: %v", err)
	}
	if len(res2.Launched) != 0 {
		t.Fatalf("expected 0 launched on second run, got %d", len(res2.Launched))
	}
}

func TestOrchestratorDisabledRefusal(t *testing.T) {
	ctx := context.Background()
	d, _ := orchestratorTestDB(t)

	pDisabled, err := d.CreateProject(models.CreateProjectRequest{
		Name:                "DisabledOrchestrator",
		IssueTracker:        "local",
		OrchestratorEnabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	orchestratorTask(t, d, pDisabled.ID)
	actor := Actor{ID: "admin", Name: "Admin"}

	// No caller can run a disabled orchestrator: enabling it is a project setting.
	if _, err := d.EvaluateAndOrchestrateBacklog(ctx, pDisabled.ID, actor, models.OrchestrateProjectRequest{}); !errors.Is(err, ErrOrchestratorDisabled) {
		t.Fatalf("expected ErrOrchestratorDisabled, got %v", err)
	}
	if count, err := d.ActiveRunsCountForProject(pDisabled.ID); err != nil || count != 0 {
		t.Fatalf("a disabled orchestrator reserved slots: %d, %v", count, err)
	}

	enabled := true
	if _, err := d.UpdateProject(pDisabled.ID, models.UpdateProjectRequest{OrchestratorEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	res, err := d.EvaluateAndOrchestrateBacklog(ctx, pDisabled.ID, actor, models.OrchestrateProjectRequest{})
	if err != nil || res.ProjectID != pDisabled.ID || len(res.Launched) != 1 {
		t.Fatalf("enabled orchestrator: %+v, %v", res, err)
	}
}

func TestOrchestratorUnknownProject(t *testing.T) {
	d, _ := orchestratorTestDB(t)
	if _, err := d.EvaluateAndOrchestrateBacklog(context.Background(), "missing", Actor{ID: "admin"}, models.OrchestrateProjectRequest{}); !errors.Is(err, ErrOrchestratorProjectNotFound) {
		t.Fatalf("expected ErrOrchestratorProjectNotFound, got %v", err)
	}
}
