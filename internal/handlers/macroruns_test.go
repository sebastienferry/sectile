package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"

	"github.com/gorilla/websocket"
)

func macroRunFixture(t *testing.T) (*db.DB, *handlers.Handler, *httptest.Server, *models.Project) {
	t.Helper()
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: "local"})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if _, err := database.SaveMacroMeta(project.ID, "M-7", nil, nil, nil, nil); err != nil {
		t.Fatalf("macro: %v", err)
	}
	h := handlers.NewHandler(database)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws/agent-connect" {
			h.HandleAgentConnect(w, r)
			return
		}
		h.HandleProjectDetail(w, r)
	}))
	t.Cleanup(server.Close)
	return database, h, server, project
}

func postMacroSkill(t *testing.T, database *db.DB, server *httptest.Server, projectID, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/projects/"+projectID+"/macros/M-7/run-skill", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(defaultSession(t, database))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestMacroRunSkillRefusesATaskSkill(t *testing.T) {
	database, _, server, project := macroRunFixture(t)
	resp := postMacroSkill(t, database, server, project.ID, `{"skillId":"specify"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("a task skill must be refused on a macro, got %d", resp.StatusCode)
	}
}

func TestMacroRunSkillNeedsAnAgent(t *testing.T) {
	database, _, server, project := macroRunFixture(t)
	resp := postMacroSkill(t, database, server, project.ID, `{"skillId":"refine_macro"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFailedDependency {
		t.Fatalf("expected 424 without an agent, got %d", resp.StatusCode)
	}
	if runs, _ := database.MacroRuns(project.ID, "M-7", 10); len(runs) != 0 {
		t.Fatalf("a refused launch must leave no run, got %+v", runs)
	}
}

func TestMacroRunSkillDispatchesWithoutATask(t *testing.T) {
	database, _, server, project := macroRunFixture(t)
	u, _ := url.Parse(server.URL)
	u.Scheme, u.Path = "ws", "/ws/agent-connect"
	u.RawQuery = "token=" + defaultAgentKey(t, database) + "&deviceId=my-laptop&projectId=default"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("agent dial error: %v", err)
	}
	defer conn.Close()

	responses := make(chan *http.Response, 1)
	go func() { responses <- postMacroSkill(t, database, server, project.ID, `{"skillId":"refine_macro"}`) }()

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var agentMsg handlers.AgentMessage
	if err := conn.ReadJSON(&agentMsg); err != nil {
		t.Fatalf("agent did not receive the dispatch: %v", err)
	}
	var dispatch struct {
		RunID     string `json:"runId"`
		TaskID    string `json:"taskId"`
		TaskKey   string `json:"taskKey"`
		ProjectID string `json:"projectId"`
		MacroKey  string `json:"macroKey"`
		SkillID   string `json:"skillId"`
		Mode      string `json:"mode"`
	}
	if err := json.Unmarshal(agentMsg.Payload, &dispatch); err != nil {
		t.Fatal(err)
	}
	if agentMsg.Type != "dispatch_step" || agentMsg.TaskID != "" || dispatch.TaskID != "" || dispatch.TaskKey != "" {
		t.Fatalf("a macro dispatch must carry no task: %+v %s", agentMsg, agentMsg.Payload)
	}
	if dispatch.MacroKey != "M-7" || dispatch.ProjectID != project.ID || dispatch.SkillID != "refine_macro" || dispatch.Mode != models.SkillModeInteractive || dispatch.RunID == "" {
		t.Fatalf("unexpected dispatch %s", agentMsg.Payload)
	}
	payload, _ := json.Marshal(map[string]string{"status": "completed", "summary": "Execution accepted into the local queue"})
	if err := conn.WriteJSON(handlers.AgentMessage{MsgID: agentMsg.MsgID, Type: "step_status", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case resp := <-responses:
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", resp.StatusCode)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("launch acknowledgement not handled")
	}

	active, err := database.ActiveRunOnMacro(project.ID, "m-7")
	if err != nil || active == nil || active.ID != dispatch.RunID || active.MacroKey != "M-7" {
		t.Fatalf("the macro must show its run: %+v %v", active, err)
	}
	if run, _ := database.GetActivityByID(dispatch.RunID); run == nil || run.TaskID != "" || run.ProjectID != project.ID {
		t.Fatalf("the run must be a project activity with no task: %+v", run)
	}

	// A second launch while the first runs is refused.
	second := postMacroSkill(t, database, server, project.ID, `{"skillId":"refine_macro"}`)
	second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 on a busy macro, got %d", second.StatusCode)
	}

	// The runs route takes the project slug as run-skill does.
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/projects/"+project.Slug+"/macros/M-7/runs", nil)
	req.AddCookie(defaultSession(t, database))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var listed struct {
		Runs []models.TaskActivity `json:"runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil || len(listed.Runs) != 1 || listed.Runs[0].ID != dispatch.RunID {
		t.Fatalf("the runs route must list the run: %+v %v", listed, err)
	}
}

func TestMacroCancelRunClosesAnOrphanAndForcesWithoutAnAgent(t *testing.T) {
	database, _, server, project := macroRunFixture(t)
	run, err := database.StartMacroRun(project.ID, "M-7", "refine_macro", db.RunLaunch{UserID: db.ImplicitUserID})
	if err != nil {
		t.Fatal(err)
	}
	post := func(body string) int {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/projects/"+project.ID+"/macros/M-7/cancel-run", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(defaultSession(t, database))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	// No agent: the stop cannot be witnessed, so it needs force.
	if code := post(`{"runId":"` + run.ID + `"}`); code != http.StatusBadGateway {
		t.Fatalf("without an agent a stop is a 502, got %d", code)
	}
	if code := post(`{"runId":"` + run.ID + `","force":true}`); code != http.StatusOK {
		t.Fatalf("a forced stop closes the run, got %d", code)
	}
	if active, _ := database.ActiveRunOnMacro(project.ID, "M-7"); active != nil {
		t.Fatalf("the macro must be free again, got %+v", active)
	}
	if code := post(`{"runId":"` + run.ID + `","force":true}`); code != http.StatusConflict {
		t.Fatalf("a closed run is not active any more, got %d", code)
	}
}
