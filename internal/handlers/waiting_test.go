package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// waitingRun gives the test a live remote run reachable through the HTTP
// surface, which is the only way the hook report ever arrives.
func waitingRun(t *testing.T) (*Handler, string, func()) {
	t.Helper()
	h, database, cleanup := setupTestHandler(t)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Waiting"})
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Blocked", ProjectID: project.ID})
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	run, err := database.StartAgentRemoteRun(task.ID, "implement")
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	return h, run.ID, cleanup
}

func postWaiting(h *Handler, runID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/activities/"+runID+"/waiting", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleActivityDetail(rec, req)
	return rec
}

func TestWaitingSubActionSetsAndClears(t *testing.T) {
	h, runID, cleanup := waitingRun(t)
	defer cleanup()

	events := h.SubscribeEvents()
	defer h.UnsubscribeEvents(events)

	if rec := postWaiting(h, runID, `{"waiting":true}`); rec.Code != http.StatusOK {
		t.Fatalf("waiting report refused: %d %s", rec.Code, rec.Body.String())
	}
	// The UI learns of the wait through the event the run layer already emits;
	// without it a waiting run would only surface on the next poll. The run's
	// own start is broadcast from a goroutine, so it may still be in flight and
	// is skipped rather than mistaken for the one under test.
	deadline := time.After(2 * time.Second)
	for {
		var evt Event
		select {
		case evt = <-events:
		case <-deadline:
			t.Fatal("no task_updated event was emitted for the waiting run")
		}
		if evt.Type != "task_updated" || evt.Activity == nil {
			t.Fatalf("unexpected broadcast: %+v", evt)
		}
		if evt.Activity.WaitingSince != nil {
			break
		}
	}

	if rec := postWaiting(h, runID, `{"waiting":false}`); rec.Code != http.StatusOK {
		t.Fatalf("resumed report refused: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWaitingSubActionRejectsUnknownRunAndMalformedBody(t *testing.T) {
	h, runID, cleanup := waitingRun(t)
	defer cleanup()

	if rec := postWaiting(h, "no-such-run", `{"waiting":true}`); rec.Code != http.StatusNotFound {
		t.Fatalf("an unknown run answered %d instead of 404", rec.Code)
	}
	for _, body := range []string{``, `{}`, `{"waiting":"yes"}`, `not json`} {
		if rec := postWaiting(h, runID, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q answered %d instead of 400", body, rec.Code)
		}
	}
}
