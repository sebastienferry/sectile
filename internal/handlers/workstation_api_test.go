package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

func agentRequest(h *Handler, handler http.HandlerFunc, method, target, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	h.AgentAPIAuth(handler).ServeHTTP(rec, req)
	return rec
}

// The seed serves the former values read-only, the deployment's with the
// caller's own terminal and editor, and a project's former composition.
func TestAgentExecutionSeed(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	if rec := agentRequest(h, h.HandleAgentExecutionSeed, http.MethodGet, "/api/v1/agent/execution-seed", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credential: %d", rec.Code)
	}
	key := agentTestKey(t, h)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Seeded"})
	if err != nil {
		t.Fatal(err)
	}
	rec := agentRequest(h, h.HandleAgentExecutionSeed, http.MethodGet, "/api/v1/agent/execution-seed?projectId="+project.ID, key, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("seed: %d %s", rec.Code, rec.Body.String())
	}
	var seed agentconfig.Seed
	if err := json.Unmarshal(rec.Body.Bytes(), &seed); err != nil {
		t.Fatal(err)
	}
	if seed.SchemaVersion != agentconfig.Version || seed.Defaults == nil || seed.Project == nil || seed.Project.ProjectID != project.ID {
		t.Fatalf("seed: %s", rec.Body.String())
	}
	// The column default editor is the provider default and is not sent.
	if seed.Defaults.EditorCommand != "" || seed.Defaults.AIProvider != "agy" {
		t.Fatalf("defaults: %+v", seed.Defaults)
	}
	if rec := agentRequest(h, h.HandleAgentExecutionSeed, http.MethodGet, "/api/v1/agent/execution-seed?projectId=missing", key, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown project: %d", rec.Code)
	}
}

// A report is stored under the credential's user, whatever the body says, and
// the web reads only its own, and only while that workstation is connected.
func TestAgentCapabilitiesAndProjectEngine(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Engine"})
	if err != nil {
		t.Fatal(err)
	}
	engine := func() models.EngineReport {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/engine", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("engine: %d %s", rec.Code, rec.Body.String())
		}
		var report models.EngineReport
		_ = json.Unmarshal(rec.Body.Bytes(), &report)
		return report
	}
	if report := engine(); report.State != models.EngineUnknown {
		t.Fatalf("no agent, no report: %+v", report)
	}

	body := `{"schemaVersion":1,"deviceId":"laptop","projects":[{"projectId":"` + project.ID + `","provider":"claude","model":"opus","models":["opus","sonnet"],"modelSlot":true,"headless":true}]}`
	if rec := agentRequest(h, h.HandleAgentCapabilities, http.MethodPut, "/api/v1/agent/capabilities", "", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credential: %d", rec.Code)
	}
	if rec := agentRequest(h, h.HandleAgentCapabilities, http.MethodPut, "/api/v1/agent/capabilities", key, `{"schemaVersion":9}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown version: %d", rec.Code)
	}
	if rec := agentRequest(h, h.HandleAgentCapabilities, http.MethodPut, "/api/v1/agent/capabilities", key, body); rec.Code != http.StatusNoContent {
		t.Fatalf("report: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := database.EngineReport("someone-else", project.ID, ""); ok {
		t.Fatal("the report was stored under another user")
	}
	// Reported, but the workstation is not connected: what would run is unknown.
	if report := engine(); report.State != models.EngineUnknown {
		t.Fatalf("a report without a connected workstation: %+v", report)
	}

	server := httptest.NewServer(http.HandlerFunc(h.HandleAgentConnect))
	defer server.Close()
	agent, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?projectId="+project.ID+"&deviceId=laptop", http.Header{"Authorization": []string{"Bearer " + key}})
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	if err = agent.WriteJSON(AgentMessage{Type: "heartbeat"}); err != nil {
		t.Fatal(err)
	}
	var heartbeat AgentMessage
	if err = agent.ReadJSON(&heartbeat); err != nil {
		t.Fatal(err)
	}
	report := engine()
	if report.State != models.EngineReported || report.Provider != "claude" || report.Model != "opus" || len(report.Models) != 2 || !report.ModelSlot || !report.Headless {
		t.Fatalf("the connected workstation's report: %+v", report)
	}
}
