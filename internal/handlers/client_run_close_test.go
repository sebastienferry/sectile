package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

// clientRunServer serves the two routes that close a client run: the board's
// cancel-run and the activities view's cancel.
func clientRunServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tasks/", h.HandleTaskDetail)
	mux.HandleFunc("/api/activities/", h.HandleActivityDetail)
	server := httptest.NewServer(h.EnableCORS(h.RequireSession(mux)))
	t.Cleanup(server.Close)
	return server
}

// A client-created run has no agent to stop, so the board closes it the way a
// disconnection would: owner or admin only, with the disconnect note, which
// leaves the owner free to report the real outcome afterwards (#319).
func TestBoardClosesAClientRunForItsOwnerOrAnAdmin(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := clientRunServer(t, h)
	_, alice := account(t, database, "alice@example.com") // admin
	_, bob := account(t, database, "bob@example.com")
	carolID, carol := account(t, database, "carol@example.com")
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Client run", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartRemoteRunBy(carolID, task.ID, "clarify", "")
	if err != nil || run.Action != db.RunActionClient {
		t.Fatalf("client run: %+v %v", run, err)
	}

	path := "/api/tasks/" + task.ID + "/cancel-run"
	body := `{"runId":"` + run.ID + `"}`
	if status, text := call(t, server, bob, http.MethodPost, path, body); status != http.StatusForbidden || !strings.Contains(text, msgNotOwner) {
		t.Fatalf("a colleague closed the run: %d %s", status, text)
	}
	if status, text := call(t, server, carol, http.MethodPost, path, body); status != http.StatusOK {
		t.Fatalf("the owner could not close the run: %d %s", status, text)
	}
	closed, _ := database.GetActivityByID(run.ID)
	if closed.Status != "canceled" || !strings.Contains(closed.Summary, models.RunDisconnectNote) {
		t.Fatalf("closed run = %q %q, want canceled with the disconnect note", closed.Status, closed.Summary)
	}
	// The owner may still say how it really ended.
	if recovered, err := database.FinishRemoteRunAs(db.Actor{ID: carolID}, false, task.ID, run.ID, "completed", "it did finish"); err != nil || recovered.Status != "completed" {
		t.Fatalf("the owner could not report the outcome after a board close: %+v %v", recovered, err)
	}

	// An ownerless client run predates ownership: an admin's to close.
	legacy, err := database.StartRemoteRun(task.ID, "clarify", "")
	if err != nil {
		t.Fatal(err)
	}
	legacyBody := `{"runId":"` + legacy.ID + `"}`
	if status, text := call(t, server, bob, http.MethodPost, path, legacyBody); status != http.StatusForbidden {
		t.Fatalf("a member closed an ownerless run: %d %s", status, text)
	}
	if status, text := call(t, server, alice, http.MethodPost, path, legacyBody); status != http.StatusOK {
		t.Fatalf("an admin could not close an ownerless run: %d %s", status, text)
	}
}

// The activities view cancelled a client run as a plain activity: no owner
// check, no hand-back, and a session that still thought it owned the run.
func TestActivitiesCancelOfAClientRunFollowsOwnership(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := clientRunServer(t, h)
	_, _ = account(t, database, "alice@example.com") // admin
	_, bob := account(t, database, "bob@example.com")
	carolID, carol := account(t, database, "carol@example.com")
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Client run", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartRemoteRunBy(carolID, task.ID, "clarify", "")
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/activities/" + run.ID + "/cancel"
	if status, text := call(t, server, bob, http.MethodPost, path, ""); status != http.StatusForbidden {
		t.Fatalf("a colleague cancelled the run: %d %s", status, text)
	}
	if still, _ := database.GetActivityByID(run.ID); still.Status != "running" {
		t.Fatalf("the refused cancel changed the run to %q", still.Status)
	}
	if status, text := call(t, server, carol, http.MethodPost, path, ""); status != http.StatusOK {
		t.Fatalf("the owner could not cancel: %d %s", status, text)
	}
	canceled, _ := database.GetActivityByID(run.ID)
	if canceled.Status != "canceled" || !strings.Contains(canceled.Summary, "activities view") {
		t.Fatalf("canceled run = %q %q", canceled.Status, canceled.Summary)
	}
	// A cancellation somebody chose is final, unlike a disconnection.
	if _, err := database.FinishRemoteRunAs(db.Actor{ID: carolID}, false, task.ID, run.ID, "completed", "no"); err == nil {
		t.Fatal("a deliberate cancellation was rewritten")
	}
}
