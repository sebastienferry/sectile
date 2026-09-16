package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"tasks/internal/models"
	"testing"
)

func TestDesktopSkillResultMatchesOwnedExecution(t *testing.T) {
	projectID := "project"
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "unavailable", 503)
			return
		}
		switch r.URL.Path {
		case "/api/tasks/task":
			_ = json.NewEncoder(w).Encode(models.Task{ID: "task", ProjectID: projectID, Status: "to_specify"})
		case "/api/tasks/task/activities":
			_ = json.NewEncoder(w).Encode([]models.TaskActivity{
				{ID: "old", TaskID: "task", SkillID: "clarify", Status: "completed"},
				{ID: "run", TaskID: "task", SkillID: "clarify", Status: "running"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	d := &agentDaemon{serverURL: server.URL, desktopToken: "private", queue: runQueue{runs: map[string]*controlledRun{
		"run": {taskID: "task", desktop: desktopRun{ProjectID: "project", Status: "running"}},
	}}}
	request := httptest.NewRequest("GET", "/desktop/run-result?id=run", nil)
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != 401 {
		t.Fatal("unauthenticated result lookup accepted")
	}
	response = disconnectRequest(d, "GET", "/desktop/run-result?id=run", "")
	var result struct {
		Activity models.TaskActivity `json:"activity"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || result.Activity.ID != "run" || result.Activity.Status != "running" {
		t.Fatalf("wrong result: %s", response.Body.String())
	}
	if response = disconnectRequest(d, "GET", "/desktop/run-result?id=unknown", ""); response.Code != 404 {
		t.Fatal("unknown run accepted")
	}
	projectID = "other"
	if response = disconnectRequest(d, "GET", "/desktop/run-result?id=run", ""); response.Code != 409 {
		t.Fatal("mismatched project accepted")
	}
	fail = true
	if response = disconnectRequest(d, "GET", "/desktop/run-result?id=run", ""); response.Code != 502 {
		t.Fatal("unavailable result accepted")
	}
}
