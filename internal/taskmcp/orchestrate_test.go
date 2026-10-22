package taskmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func TestOrchestrateProjectMCPTool(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, nil
	})

	project, err := database.CreateProject(models.CreateProjectRequest{
		Name:                      "MCPOrchestrator",
		IssueTracker:              "local",
		OrchestratorEnabled:       true,
		OrchestratorMaxConcurrent: 2,
		OrchestratorPrompt:        "Only implement backend service changes.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = database.CreateTask(models.CreateTaskRequest{
		ProjectID:   project.ID,
		Title:       "Setup Backend Service",
		Description: "Detailed task description for automated autonomous development.",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := call(t, database, "orchestrate_project", map[string]any{
		"projectId": project.ID,
	})
	if err != nil {
		t.Fatalf("call orchestrate_project failed: %v", err)
	}

	if res["projectId"] != project.ID {
		t.Fatalf("expected projectId %s, got %v", project.ID, res["projectId"])
	}
	launched, ok := res["launched"].([]any)
	if !ok || len(launched) != 1 {
		t.Fatalf("expected 1 launched task in MCP response, got %+v", res["launched"])
	}
}

func TestOrchestrateProjectMCPDispatchesAsCaller(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dispatched := make(chan agentprotocol.Operation, 1)
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action == "execute_skill" {
			dispatched <- op
		}
		return nil, nil
	})
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Caller orchestration", IssueTracker: "local", OrchestratorEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Validate requests", Description: "Reject invalid requests with a clear error response."})
	if err != nil {
		t.Fatal(err)
	}
	database.SetAgentSelection(func(user, projectID, device string) (string, error) {
		if user != tester.UserID || projectID != project.ID {
			return "", fmt.Errorf("wrong caller route: %q / %q", user, projectID)
		}
		return "test-workstation", nil
	})
	if _, err := call(t, database, "orchestrate_project", map[string]any{"projectId": project.ID}); err != nil {
		t.Fatal(err)
	}
	select {
	case op := <-dispatched:
		if op.UserID != tester.UserID || op.TaskID != task.ID || op.DeviceID != "test-workstation" {
			t.Fatalf("unexpected dispatch: %+v", op)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MCP caller's workstation never received a launch")
	}
}

func TestOrchestrateProjectMCPRefusesOverrides(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return nil, nil
	})
	disabled, err := database.CreateProject(models.CreateProjectRequest{Name: "Disabled MCP orchestration", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateTask(models.CreateTaskRequest{ProjectID: disabled.ID, Title: "Validate requests", Description: "Reject invalid requests with a clear error response."}); err != nil {
		t.Fatal(err)
	}
	// An agent cannot enable the orchestrator nor raise the project's cap.
	for _, args := range []map[string]any{
		{"projectId": disabled.ID},
		{"projectId": disabled.ID, "force": true},
		{"projectId": disabled.ID, "maxConcurrent": 10},
	} {
		if res, err := call(t, database, "orchestrate_project", args); err == nil {
			t.Fatalf("orchestrate_project accepted %v: %+v", args, res)
		}
	}
	if count, err := database.ActiveRunsCountForProject(disabled.ID); err != nil || count != 0 {
		t.Fatalf("refused calls reserved slots: %d, %v", count, err)
	}

	capped, err := database.CreateProject(models.CreateProjectRequest{Name: "Capped MCP orchestration", IssueTracker: "local", OrchestratorEnabled: true, OrchestratorMaxConcurrent: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Validate requests", "Validate responses"} {
		if _, err := database.CreateTask(models.CreateTaskRequest{ProjectID: capped.ID, Title: title, Description: "Reject invalid payloads with a clear error response."}); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = call(t, database, "orchestrate_project", map[string]any{"projectId": capped.ID, "maxConcurrent": 10})
	if count, err := database.ActiveRunsCountForProject(capped.ID); err != nil || count > 1 {
		t.Fatalf("a caller raised the project cap of 1: %d, %v", count, err)
	}
}
