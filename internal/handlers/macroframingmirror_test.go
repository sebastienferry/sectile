package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// framingHandler is a handler on a Jira project whose tracker keeps marked
// comments, the only one a framing is copied on (#636).
func framingHandler(t *testing.T) (*db.DB, *models.Project, *handlers.Handler) {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	database.TrackerRegistry().Register("jira", &commentTracker{tracker.BaseTicketingSystem{
		TrackerName:  "jira",
		Capabilities: []tracker.Capability{tracker.CapComment},
	}})
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	return database, project, handlers.NewHandler(database)
}

func framingActivities(t *testing.T, database *db.DB, projectID, key string) int {
	t.Helper()
	activities, err := database.GetActivities(projectID, "", "", "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, act := range activities {
		if act.Action == "Cadrage de "+key+" ➔ tracker" {
			n++
		}
	}
	return n
}

func TestFramingMirrorRepublishIsQueuedOrRefusedWithItsReason(t *testing.T) {
	database, project, h := framingHandler(t)

	post := func(key string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.HandleProjectDetail(rr, httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/macros/"+key+"/framing-mirror", nil))
		return rr
	}
	rr := post("PE-1")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d %s, want 202", rr.Code, rr.Body.String())
	}
	var out struct {
		Activity models.TaskActivity `json:"activity"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.Activity.Action != "Cadrage de PE-1 ➔ tracker" {
		t.Fatalf("activity %+v %v", out.Activity, err)
	}

	rr = post("M-4")
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "reste dans Sectile") {
		t.Fatalf("got %d %s, want 400 with the reason", rr.Code, rr.Body.String())
	}
	if n := framingActivities(t, database, project.ID, "M-4"); n != 0 {
		t.Fatalf("a refusal queues nothing, got %d", n)
	}
}

func TestMacroSaveCarriesTheFramingStatusAndABulkEditQueuesNoCopy(t *testing.T) {
	database, project, h := framingHandler(t)

	put := func(key, body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.HandleProjectDetail(rr, httptest.NewRequest(http.MethodPut, "/api/projects/"+project.ID+"/macros/"+key, strings.NewReader(body)))
		if rr.Code != http.StatusOK {
			t.Fatalf("PUT %s: %d %s", key, rr.Code, rr.Body.String())
		}
		return rr
	}
	rr := put("PE-1", `{"framingComment":"Why","bulk":true}`)
	put("PE-2", `{"framingComment":"Why"}`)

	var out struct {
		Macro models.MacroMeta `json:"macro"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.Macro.FramingMirror == nil {
		t.Fatalf("the answer carries the framing status: %+v %v", out.Macro, err)
	}
	if m := out.Macro.FramingMirror; m.Kind != models.MacroTodosMirrorJiraComment || m.UpToDate {
		t.Fatalf("a framing never copied is waiting: %+v", m)
	}

	// The copy waits a few seconds for the saves to settle.
	deadline := time.Now().Add(10 * time.Second)
	for framingActivities(t, database, project.ID, "PE-2") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the framing save of PE-2 queued no copy")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if n := framingActivities(t, database, project.ID, "PE-1"); n != 0 {
		t.Fatalf("a bulk edit queues no framing copy, got %d", n)
	}
}
