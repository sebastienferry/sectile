package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// silence remains, and silence is not proof of death — the same silence a stage
// that compiles or waits for its owner produces. The run therefore stays open
// and is merely remarked upon, and its owner can still report the outcome. This
// exercises the documented chain end to end — client, stdio bridge process,
// loopback proxy, server — because the proxy is the hop that must carry the
// session identifier without knowing what it means.
func TestKilledBridgeLeavesItsRunOpenAndRemarkedUpon(t *testing.T) {
	// The handler reads the silence bound when it is built, so this must
	// precede it.
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
	daemon := &agentDaemon{link: serverLink{serverURL: upstream.URL, token: "test-token"}}
	if err := daemon.startLocalProxy(ctx); err != nil {
		t.Fatal(err)
	}
	defer daemon.loopback.server.Close()

	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Killed bridge", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestMCPStdioHelper$")
	command.Env = append(os.Environ(), "SECTILE_MCP_HELPER=1", "SECTILE_AGENT_URL="+daemon.loopback.url, "SECTILE_AGENT_TOKEN="+daemon.link.token)
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
		if activity != nil && activity.Status != "running" {
			t.Fatalf("silence ended run %s as %q: %+v", run.ID, activity.Status, activity)
		}
		if activity != nil && strings.Contains(activity.Summary, models.RunSilencePrefix) {
			// The observation is recorded; the run itself is untouched and its
			// owner remains free to report the outcome it really reached.
			if _, err := database.FinishRemoteRun(task.ID, run.ID, "completed", "reported after the silence"); err != nil {
				t.Fatalf("finish_run after a silence: %v", err)
			}
			finished, _ := database.GetActivityByID(run.ID)
			if finished.Status != "completed" || !strings.Contains(finished.Summary, models.RunSilencePrefix) {
				t.Fatalf("finished run = %q/%q, want both halves of the story", finished.Status, finished.Summary)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s is %+v, want the silence remarked upon", run.ID, activity)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
