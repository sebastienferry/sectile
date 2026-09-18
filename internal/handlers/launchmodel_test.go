package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

func launchModelTask(t *testing.T) (*Handler, *db.DB, *models.Task, func()) {
	t.Helper()
	h, database, cleanup := setupTestHandler(t)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Launch", AIProvider: "claude", AIModel: "claude-sonnet-5"})
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Pick a model", ProjectID: project.ID})
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	return h, database, task, cleanup
}

func postRunSkill(h *Handler, taskID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+taskID+"/run-skill", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleTaskDetail(rec, req)
	return rec
}

// A malformed model is refused before anything is recorded or dispatched: the
// value would otherwise reach a command line run through sh -c.
func TestRunSkillRefusesAMalformedModel(t *testing.T) {
	h, database, task, cleanup := launchModelTask(t)
	defer cleanup()

	rec := postRunSkill(h, task.ID, `{"skillId":"clarify","model":"claude-opus-5; rm -rf ~"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a malformed model, got %d: %s", rec.Code, rec.Body.String())
	}
	activities, err := database.GetTaskActivities(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 0 {
		t.Fatalf("a refused launch must record nothing, got %d activities", len(activities))
	}
}

// Validation is on shape, not on membership: a list edited a minute ago must not
// break a launch already in flight.
func TestRunSkillAcceptsAModelAbsentFromTheConfiguredList(t *testing.T) {
	h, _, task, cleanup := launchModelTask(t)
	defer cleanup()

	rec := postRunSkill(h, task.ID, `{"skillId":"clarify","model":"model-released-yesterday"}`)
	// No agent is connected in this test, so the launch is refused for that
	// reason and not for the model: what matters is that it got that far.
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("a well-formed model was rejected: %s", rec.Body.String())
	}
}

// The advance route carries the model into the queued job, which is the path a
// card launch takes when no agent is connected.
func TestAdvanceRefusesAMalformedModel(t *testing.T) {
	h, _, task, cleanup := launchModelTask(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/advance", strings.NewReader(`{"model":"bad model"}`))
	rec := httptest.NewRecorder()
	h.HandleTaskDetail(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a malformed model, got %d: %s", rec.Code, rec.Body.String())
	}
}

// The engine route is how the agent corrects the record once it has built the
// real command line.
func TestEngineReportUpdatesTheRun(t *testing.T) {
	h, database, task, cleanup := launchModelTask(t)
	defer cleanup()

	run, err := database.StartAgentRun(task.ID, "implement", db.RunLaunch{Provider: "claude", Model: "claude-sonnet-5"})
	if err != nil {
		t.Fatal(err)
	}

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/activities/"+run.ID+"/engine", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.HandleActivityDetail(rec, req)
		return rec
	}

	if rec := post(`{"provider":"claude","model":"workstation-model"}`); rec.Code != http.StatusOK {
		t.Fatalf("engine report refused: %d %s", rec.Code, rec.Body.String())
	}
	stored, err := database.GetActivityByID(run.ID)
	if err != nil || stored == nil {
		t.Fatalf("run not found: %v", err)
	}
	if stored.Model != "workstation-model" {
		t.Fatalf("engine report ignored: %q", stored.Model)
	}

	if rec := post(`{"provider":"claude","model":"bad model"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("a malformed reported model must be refused, got %d", rec.Code)
	}
}

// The per-provider list is refused as a whole when one identifier is unusable,
// and the error says which provider carries it.
func TestSettingsRefuseAnInvalidProviderModel(t *testing.T) {
	h, _, _, cleanup := launchModelTask(t)
	defer cleanup()

	body := `{"aiProviderModels":{"gemini":["gemini-2.5-pro; rm -rf ~"]}}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleSettings(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if !strings.Contains(strings.Join([]string{payload["error"], rec.Body.String()}, " "), "gemini") {
		t.Fatalf("the error must name the provider: %s", rec.Body.String())
	}
}
