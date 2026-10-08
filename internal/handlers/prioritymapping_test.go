package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// schemeTracker lists a numbered priority scheme, every line of which the
// mapping guesses (#679).
type schemeTracker struct {
	tracker.BaseTicketingSystem
	fresh []bool
}

func (s *schemeTracker) PriorityScheme(ctx context.Context, trk *models.Tracker, fresh bool) ([]models.PriorityOption, error) {
	s.fresh = append(s.fresh, fresh)
	return []models.PriorityOption{{ID: "1", Name: "P1"}, {ID: "2", Name: "P2"}, {ID: "3", Name: "P3"}, {ID: "4", Name: "P4"}}, nil
}

func (s *schemeTracker) ClassifyPriority(name string, rank, n int) (models.Priority, bool) {
	return models.PriorityLevels[rank], false
}

// TestPriorityMappingOverHTTP covers what the Tracker tab relies on (#679):
// the refresh fills the mapping from a fresh read, a person's edit makes a
// line sure, an unknown option answers 400, and a priority the mapping only
// guessed answers 422 with the sentence the person reads.
func TestPriorityMappingOverHTTP(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	fake := &schemeTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "jira"}}
	database.TrackerRegistry().Register("jira", fake)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Ops", IssueTracker: "jira", JiraProject: "OPS"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Guarded", Priority: models.PriorityMedium, Source: "local"})
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(database)
	send := func(method, path, body string, handle func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		rec := httptest.NewRecorder()
		handle(rec, req)
		return rec
	}

	rec := send(http.MethodPost, "/api/projects/"+project.ID+"/priority-mapping/refresh", "", h.HandleProjectDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh returned %d: %s", rec.Code, rec.Body.String())
	}
	var refreshed models.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &refreshed); err != nil {
		t.Fatal(err)
	}
	if len(refreshed.PriorityMapping.Options) != 4 || !refreshed.PriorityMapping.Options[1].Guessed {
		t.Fatalf("refreshed mapping = %+v", refreshed.PriorityMapping)
	}
	if len(fake.fresh) != 1 || !fake.fresh[0] {
		t.Fatalf("the refresh must read the scheme fresh: %v", fake.fresh)
	}

	rec = send(http.MethodPut, "/api/tasks/"+task.ID, `{"priority":"high"}`, h.HandleTaskDetail)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a guessed priority returned %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Tracker tab") {
		t.Fatalf("the refusal reads %s", rec.Body.String())
	}

	rec = send(http.MethodPatch, "/api/projects/"+project.ID, `{"priorityMapping":{"options":[{"id":"9","level":"high"}]}}`, h.HandleProjectDetail)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an unknown option returned %d, want 400: %s", rec.Code, rec.Body.String())
	}

	rec = send(http.MethodPatch, "/api/projects/"+project.ID, `{"priorityMapping":{"options":[{"id":"2","level":"high"}]}}`, h.HandleProjectDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirming a line returned %d: %s", rec.Code, rec.Body.String())
	}
	rec = send(http.MethodPut, "/api/tasks/"+task.ID, `{"priority":"high"}`, h.HandleTaskDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("a confirmed priority returned %d: %s", rec.Code, rec.Body.String())
	}
}
