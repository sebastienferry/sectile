package taskmcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

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
