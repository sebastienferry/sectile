package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

// The repository routes of #456: a refused declaration is the caller's fault,
// and only the first conversion of a project applies.
func TestProjectRepositoriesOverHTTP(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	no := false
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Multi", MonoRepo: &no, GitRemoteUrl: "git@github.com:o/a.git"})
	if err != nil {
		t.Fatal(err)
	}
	// A ticket pinned to a legacy path gives the project one to convert.
	pinned, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "pinned"})
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := "/src/b"
	if _, err := database.UpdateTask(pinned.ID, models.UpdateTaskRequest{RepoPath: &legacyPath}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(database)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}

	if rec := do(http.MethodPatch, "/api/projects/"+project.ID, `{"repositories":["https://github.com/o/b","git@github.com:o/b.git"]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("duplicate repository: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodGet, "/api/projects/"+project.ID+"/legacy-repo-paths", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"/src/b"`) {
		t.Errorf("legacy paths: %d %s", rec.Code, rec.Body.String())
	}
	report := `{"converted":[{"path":"/src/b","url":"git@github.com:o/b.git"}],"dropped":[]}`
	if rec := do(http.MethodPost, "/api/projects/"+project.ID+"/repositories/convert", report); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "github.com/o/b") {
		t.Errorf("conversion: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPost, "/api/projects/"+project.ID+"/repositories/convert", report); rec.Code != http.StatusConflict {
		t.Errorf("second conversion: %d %s", rec.Code, rec.Body.String())
	}
}

// The local agent marks a parked launch with its own credential, as it
// reports the engine: the route answers it, and says when the run is gone.
func TestRunAwaitingRepositoryOverHTTP(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Multi"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "parked"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartRemoteRun(task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(database)
	post := func(id, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.HandleActivityDetail(rec, httptest.NewRequest(http.MethodPost, "/api/activities/"+id+"/awaiting-repository", strings.NewReader(body)))
		return rec
	}
	if rec := post(run.ID, `{"waiting":true}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"waitingReason":"repository"`) {
		t.Errorf("mark: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(run.ID, `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("no waiting field: %d", rec.Code)
	}
	if rec := post("missing", `{"waiting":true}`); rec.Code != http.StatusNotFound {
		t.Errorf("unknown run: %d", rec.Code)
	}
}
