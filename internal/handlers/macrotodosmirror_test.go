package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// commentTracker is a Jira site that keeps the comment Sectile owns on an
// epic, and accepts every write.
type commentTracker struct {
	tracker.BaseTicketingSystem
}

func (c *commentTracker) UpsertMarkedComment(ctx context.Context, req tracker.UpsertMarkedCommentRequest) (string, error) {
	return "1", nil
}

func postTodosMirror(t *testing.T, h *handlers.Handler, projectID, key string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/macros/"+key+"/todos-mirror", nil)
	h.HandleProjectDetail(rr, req)
	return rr
}

func TestTodosMirrorRepublishIsQueuedOrRefusedWithItsReason(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	database.TrackerRegistry().Register("jira", &commentTracker{tracker.BaseTicketingSystem{
		TrackerName:  "jira",
		Capabilities: []tracker.Capability{tracker.CapComment},
	}})
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	h := handlers.NewHandler(database)

	rr := postTodosMirror(t, h, project.ID, "PE-1")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d %s, want 202", rr.Code, rr.Body.String())
	}
	var out struct {
		Activity models.TaskActivity `json:"activity"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.Activity.Action != "Todos de PE-1 ➔ tracker" {
		t.Fatalf("activity %+v %v", out.Activity, err)
	}

	before, _ := database.GetActivities(project.ID, "", "", "", "", 100)
	rr = postTodosMirror(t, h, project.ID, "M-4")
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "restent dans Sectile") {
		t.Fatalf("got %d %s, want 400 with the reason", rr.Code, rr.Body.String())
	}
	if after, _ := database.GetActivities(project.ID, "", "", "", "", 100); len(after) != len(before) {
		t.Fatalf("a refusal queues nothing: %d then %d activities", len(before), len(after))
	}
}
