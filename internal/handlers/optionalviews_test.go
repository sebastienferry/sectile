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
)

// TestProjectEnabledViewsOverHTTP covers the contract the settings screen talks
// to. The clearing case is the one worth pinning: an empty list has to reach the
// database as "show none", not as "the caller said nothing", or a view could be
// switched on and never switched off again.
func TestProjectEnabledViewsOverHTTP(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Views"})
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(database)
	patch := func(body string) models.Project {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/projects/"+project.ID, strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %s returned %d: %s", body, rec.Code, rec.Body.String())
		}
		var out models.Project
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decoding the response to %s: %v", body, err)
		}
		return out
	}

	if got := patch(`{"enabledViews":["timeline","triage"]}`); len(got.EnabledViews) != 2 ||
		got.EnabledViews[0] != "triage" || got.EnabledViews[1] != "timeline" {
		t.Fatalf("after enabling, the project shows %q, want [triage timeline]", got.EnabledViews)
	}

	// A payload that does not mention the setting must leave it alone: the
	// settings screen sends the whole project on every other change too.
	if got := patch(`{"description":"unrelated"}`); len(got.EnabledViews) != 2 {
		t.Fatalf("an unrelated change reset the views to %q", got.EnabledViews)
	}

	if got := patch(`{"enabledViews":[]}`); len(got.EnabledViews) != 0 {
		t.Fatalf("after clearing, the project still shows %q", got.EnabledViews)
	}

	// An unknown view must never reach the sidebar, whatever a client sends.
	if got := patch(`{"enabledViews":["gantt","roadmap"]}`); len(got.EnabledViews) != 1 || got.EnabledViews[0] != "roadmap" {
		t.Fatalf("an unknown view survived: %q", got.EnabledViews)
	}
}
