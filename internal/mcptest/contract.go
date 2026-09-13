// Package mcptest shares the naming contract checks across HTTP and stdio tests.
package mcptest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/models"
)

// AssertNaming checks the entire catalog, successful calls, and rejection of
// every legacy name using valid arguments so validation cannot mask dispatch.
func AssertNaming(t *testing.T, ctx context.Context, session *mcp.ClientSession, database *db.DB, task *models.Task, connect func() *mcp.ClientSession) {
	t.Helper()
	if session.InitializeResult().ServerInfo.Name != "sectile" {
		t.Fatal("server identity is not sectile")
	}
	args := map[string]map[string]any{
		"get_task":            {"taskKey": task.ID},
		"transition_stage":    {"taskKey": task.ID, "stage": "clarified", "note": "Canonical contract"},
		"add_comment":         {"taskKey": task.ID, "body": "Canonical contract"},
		"list_tasks":          {"projectId": "default"},
		"get_project_context": {"projectId": "default"},
		"list_projects":       {},
		"start_run":           {"taskKey": task.ID, "skill": "implement"},
		"finish_run":          {"taskKey": task.ID, "status": "completed", "note": "Canonical contract"},
	}
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != len(args) {
		t.Fatalf("unexpected catalog size: %d", len(list.Tools))
	}
	seen := map[string]bool{}
	for _, tool := range list.Tools {
		if _, ok := args[tool.Name]; !ok || seen[tool.Name] {
			t.Fatalf("unexpected tool %s", tool.Name)
		}
		seen[tool.Name] = true
		if strings.Contains(tool.Description, "taskflow_") || tool.InputSchema == nil {
			t.Fatalf("invalid metadata for %s", tool.Name)
		}
	}
	call := func(name string) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args[name]})
		if err != nil || result.IsError {
			t.Fatalf("%s: %v %v", name, result, err)
		}
		return result
	}
	result := call("start_run")
	raw, _ := json.Marshal(result.StructuredContent)
	var run models.TaskActivity
	if err := json.Unmarshal(raw, &run); err != nil || run.ID == "" {
		t.Fatalf("invalid run: %s %v", raw, err)
	}
	args["start_run"]["runId"] = run.ID
	args["finish_run"]["runId"] = run.ID
	call("start_run")
	for _, name := range []string{"get_task", "get_project_context", "list_tasks", "list_projects"} {
		call(name)
	}
	snapshot := func() string {
		t.Helper()
		current, err := database.GetTaskByID(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		comments, err := database.GetTaskComments(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		activities, err := database.GetTaskActivities(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal([]any{current, comments, activities})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	before := snapshot()
	for name, input := range args {
		legacySession := connect()
		_, err := legacySession.CallTool(ctx, &mcp.CallToolParams{Name: "taskflow_" + name, Arguments: input})
		legacySession.Close()
		if err == nil || !strings.Contains(err.Error(), "unknown tool") {
			t.Fatalf("legacy %s was not rejected as unknown: %v", name, err)
		}
	}
	if after := snapshot(); after != before {
		t.Fatal("legacy requests mutated workflow state")
	}
	active, err := database.GetActivityByID(run.ID)
	if err != nil || active.Status != "running" {
		t.Fatal("read/transition/legacy finish changed run ownership")
	}
	call("add_comment")
	call("transition_stage")
	call("finish_run")
	active, err = database.GetActivityByID(run.ID)
	if err != nil || active.Status != "completed" {
		t.Fatal("canonical completion failed")
	}
}
