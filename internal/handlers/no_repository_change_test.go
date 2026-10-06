package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/models"
)

// transition_stage takes the statement that a task changed no repository, and
// refuses it next to a pull request, which would contradict it (#584).
func TestTransitionStageTakesNoRepositoryChange(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Configuration only", Labels: []string{"#specified"}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	t.Cleanup(srv.Close)
	session := mcpSession(t, srv.URL, key)
	t.Cleanup(func() { session.Close() })

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "transition_stage", Arguments: map[string]any{
		"taskKey": task.ID, "stage": "implemented", "note": "done", "noRepositoryChange": true, "prUrl": "https://github.com/o/r/pull/1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(textOf(result), "not both") {
		t.Fatalf("the statement with a prUrl was accepted: %+v", result)
	}

	callTool(t, session, "transition_stage", map[string]any{
		"taskKey": task.ID, "stage": "implemented", "note": "webhook fixed through the API", "noRepositoryChange": true,
	})
	got, err := database.GetTaskByID(task.ID)
	if err != nil || database.StageOfTask(got) != "implemented" {
		t.Fatalf("stage after the statement = %q, %v; want implemented", database.StageOfTask(got), err)
	}
}

func textOf(result *mcp.CallToolResult) string {
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}
