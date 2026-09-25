package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
)

func waitingSinceOf(t *testing.T, database *db.DB, runID string) *time.Time {
	t.Helper()
	activity, err := database.GetActivityByID(runID)
	if err != nil || activity == nil {
		t.Fatalf("read run %s: %v", runID, err)
	}
	return activity.WaitingSince
}

// A wait is declared by the model and ended by whatever the session does next,
// so a model that forgets to clear it cannot leave its run waiting forever.
func TestReportWaitingIsClearedByTheSessionsNextCall(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Asks", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	ctx := context.Background()
	session := mcpSession(t, srv.URL, key)
	defer session.Close()
	other := mcpSession(t, srv.URL, key)
	defer other.Close()
	run := startRun(t, ctx, session, task.ID)

	callTool(t, session, "report_waiting", map[string]any{"taskKey": task.ID, "runId": run.ID, "waiting": true})
	first := waitingSinceOf(t, database, run.ID)
	if first == nil {
		t.Fatal("report_waiting did not mark the run")
	}
	// Reporting again, a ping, and another session's call all leave the wait.
	callTool(t, session, "report_waiting", map[string]any{"taskKey": task.ID, "runId": run.ID, "waiting": true})
	if err := session.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}
	callTool(t, other, "get_task", map[string]any{"taskKey": task.ID})
	if again := waitingSinceOf(t, database, run.ID); again == nil || !again.Equal(*first) {
		t.Fatalf("the wait moved or ended without its session calling anything: %v -> %v", first, again)
	}

	// The session's next real call means it is no longer blocked.
	callTool(t, session, "get_task", map[string]any{"taskKey": task.ID})
	if waitingSinceOf(t, database, run.ID) != nil {
		t.Fatal("the session called a tool and its run still shows waiting")
	}

	// An explicit clear works as well.
	callTool(t, session, "report_waiting", map[string]any{"taskKey": task.ID, "runId": run.ID, "waiting": true})
	callTool(t, session, "report_waiting", map[string]any{"taskKey": task.ID, "runId": run.ID, "waiting": false})
	if waitingSinceOf(t, database, run.ID) != nil {
		t.Fatal("waiting false left the mark")
	}
}

// A run the session does not own, such as one a launcher handed over, loses its
// wait when the session goes away: nobody is left to be waited for.
func TestReportWaitingEndsWithItsSession(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Launched", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	launched, err := database.StartAgentRun(task.ID, "clarify", db.RunLaunch{Mode: models.SkillModeInteractive})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	transport := &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: testTokenTransport{key}}}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	callTool(t, session, "report_waiting", map[string]any{"taskKey": task.ID, "runId": launched.ID, "waiting": true})
	if waitingSinceOf(t, database, launched.ID) == nil {
		t.Fatal("report_waiting did not mark the launched run")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for waitingSinceOf(t, database, launched.ID) != nil {
		if time.Now().After(deadline) {
			t.Fatal("the wait outlived the session that declared it")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if run, _ := database.GetActivityByID(launched.ID); run.Status != "running" {
		t.Fatalf("a launched run was closed with the session: %q", run.Status)
	}
}

// readRunWaiting reads agent messages until a run_waiting arrives.
func readRunWaiting(t *testing.T, agent *websocket.Conn) agentprotocol.RunWaiting {
	t.Helper()
	_ = agent.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, raw, err := agent.ReadMessage()
		if err != nil {
			t.Fatalf("the owner's agent received no run_waiting: %v", err)
		}
		var message AgentMessage
		if err := json.Unmarshal(raw, &message); err != nil || message.Type != agentprotocol.RunWaitingType {
			continue
		}
		var payload agentprotocol.RunWaiting
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		return payload
	}
}

// The desktop banner is raised from the agent's run list, so a wait declared on
// the server has to reach the agent of the person it waits for.
func TestWaitingIsPushedToTheOwnersAgent(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)
	_, _ = account(t, database, "alice@example.com")
	carolID, _ := account(t, database, "carol@example.com")
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Carol's question", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartAgentRun(task.ID, "clarify", db.RunLaunch{UserID: carolID, Mode: models.SkillModeInteractive})
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := database.CreateAPIKey(carolID, "carol laptop", 0)
	if err != nil {
		t.Fatal(err)
	}
	agent := connectAgentAs(t, server, key, "default")
	deadline := time.Now().Add(3 * time.Second)
	for h.agentDispatcher.Lookup(carolID, "default") == nil {
		if time.Now().After(deadline) {
			t.Fatal("carol's agent never registered")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	marked := readRunWaiting(t, agent)
	if marked.RunID != run.ID || marked.WaitingSince == nil {
		t.Fatalf("the agent received %+v, want the run marked", marked)
	}
	if err := database.SetRemoteRunWaiting(run.ID, false); err != nil {
		t.Fatal(err)
	}
	if cleared := readRunWaiting(t, agent); cleared.RunID != run.ID || cleared.WaitingSince != nil {
		t.Fatalf("the agent received %+v, want the mark cleared", cleared)
	}
}
