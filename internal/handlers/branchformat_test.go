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

// TestProjectBranchNameFormatOverHTTP covers what the settings modal relies on:
// a refused format answers 400 with the reason the user reads, in French.
func TestProjectBranchNameFormatOverHTTP(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Branches"})
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

	rec := patch(`{"branchNameFormat":"{key}"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("a valid format returned %d: %s", rec.Code, rec.Body.String())
	}
	var saved models.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.BranchNameFormat != "{key}" {
		t.Fatalf("the saved project reads format %q, want {key}", saved.BranchNameFormat)
	}

	rec = patch(`{"branchNameFormat":"feat/{title}"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a format without a key returned %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var refusal map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &refusal); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(refusal["error"], "doit contenir {key} ou {key_lower}") {
		t.Fatalf("the refusal reads %q, want the French reason", refusal["error"])
	}
}
