package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/taskmcp"
)

// openPostgresInstance opens a store on the test database as one serving
// instance, advertising the internal listener of node. It skips without
// SECTILE_TEST_POSTGRES_DSN, like the store's own PostgreSQL tests; the two
// packages share that database, so run them with -p 1.
func openPostgresInstance(t *testing.T) *db.DB {
	t.Helper()
	dsn := os.Getenv("SECTILE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SECTILE_TEST_POSTGRES_DSN to run the PostgreSQL tests")
	}
	d, err := db.Open(db.Config{Driver: db.DriverPostgres, DSN: dsn})
	if err != nil {
		t.Fatalf("opening PostgreSQL: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// startPostgresNode makes a store a serving instance behind its own endpoints,
// found by the others through server_instances.
func startPostgresNode(t *testing.T, d *db.DB) (*mcpNode, func()) {
	t.Helper()
	n := newMCPNode(t, nil, d)
	d.SetInstanceAddress(n.internal.URL)
	stop, err := d.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	n.h.setMCPCluster(d, mcpClusterToken, nil)
	return n, stop
}

// Two instances on one PostgreSQL database, found through the table #403
// keeps: a session held by A is served through B, dies with A, and the run it
// left behind is still its owner's to report on once reclaimed (#408).
func TestPostgresMCPSessionAcrossTwoInstances(t *testing.T) {
	t.Setenv("SECTILE_SERVER_TOKEN", mcpClientKey)
	a, stopA := startPostgresNode(t, openPostgresInstance(t))
	b, _ := startPostgresNode(t, openPostgresInstance(t))

	project, err := a.db.CreateProject(models.CreateProjectRequest{Name: "Replicas " + a.id()[:8],
		IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := a.db.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "across replicas"})
	if err != nil {
		t.Fatal(err)
	}

	session := connectThrough(t, initializeOn(a, b), nil)
	defer session.Close()
	if owner := taskmcp.SessionOwner(session.ID()); owner != a.id() {
		t.Fatalf("session owned by %q, want A", owner)
	}
	finished := startRunThrough(t, session, task.ID)
	if runs := sessionRuns(a)[session.ID()]; len(runs) != 1 || runs[0] != finished {
		t.Fatalf("A's session runs = %v, want the run started through B", runs)
	}
	callTool(t, session, "finish_run", map[string]any{"taskKey": task.ID, "runId": finished, "status": "completed", "note": "done"})
	lost := startRunThrough(t, session, task.ID)

	views := readSessions(t, b)
	listed := false
	for _, view := range views.Sessions {
		listed = listed || (view.ID == session.ID() && view.Instance == a.id())
	}
	if !listed || len(views.Unreachable) != 0 {
		t.Errorf("B's sessions view = %+v, want A's session listed", views)
	}

	// A stops serving: its row goes, so the session is found nowhere.
	stopA()
	status, body := rawMCP(t, http.MethodPost, b.public.URL+"/mcp",
		map[string]string{"Authorization": "Bearer " + mcpClientKey, mcpSessionHeader: session.ID()}, rawPing)
	if status != http.StatusNotFound {
		t.Fatalf("a request for a dead owner's session answered %d %s, want 404", status, body)
	}

	// The next instance to start reclaims what A left running.
	openPostgresInstance(t)
	run, _ := b.db.GetActivityByID(lost)
	if run == nil || run.Status != "canceled" || !strings.Contains(run.Summary, models.RunDisconnectNote) {
		t.Fatalf("lost run = %+v, want canceled with the disconnect note", run)
	}

	again := connectThrough(t, func(*http.Request) *httptest.Server { return b.public }, nil)
	defer again.Close()
	callTool(t, again, "finish_run", map[string]any{"taskKey": task.ID, "runId": lost, "status": "completed", "note": "reported after all"})
	if run, _ := b.db.GetActivityByID(lost); run == nil || run.Status != "completed" || run.Summary != "reported after all" {
		t.Fatalf("recovered run = %+v, want the owner's report", run)
	}
}
