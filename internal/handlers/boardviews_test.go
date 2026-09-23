package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
)

// boardViewServer serves the view routes and the task list behind the real
// session guard.
func boardViewServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/me/board-views", h.HandleBoardViews)
	mux.HandleFunc("/api/me/board-views/", h.HandleBoardViews)
	mux.HandleFunc("/api/tasks", h.HandleTasks)
	mux.HandleFunc("/api/tasks/facets", h.HandleTaskFacets)
	server := httptest.NewServer(h.EnableCORS(h.RequireSession(mux)))
	t.Cleanup(server.Close)
	return server
}

func TestBoardViewRoutes(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := boardViewServer(t, h)

	alpha, err := database.CreateProject(models.CreateProjectRequest{Name: "Alpha", Slug: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	beta, err := database.CreateProject(models.CreateProjectRequest{Name: "Beta", Slug: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.ImportOrUpdateTasks([]models.Task{
		{ProjectID: alpha.ID, Key: "#1", Title: "in view", Labels: []string{"Platform"}, Status: models.StatusToClarify, Priority: models.PriorityMedium},
		{ProjectID: alpha.ID, Key: "#2", Title: "other label", Labels: []string{"platform-x"}, Status: models.StatusToClarify, Priority: models.PriorityMedium},
		{ProjectID: beta.ID, Key: "#3", Title: "other project", Labels: []string{"platform"}, Status: models.StatusToClarify, Priority: models.PriorityMedium},
	}); err != nil {
		t.Fatal(err)
	}

	_, owner := account(t, database, "owner@example.com")
	_, intruder := account(t, database, "intruder@example.com")

	if status, _ := call(t, server, nil, http.MethodGet, "/api/me/board-views", ""); status != http.StatusUnauthorized {
		t.Errorf("anonymous list: status %d, want 401", status)
	}

	status, body := call(t, server, owner, http.MethodPost, "/api/me/board-views", `{"name":"Platform","projectIds":["`+alpha.ID+`"],"labels":["platform"]}`)
	if status != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", status, body)
	}
	var view models.BoardView
	if err := json.Unmarshal([]byte(body), &view); err != nil || view.ID == "" {
		t.Fatalf("create body %s: %v", body, err)
	}
	if strings.Contains(body, "userId") {
		t.Errorf("the owner leaked into the payload: %s", body)
	}

	for _, c := range []struct {
		body string
		want int
	}{
		{`{"name":" platform ","projectIds":["` + alpha.ID + `"]}`, http.StatusConflict},
		{`{"name":"","projectIds":["` + alpha.ID + `"]}`, http.StatusBadRequest},
		{`{"name":"Empty","projectIds":[]}`, http.StatusBadRequest},
		{`{"name":"Ghost","projectIds":["nope"]}`, http.StatusBadRequest},
		{`not json`, http.StatusBadRequest},
	} {
		if status, body := call(t, server, owner, http.MethodPost, "/api/me/board-views", c.body); status != c.want {
			t.Errorf("create %s: status %d (%s), want %d", c.body, status, body, c.want)
		}
	}

	status, body = call(t, server, owner, http.MethodGet, "/api/tasks?viewId="+view.ID, "")
	if status != http.StatusOK {
		t.Fatalf("tasks in view: status %d, body %s", status, body)
	}
	var tasks []models.Task
	_ = json.Unmarshal([]byte(body), &tasks)
	if len(tasks) != 1 || tasks[0].Title != "in view" {
		t.Errorf("tasks in view = %s, want only \"in view\"", body)
	}
	if status, _ := call(t, server, owner, http.MethodGet, "/api/tasks/facets?viewId="+view.ID, ""); status != http.StatusOK {
		t.Errorf("facets in view: status %d", status)
	}

	// Another user's view and a missing one give the same answer.
	_, missingBody := call(t, server, owner, http.MethodGet, "/api/me/board-views/missing", "")
	for _, req := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/me/board-views/" + view.ID, ""},
		{http.MethodPatch, "/api/me/board-views/" + view.ID, `{"name":"Mine"}`},
		{http.MethodDelete, "/api/me/board-views/" + view.ID, ""},
		{http.MethodGet, "/api/tasks?viewId=" + view.ID, ""},
		{http.MethodGet, "/api/tasks/facets?viewId=" + view.ID, ""},
	} {
		status, body := call(t, server, intruder, req.method, req.path, req.body)
		if status != http.StatusNotFound {
			t.Errorf("intruder %s %s: status %d, want 404", req.method, req.path, status)
		}
		if body != missingBody {
			t.Errorf("intruder %s %s: body %q, want the missing-view body %q", req.method, req.path, body, missingBody)
		}
	}
	status, body = call(t, server, intruder, http.MethodGet, "/api/me/board-views", "")
	if status != http.StatusOK || strings.TrimSpace(body) != "[]" {
		t.Errorf("intruder list: status %d, body %s, want an empty list", status, body)
	}

	status, body = call(t, server, owner, http.MethodPatch, "/api/me/board-views/"+view.ID, `{"projectIds":["`+alpha.ID+`","`+beta.ID+`"]}`)
	if status != http.StatusOK {
		t.Fatalf("patch: status %d, body %s", status, body)
	}
	var patched models.BoardView
	_ = json.Unmarshal([]byte(body), &patched)
	if patched.Name != "Platform" || len(patched.ProjectIDs) != 2 || len(patched.Labels) != 1 {
		t.Errorf("patched = %+v, want only the projects changed", patched)
	}

	status, body = call(t, server, owner, http.MethodGet, "/api/me/board-views", "")
	var views []models.BoardView
	_ = json.Unmarshal([]byte(body), &views)
	if status != http.StatusOK || len(views) != 1 {
		t.Errorf("owner list: status %d, body %s", status, body)
	}

	if status, _ := call(t, server, owner, http.MethodDelete, "/api/me/board-views/"+view.ID, ""); status != http.StatusNoContent {
		t.Errorf("delete: status %d, want 204", status)
	}
	if status, _ := call(t, server, owner, http.MethodGet, "/api/tasks?viewId="+view.ID, ""); status != http.StatusNotFound {
		t.Errorf("tasks of a deleted view: status %d, want 404", status)
	}
	if status, _ := call(t, server, owner, http.MethodPut, "/api/me/board-views", ""); status != http.StatusMethodNotAllowed {
		t.Errorf("PUT collection: status %d, want 405", status)
	}
}
