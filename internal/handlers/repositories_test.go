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

// The repository routes of #456: a refused declaration is the caller's fault,
// and only the first conversion of a project applies.
func TestProjectRepositoriesOverHTTP(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Multi", GitRemoteUrl: "git@github.com:o/a.git"})
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

// The mono-repo setting is gone (#484), but a client written before its
// removal still sends it: creating and saving a project with it succeeds, the
// key is ignored, and no project answers with it any more. The route a local
// agent used to park a launch on a repository choice no longer exists.
func TestProjectIgnoresTheRemovedMonoRepoKey(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := NewHandler(database)

	rec := httptest.NewRecorder()
	h.HandleProjects(rec, httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"Legacy client","monoRepo":false}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create with monoRepo: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "monoRepo") {
		t.Errorf("the created project still reports monoRepo: %s", rec.Body.String())
	}
	var created models.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	h.HandleProjectDetail(rec, httptest.NewRequest(http.MethodPatch, "/api/projects/"+created.ID, strings.NewReader(`{"description":"saved","monoRepo":true}`)))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "monoRepo") || !strings.Contains(rec.Body.String(), `"description":"saved"`) {
		t.Errorf("update with monoRepo: %d %s", rec.Code, rec.Body.String())
	}

	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: created.ID, Title: "parked"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartRemoteRun(task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.HandleActivityDetail(rec, httptest.NewRequest(http.MethodPost, "/api/activities/"+run.ID+"/awaiting-repository", strings.NewReader(`{"waiting":true}`)))
	if rec.Code == http.StatusOK {
		t.Errorf("the repository wait route still answers: %d %s", rec.Code, rec.Body.String())
	}
}
