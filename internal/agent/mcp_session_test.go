package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http/httptest"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
)

// A bridge that is killed never gets to report anything: no tool call, no
// protocol termination, not even a closed pipe the server could read. Only
// silence reveals it, which is what the session timeout is for. This exercises
// the documented chain end to end — client, stdio bridge process, loopback
// proxy, server — because the proxy is the hop that must carry the session
// identifier without knowing what it means.
func TestKilledBridgeClosesItsRun(t *testing.T) {
	// The handler reads the timeout when it is built, so this must precede it.
	t.Setenv("SECTILE_MCP_SESSION_TIMEOUT", "1s")
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	upstream := httptest.NewServer(handlers.NewHandler(database).MCPHandler())
	defer upstream.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	daemon := &agentDaemon{serverURL: upstream.URL, token: "test-token", loopbackToken: "session-secret"}
	if err := daemon.startLocalProxy(ctx); err != nil {
		t.Fatal(err)
	}
	defer daemon.httpServer.Close()

	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Killed bridge", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestMCPStdioHelper$")
	command.Env = append(os.Environ(), "SECTILE_MCP_HELPER=1", "SECTILE_AGENT_URL="+daemon.agentURL, "SECTILE_AGENT_TOKEN="+daemon.loopbackToken)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "stdio-test", Version: "1"}, nil).Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "start_run",
		Arguments: map[string]any{"taskKey": task.ID, "skill": "pickup"}})
	if err != nil || result.IsError {
		t.Fatalf("start_run: %+v %v", result, err)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var run models.TaskActivity
	if err := json.Unmarshal(raw, &run); err != nil || run.ID == "" {
		t.Fatalf("invalid run: %s %v", raw, err)
	}

	if command.Process == nil {
		t.Fatal("bridge process was never started")
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatalf("kill bridge: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		activity, err := database.GetActivityByID(run.ID)
		if err != nil {
			t.Fatalf("read run: %v", err)
		}
		if activity != nil && activity.Status == "canceled" {
			if activity.Summary == "" {
				t.Fatalf("closed run carries no explanation: %+v", activity)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s is %+v, want it closed after the bridge was killed", run.ID, activity)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
