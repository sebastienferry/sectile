package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
)

// runConsoleTransport is the stdio bridge of a console the agent launched for
// a run: the workstation key, plus the run the console belongs to.
type runConsoleTransport struct{ token, runID string }

func (tr runConsoleTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+tr.token)
	if tr.runID != "" {
		r.Header.Set(agentprotocol.RunIDHeader, tr.runID)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func consoleSession(t *testing.T, endpoint, key, runID string) *mcp.ClientSession {
	t.Helper()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "console", Version: "1"}, nil).Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: &http.Client{Transport: runConsoleTransport{key, runID}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// A wait declared before a server restart belongs to a session nothing serves
// any more. The console's client initializes a new one, and its next call,
// which names the console's run, ends the wait; calls that name another run,
// or none, leave it (#498).
func TestLaunchedConsoleEndsItsWaitAfterReinitializing(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Asks", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	launched, err := database.StartAgentRun(task.ID, "clarify", db.RunLaunch{Mode: models.SkillModeInteractive, UserID: ImplicitUser})
	if err != nil {
		t.Fatal(err)
	}
	// What the declaration left behind: the wait, tied to a session of an
	// instance that restarted since.
	if _, applied, err := database.ReportSessionRunWaitingAs(db.Actor{ID: ImplicitUser}, false, "dead-instance.session-1", task.ID, launched.ID, true); err != nil || !applied {
		t.Fatalf("declaring the wait: applied=%v, %v", applied, err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	// A cleanup rather than a defer: the sessions, closed by their own cleanups,
	// must end their event streams before the server waits for its connections.
	t.Cleanup(srv.Close)

	callTool(t, consoleSession(t, srv.URL, key, ""), "get_task", map[string]any{"taskKey": task.ID})
	callTool(t, consoleSession(t, srv.URL, key, "another-run"), "get_task", map[string]any{"taskKey": task.ID})
	if waitingSinceOf(t, database, launched.ID) == nil {
		t.Fatal("a call that did not name the run ended its wait")
	}

	callTool(t, consoleSession(t, srv.URL, key, launched.ID), "get_task", map[string]any{"taskKey": task.ID})
	if waitingSinceOf(t, database, launched.ID) != nil {
		t.Fatal("the console's next call after re-initializing left its run waiting")
	}
}

// report_waiting itself is not a sign that the console moved on: a console
// that declares a wait from its new session keeps it.
func TestLaunchedConsoleReportingAgainKeepsItsWait(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Asks", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	launched, err := database.StartAgentRun(task.ID, "clarify", db.RunLaunch{Mode: models.SkillModeInteractive, UserID: ImplicitUser})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := database.ReportSessionRunWaitingAs(db.Actor{ID: ImplicitUser}, false, "dead-instance.session-1", task.ID, launched.ID, true); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	t.Cleanup(srv.Close)
	console := consoleSession(t, srv.URL, key, launched.ID)
	callTool(t, console, "report_waiting", map[string]any{"taskKey": task.ID, "runId": launched.ID, "waiting": true})
	if waitingSinceOf(t, database, launched.ID) == nil {
		t.Fatal("report_waiting from the new session ended the wait")
	}
	// The new session now owns the wait, and its next call ends it as usual.
	callTool(t, console, "get_task", map[string]any{"taskKey": task.ID})
	if waitingSinceOf(t, database, launched.ID) != nil {
		t.Fatal("the declaring session's next call left the wait")
	}
}
