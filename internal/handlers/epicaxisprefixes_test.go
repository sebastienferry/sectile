package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

// TestProjectEpicAxisPrefixesOverHTTP covers what the Roadmap section of the
// settings relies on (#635): a prefix is stored cleaned, and a refused one
// answers 400 with the sentence the user reads, storing nothing.
func TestProjectEpicAxisPrefixesOverHTTP(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Roadmap", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(database)
	patch := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/projects/"+project.ID, strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, req)
		return rec
	}

	rec := patch(`{"epicAxisPrefixes":{"priority":" #Prio- "}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("a valid prefix returned %d: %s", rec.Code, rec.Body.String())
	}
	var saved models.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.EpicAxisPrefixes.Priority != "prio-" {
		t.Fatalf("the saved project reads %+v, want prio-", saved.EpicAxisPrefixes)
	}

	rec = patch(`{"epicAxisPrefixes":{"priority":"p","quarter":"priority:"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("overlapping prefixes returned %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var refusal map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &refusal); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(refusal["error"], "se recouvrent") {
		t.Fatalf("the refusal reads %q, want the French reason", refusal["error"])
	}
	stored, _ := database.GetProjectByID(project.ID)
	if stored.EpicAxisPrefixes.Priority != "prio-" || stored.EpicAxisPrefixes.Quarter != "" {
		t.Fatalf("a refused save changed the prefixes: %+v", stored.EpicAxisPrefixes)
	}
}
