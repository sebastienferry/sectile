package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
)

func TestDesktopFinishesCanonicalRun(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Desktop completion"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartRemoteRun(task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECTILE_SERVER_TOKEN", "desktop-secret")
	server := httptest.NewServer(handlers.NewHandler(database).MCPHandler())
	defer server.Close()
	daemon := &agentDaemon{serverURL: server.URL, token: "desktop-secret"}
	if err := daemon.finishDesktopRun(context.Background(), task.ID, run.ID, "completed"); err != nil {
		t.Fatal(err)
	}
	finished, err := database.GetActivityByID(run.ID)
	if err != nil || finished.Status != "completed" {
		t.Fatalf("original run not completed: %v %v", finished, err)
	}
	activities, err := database.GetTaskActivities(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, activity := range activities {
		if activity.SkillID == "remote_run" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly the original remote run, got %d", count)
	}
	unchanged, err := database.GetTaskByID(task.ID)
	if err != nil || unchanged.Status != task.Status {
		t.Fatal("completion advanced task stage")
	}
}

func TestMCPBridgeRejectsIncompatibleCatalog(t *testing.T) {
	for _, name := range []string{"sectile_get_task", "sectile_get_task", "get_task"} {
		t.Run(name, func(t *testing.T) {
			upstream := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1"}, nil)
			upstream.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				t.Error("incompatible upstream tool executed")
				return &mcp.CallToolResult{}, nil
			})
			server := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return upstream }, nil))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := runMCPCommand(ctx, []string{"--url", server.URL})
			if err == nil || !strings.Contains(err.Error(), "incompatible Sectile MCP catalog") {
				t.Fatalf("catalog accepted or wrong error: %v", err)
			}
		})
	}
}
