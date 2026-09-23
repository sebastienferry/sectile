package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
)

func TestProjectUpdateProviderClearAndPreserveOverHTTP(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Provider API", AIProvider: "claude"})
	if err != nil {
		t.Fatal(err)
	}

	update := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPatch, "/api/projects/"+project.ID, strings.NewReader(body))
		h.HandleProjectDetail(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("update failed: %d %s", recorder.Code, recorder.Body.String())
		}
		return recorder
	}

	update(`{"name":"Provider API renamed"}`)
	kept, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kept.AIProvider != "claude" {
		t.Fatalf("an omitted provider was not preserved: %q", kept.AIProvider)
	}

	update(`{"aiProvider":""}`)
	reread, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reread.AIProvider != "" {
		t.Fatalf("an explicit empty provider did not clear the stored override: %q", reread.AIProvider)
	}
}
