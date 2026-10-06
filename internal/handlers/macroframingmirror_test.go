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

func TestBulkFramingRepublishListsAndQueuesThePendingEpics(t *testing.T) {
	database, project, h := framingHandler(t)
	cookie := defaultSession(t, database)
	for key, text := range map[string]string{"PE-1": "One", "PE-2": "Two", "M-3": "Local", "DS-4": "Foreign", "PE-5": ""} {
		if _, err := database.SaveMacroMeta(project.ID, key, nil, nil, &text, nil); err != nil {
			t.Fatal(err)
		}
	}
	call := func(method, segment string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/projects/"+project.ID+"/"+segment+"/framing-mirror", nil)
		req.AddCookie(cookie)
		h.HandleProjectDetail(rr, req)
		return rr
	}

	for _, segment := range []string{"macros", "epics"} {
		rr := call(http.MethodGet, segment)
		var listed []models.MacroMeta
		if err := json.Unmarshal(rr.Body.Bytes(), &listed); rr.Code != http.StatusOK || err != nil || len(listed) != 2 || listed[0].Key != "PE-1" || listed[1].Key != "PE-2" {
			t.Fatalf("GET %s: %d %s", segment, rr.Code, rr.Body.String())
		}
	}

	rr := call(http.MethodPost, "macros")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("POST: %d %s, want 202", rr.Code, rr.Body.String())
	}
	var out struct {
		Queued   int                  `json:"queued"`
		Skipped  int                  `json:"skipped"`
		Activity *models.TaskActivity `json:"activity"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.Queued != 2 || out.Skipped != 1 || out.Activity == nil {
		t.Fatalf("answer %s %v", rr.Body.String(), err)
	}
	if out.Activity.Action != "Cadrages de roadmap ➔ tracker" || out.Activity.UserID != db.ImplicitUserID {
		t.Fatalf("the activity is signed by the person asking: %+v", out.Activity)
	}

	// Once written, nothing is pending: POST queues nothing and says so.
	deadline := time.Now().Add(10 * time.Second)
	for {
		pending, _, err := database.PendingFramingCopies(project.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the batch left %d epics pending", len(pending))
		}
		time.Sleep(50 * time.Millisecond)
	}
	rr = call(http.MethodPost, "epics")
	if err := json.Unmarshal(rr.Body.Bytes(), &out); rr.Code != http.StatusOK || err != nil || out.Queued != 0 || out.Skipped != 1 || out.Activity != nil {
		t.Fatalf("POST with nothing pending: %d %s", rr.Code, rr.Body.String())
	}

	if rr := call(http.MethodDelete, "macros"); rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE: %d, want 405", rr.Code)
	}
	// The republish of one macro keeps its own route.
	single := httptest.NewRecorder()
	h.HandleProjectDetail(single, httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/macros/PE-1/framing-mirror", nil))
	if single.Code != http.StatusAccepted || !strings.Contains(single.Body.String(), "Cadrage de PE-1") {
		t.Fatalf("single republish: %d %s", single.Code, single.Body.String())
	}
}
