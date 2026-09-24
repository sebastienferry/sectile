package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/taskmcp"
)

// liveSessions reads the status endpoint rather than the registry, so the test
// covers what an operator actually sees.
func liveSessions(t *testing.T, h *Handler) []taskmcp.SessionView {
	t.Helper()
	recorder := httptest.NewRecorder()
	h.HandleMCPSessions(recorder, httptest.NewRequest("GET", "/api/mcp/sessions", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("sessions endpoint status = %d", recorder.Code)
	}
	var payload struct {
		Sessions []taskmcp.SessionView `json:"sessions"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode sessions: %v (%s)", err, recorder.Body.String())
	}
	return payload.Sessions
}

// awaitStatus waits for a status the server sets asynchronously when a client
// disappears. Polling keeps the test honest about the ordering the production
// path actually has: the client is gone before the run is closed.
func awaitStatus(t *testing.T, database *db.DB, runID, want string) *models.TaskActivity {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last *models.TaskActivity
	for time.Now().Before(deadline) {
		activity, err := database.GetActivityByID(runID)
		if err != nil {
			t.Fatalf("read run %s: %v", runID, err)
		}
		last = activity
		if activity != nil && activity.Status == want {
			return activity
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s is %+v, want status %q", runID, last, want)
	return nil
}

func startRun(t *testing.T, ctx context.Context, session *mcp.ClientSession, taskKey string) models.TaskActivity {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "start_run",
		Arguments: map[string]any{"taskKey": taskKey, "skill": "pickup"}})
	if err != nil || result.IsError {
		t.Fatalf("start_run: %+v %v", result, err)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var run models.TaskActivity
	if err := json.Unmarshal(raw, &run); err != nil || run.ID == "" {
		t.Fatalf("invalid run: %s %v", raw, err)
	}
	return run
}

// A disconnecting client must not leave its task active: the session that
// started the run owns it, and the server closes what the client no longer can.
func TestDisconnectedSessionClosesItsRun(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Owned run", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	ctx := context.Background()

	transport := &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: testTokenTransport{key}}}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "sectile-stdio", Title: "desktop/1", Version: "1"}, nil).Connect(ctx, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := startRun(t, ctx, session, task.ID)

	// While the client is connected the server reports it, and its run stays
	// active: observing a session never invents or ends work.
	sessions := liveSessions(t, h)
	if len(sessions) != 1 {
		t.Fatalf("sessions = %+v, want one", sessions)
	}
	if sessions[0].Client != "sectile-stdio" || sessions[0].Title != "desktop/1" {
		t.Fatalf("session = %+v, want the client's own identity", sessions[0])
	}
	if len(sessions[0].Runs) != 1 || sessions[0].Runs[0] != run.ID {
		t.Fatalf("session runs = %+v, want %s", sessions[0].Runs, run.ID)
	}
	if active, err := database.GetActivityByID(run.ID); err != nil || active.Status != "running" {
		t.Fatalf("run before disconnection: %+v %v", active, err)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("close session: %v", err)
	}
	closed := awaitStatus(t, database, run.ID, "canceled")
	if closed.Summary == "" {
		t.Fatalf("closed run carries no explanation: %+v", closed)
	}
	if remaining := liveSessions(t, h); len(remaining) != 0 {
		t.Fatalf("sessions after disconnection = %+v, want none", remaining)
	}
}

// A run the client finished itself keeps the outcome the client reported.
func TestReportedRunKeepsItsOutcome(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Reported run", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	ctx := context.Background()

	transport := &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: testTokenTransport{key}}}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "sectile-stdio", Version: "1"}, nil).Connect(ctx, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := startRun(t, ctx, session, task.ID)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "finish_run",
		Arguments: map[string]any{"taskKey": task.ID, "runId": run.ID, "status": "completed", "note": "Done"}})
	if err != nil || result.IsError {
		t.Fatalf("finish_run: %+v %v", result, err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close session: %v", err)
	}

	// The disconnection must not overwrite a reported outcome, so the status
	// has to still be "completed" after the server has finished reacting.
	time.Sleep(200 * time.Millisecond)
	final, err := database.GetActivityByID(run.ID)
	if err != nil || final.Status != "completed" {
		t.Fatalf("reported run was overwritten: %+v %v", final, err)
	}
}

// Concurrent clients are told apart, so one client's departure never closes
// another's run.
func TestSessionsAreIndependent(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Two clients", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	ctx := context.Background()
	connect := func(title string) *mcp.ClientSession {
		t.Helper()
		transport := &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: testTokenTransport{key}}}
		session, err := mcp.NewClient(&mcp.Implementation{Name: "sectile-stdio", Title: title, Version: "1"}, nil).Connect(ctx, transport, nil)
		if err != nil {
			t.Fatal(err)
		}
		return session
	}

	first, second := connect("first"), connect("second")
	firstRun := startRun(t, ctx, first, task.ID)
	secondRun := startRun(t, ctx, second, task.ID)
	if firstRun.ID == secondRun.ID {
		t.Fatalf("both clients share run %s", firstRun.ID)
	}
	if sessions := liveSessions(t, h); len(sessions) != 2 {
		t.Fatalf("sessions = %+v, want two", sessions)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first session: %v", err)
	}
	awaitStatus(t, database, firstRun.ID, "canceled")
	if survivor, err := database.GetActivityByID(secondRun.ID); err != nil || survivor.Status != "running" {
		t.Fatalf("second client's run = %+v %v, want it still running", survivor, err)
	}

	// Closing the survivor here, rather than deferring it, keeps the server's
	// reaction inside the test: the database must still be open when it lands.
	if err := second.Close(); err != nil {
		t.Fatalf("close second session: %v", err)
	}
	awaitStatus(t, database, secondRun.ID, "canceled")
}

// A run dispatched by an agent keeps its launcher's ownership: the agent's
// supervisor reports the real process exit, so a disconnecting MCP client must
// not close an execution that is still running.
func TestLauncherRunKeepsItsOwner(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Dispatched run", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := database.StartAgentRemoteRun(task.ID, "pickup")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	ctx := context.Background()

	transport := &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: testTokenTransport{key}}}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "sectile-stdio", Version: "1"}, nil).Connect(ctx, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "start_run",
		Arguments: map[string]any{"taskKey": task.ID, "skill": "pickup", "runId": dispatched.ID}})
	if err != nil || result.IsError {
		t.Fatalf("start_run reusing a launcher run: %+v %v", result, err)
	}
	if sessions := liveSessions(t, h); len(sessions) != 1 || len(sessions[0].Runs) != 0 {
		t.Fatalf("sessions = %+v, want one session owning nothing", sessions)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close session: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	survivor, err := database.GetActivityByID(dispatched.ID)
	if err != nil || survivor.Status != "running" {
		t.Fatalf("launcher run = %+v %v, want it still running", survivor, err)
	}
}

func TestMCPSessionsEndpointRejectsWrites(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()
	recorder := httptest.NewRecorder()
	h.HandleMCPSessions(recorder, httptest.NewRequest("POST", "/api/mcp/sessions", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}

func TestMCPSilenceNoticeOverride(t *testing.T) {
	if got := mcpSilenceNotice(); got != defaultMCPSilenceNotice {
		t.Fatalf("default timeout = %s, want %s", got, defaultMCPSilenceNotice)
	}
	t.Setenv("SECTILE_MCP_SESSION_TIMEOUT", "90s")
	if got := mcpSilenceNotice(); got != 90*time.Second {
		t.Fatalf("configured timeout = %s, want 90s", got)
	}
	// An unusable value must not silently remove the silence observation.
	for _, raw := range []string{"soon", "-1m", "0"} {
		t.Setenv("SECTILE_MCP_SESSION_TIMEOUT", raw)
		if got := mcpSilenceNotice(); got != defaultMCPSilenceNotice {
			t.Fatalf("timeout for %q = %s, want the default", raw, got)
		}
	}
}

func TestMCPAbandonAfterOverride(t *testing.T) {
	if got := mcpAbandonAfter(4 * time.Hour); got != defaultMCPAbandonAfter {
		t.Fatalf("default abandon bound = %s, want %s", got, defaultMCPAbandonAfter)
	}
	t.Setenv("SECTILE_MCP_SESSION_ABANDON_AFTER", "12h")
	if got := mcpAbandonAfter(4 * time.Hour); got != 12*time.Hour {
		t.Fatalf("configured abandon bound = %s, want 12h", got)
	}
	// A session is always remarked upon before it is closed.
	t.Setenv("SECTILE_MCP_SESSION_ABANDON_AFTER", "1h")
	if got := mcpAbandonAfter(4 * time.Hour); got != 4*time.Hour {
		t.Fatalf("abandon bound below the silence bound = %s, want it raised to 4h", got)
	}
	for _, raw := range []string{"never", "-1m", "0"} {
		t.Setenv("SECTILE_MCP_SESSION_ABANDON_AFTER", raw)
		if got := mcpAbandonAfter(4 * time.Hour); got != defaultMCPAbandonAfter {
			t.Fatalf("abandon bound for %q = %s, want the default", raw, got)
		}
	}
}
