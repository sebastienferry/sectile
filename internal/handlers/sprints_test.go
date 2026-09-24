package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
)

func TestSprintRoutesRefuseATrackerWithoutSprints(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Repo", Slug: "repo", IssueTracker: "github", GithubRepo: "org/repo"})
	if err != nil {
		t.Fatal(err)
	}
	h := handlers.NewHandler(database)
	server := httptest.NewServer(http.HandlerFunc(h.HandleProjectDetail))
	defer server.Close()
	send := func(method, path, body string) int {
		req, _ := http.NewRequest(method, server.URL+"/api/projects/"+project.ID+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(defaultSession(t, database))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := send(http.MethodPost, "/sprints", `{"name":"Sprint {n}","count":2,"start":"2026-10-05","weeks":2}`); code != http.StatusConflict {
		t.Fatalf("GitHub manages no sprint: expected 409, got %d", code)
	}
	if code := send(http.MethodPost, "/sprints", `{"count":1,"start":"05/10/2026","weeks":2}`); code != http.StatusBadRequest {
		t.Fatalf("a malformed date is a 400, got %d", code)
	}
	if code := send(http.MethodDelete, "/sprints/42", ``); code != http.StatusConflict {
		t.Fatalf("expected 409 on delete, got %d", code)
	}
	if code := send(http.MethodGet, "/sprints", ``); code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET, got %d", code)
	}
}
