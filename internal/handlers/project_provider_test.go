package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
)

// An older interface still posts execution settings with a project save: they
// are ignored, and the rest of the save goes through (#305).
func TestProjectUpdateIgnoresExecutionSettingsOverHTTP(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Provider API"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	body := `{"name":"Renamed","aiProvider":"claude","aiModel":"opus","repoPath":"/srv","useWorktrees":false,"setupProviders":["codex"],"skillOverrides":{"implement":"x"},"ttyMode":"external"}`
	h.HandleProjectDetail(recorder, httptest.NewRequest(http.MethodPatch, "/api/projects/"+project.ID, strings.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", recorder.Code, recorder.Body.String())
	}
	for _, key := range []string{"aiProvider", "aiModel", "repoPath", "useWorktrees", "setupProviders", "skillOverrides", "ttyMode"} {
		if strings.Contains(recorder.Body.String(), `"`+key+`"`) {
			t.Fatalf("the answer still carries %s: %s", key, recorder.Body.String())
		}
	}
	stored, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Renamed" || stored.AIProvider != "" || stored.AIModel != "" || stored.RepoPath != "" || !stored.UseWorktrees || len(stored.SetupProviders) != 0 || len(stored.SkillOverrides) != 0 {
		t.Fatalf("an execution setting was written, or the rest was lost: %+v", stored)
	}
}
