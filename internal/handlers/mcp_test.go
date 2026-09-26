package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/agentconfig"
	"tasks/internal/db"
	"tasks/internal/mcptest"
	"tasks/internal/models"
)

type testTokenTransport struct{ token string }

func (tr testTokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+tr.token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestMCPToolsEndToEnd(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "integration-secret")
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "MCP workflow", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	// The tools that write run as a person: a key tied to a user, as the
	// desktop app's is. The shared server key only reads (#482).
	userID, err := database.UpsertUser("okta|mcp", "mcp@example.com", "MCP")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := database.CreateAPIKey(userID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	ctx := context.Background()
	connect := func() *mcp.ClientSession {
		t.Helper()
		session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: testTokenTransport{key}}}, nil)
		if err != nil {
			t.Fatal(err)
		}

		return session
	}
	session := connect()
	defer session.Close()
	mcptest.AssertNaming(t, ctx, session, database, task, connect)
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 13 {
		t.Fatalf("tools = %+v, %v", list, err)
	}
	call := func(name string, args any, wantError bool) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			if wantError {
				return nil
			}
			t.Fatal(err)
		}
		if result.IsError != wantError {
			t.Fatalf("%s: %+v", name, result)
		}
		return result
	}
	started := call("start_run", map[string]any{"taskKey": task.ID, "skill": "pickup"}, false)
	startedJSON, _ := json.Marshal(started.StructuredContent)
	var run models.TaskActivity
	if err := json.Unmarshal(startedJSON, &run); err != nil || run.ID == "" {
		t.Fatalf("invalid run: %s %v", startedJSON, err)
	}
	call("start_run", map[string]any{"taskKey": task.ID, "skill": "pickup", "runId": run.ID}, false)
	call("transition_stage", map[string]any{"taskKey": task.ID, "stage": "clarified", "note": "Intermediate step"}, false)
	active, err := database.GetActivityByID(run.ID)
	if err != nil || active.Status != "running" {
		t.Fatalf("transition finished remote run: %+v %v", active, err)
	}
	call("finish_run", map[string]any{"taskKey": task.ID, "runId": run.ID, "status": "completed", "note": "Done"}, false)
	active, err = database.GetActivityByID(run.ID)
	if err != nil || active.Status != "completed" {
		t.Fatalf("run not finished: %+v %v", active, err)
	}
	call("finish_run", map[string]any{"taskKey": task.ID, "runId": run.ID, "status": "invented", "note": "Bad"}, true)
	call("add_comment", map[string]any{"taskKey": task.Key, "body": "Quotes: \"yes\"\nsecond line $(literal)"}, false)
	result := call("get_task", map[string]any{"taskKey": task.Key}, false)
	raw, _ := json.Marshal(result)
	if !strings.Contains(string(raw), "second line $(literal)") {
		t.Fatalf("missing comments: %s", raw)
	}
	projects := call("list_projects", map[string]any{}, false)
	projectJSON, _ := json.Marshal(projects)
	if !strings.Contains(string(projectJSON), "default") || strings.Contains(string(projectJSON), "repoPath") {
		t.Fatalf("invalid project discovery: %s", projectJSON)
	}
	call("get_project_context", map[string]any{"projectId": "default"}, false)
	call("list_tasks", map[string]any{"projectId": "default"}, false)
	call("get_project_context", map[string]any{"taskKey": task.Key}, false)
	call("transition_stage", map[string]any{"taskKey": task.Key, "stage": "clarified", "note": "Scope checked", "branch": "feat/mcp"}, false)
	updated, err := database.GetTaskByID(task.ID)
	if err != nil || updated.BranchName == nil || *updated.BranchName != "feat/mcp" {
		t.Fatalf("transition failed: %+v %v", updated, err)
	}
	for _, link := range []string{"https://github.com/example/repo/pull/42", "https://gitlab.com/example/repo/-/merge_requests/42"} {
		call("transition_stage", map[string]any{"taskKey": task.ID, "stage": "reviewed", "note": "Unverified URL", "prUrl": link}, true)
		persisted, err := database.GetTaskByID(task.ID)
		if err != nil || persisted.PrURL != nil {
			t.Fatalf("unverified PR was persisted: %+v %v", persisted, err)
		}
	}

	for _, args := range []map[string]any{
		{"taskKey": task.Key, "stage": "invented", "note": "bad"},
		{"taskKey": task.Key, "stage": "implemented"},
		{"taskKey": task.Key, "stage": "implemented", "note": "   "},
	} {
		call("transition_stage", args, true)
	}
	call("get_task", map[string]any{"taskKey": "missing"}, true)
	call("add_comment", map[string]any{"taskKey": task.Key, "body": ""}, true)
	call("get_project_context", map[string]any{}, true)
}

func TestAgentConfigAuthAndProjection(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "expected")
	handler := h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentConfig))
	for _, token := range []string{"", "wrong", "expected"} {
		req := httptest.NewRequest("GET", "/api/v1/agent/config?projectId=default", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if token != "expected" {
			if rr.Code != 401 {
				t.Fatalf("bad token accepted: %d", rr.Code)
			}
			continue
		}
		if rr.Code != 200 {
			t.Fatalf("config: %d %s", rr.Code, rr.Body.String())
		}
		var c agentconfig.Config
		if err := json.Unmarshal(rr.Body.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		if c.SchemaVersion != 1 || len(c.Skills) == 0 {
			t.Fatalf("incomplete config %+v", c)
		}
		// The layout travels, so the agent knows whether the code checkout
		// carries the specifications.
		if c.MonoRepo == nil {
			t.Fatalf("the repository layout is missing from %s", rr.Body.String())
		}
		for _, field := range []string{`"repoPath"`, `"jiraApiToken"`, `"userEmail"`} {
			if strings.Contains(rr.Body.String(), field) {
				t.Fatalf("server-only field %s leaked", field)
			}
		}
	}
	req := httptest.NewRequest("GET", "/api/v1/agent/config?projectId=default", nil)
	req.Header.Set("Authorization", "Bearer expected")
	req.Header.Set("Origin", "https://untrusted.example")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Fatalf("browser origin accepted: %d", rr.Code)
	}
}

func TestAgentProjectDiscovery(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "discovery-secret")
	handler := h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentProjects))
	for _, token := range []string{"", "discovery-secret"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/projects", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if token == "" {
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("unauthenticated status: %d", rec.Code)
			}
			continue
		}
		var result agentconfig.Projects
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || result.SchemaVersion != 1 || len(result.Projects) == 0 {
			t.Fatalf("discovery: %s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "repoPath") {
			t.Fatal("server paths exposed")
		}
	}
}

func TestRemoteRunIsolation(t *testing.T) {
	_, database, cleanup := setupTestHandler(t)
	defer cleanup()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Remote work", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	other, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Other work", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	first, err := database.StartRemoteRun(task.ID, "clarify", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := database.StartRemoteRun(task.ID, "review", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.FinishRemoteRun(other.ID, first.ID, "completed", "Wrong task"); err == nil {
		t.Fatal("cross-task completion allowed")
	}
	if _, err := database.FinishRemoteRun(task.ID, first.ID, "failed", "Client failure"); err != nil {
		t.Fatal(err)
	}
	remaining, err := database.GetActivityByID(second.ID)
	if err != nil || remaining.Status != "running" {
		t.Fatalf("concurrent run affected: %+v %v", remaining, err)
	}
	if _, err := database.FinishRemoteRun(task.ID, second.ID, "canceled", "Stopped"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.StartRemoteRun(task.ID, "review", second.ID); err == nil {
		t.Fatal("finished run reused")
	}
}

func TestWebSkillRequiresLocalAgent(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Local launch"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/run-skill", strings.NewReader(`{"skillId":"clarify"}`))
	response := httptest.NewRecorder()
	h.HandleTaskDetail(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
	activities, err := database.GetTaskActivities(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, activity := range activities {
		if activity.SkillID == "clarify" {
			t.Fatal("server queued local execution")
		}
	}
}

// The configuration names no provider or command since #305, whatever the
// deployment row held.
func TestAgentConfigServesNoExecutionSetting(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "expected")
	if _, err := database.UpdateSettings(models.Settings{AIProvider: "agy", AICommandTemplate: "agy"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/v1/agent/config?projectId=default", nil)
	req.Header.Set("Authorization", "Bearer expected")
	rr := httptest.NewRecorder()
	h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentConfig)).ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("legacy template rejected: %d %s", rr.Code, rr.Body.String())
	}
	var c agentconfig.Config
	if err := json.Unmarshal(rr.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if c.AIProvider != "" || c.AICommandTemplate != "" {
		t.Fatalf("provider=%q template=%q", c.AIProvider, c.AICommandTemplate)
	}
}

// The shared server key names no person: over MCP it still reads and reports
// runs, and every write tool is refused before anything is written, rather
// than signed by the server account (#482).
func TestMCPSharedKeyReadsButNeverWrites(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "integration-secret")
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Shared key", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	ctx := context.Background()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: testTokenTransport{"integration-secret"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	for name, args := range map[string]map[string]any{
		"create_task":      {"projectId": "default", "title": "Anonymous"},
		"update_task":      {"taskKey": task.ID, "title": "Anonymous"},
		"add_comment":      {"taskKey": task.ID, "body": "Anonymous"},
		"transition_stage": {"taskKey": task.ID, "stage": "clarified", "note": "Anonymous"},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(result.Content)
		if !result.IsError || !strings.Contains(string(raw), "not tied to a user") {
			t.Fatalf("%s from the shared key must be refused by name, got %s", name, raw)
		}
	}
	unchanged, err := database.GetTaskByID(task.ID)
	if err != nil || unchanged.Title != "Shared key" || unchanged.Status != models.StatusToClarify {
		t.Fatalf("a refused write changed the task: %+v %v", unchanged, err)
	}
	tasks, err := database.GetTasks("", "", "", "", "default", "", "", "", "", nil, nil, false)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("a refused creation filed a task: %d %v", len(tasks), err)
	}

	for name, args := range map[string]map[string]any{
		"get_task":   {"taskKey": task.ID},
		"list_tasks": {"projectId": "default"},
		"start_run":  {"taskKey": task.ID, "skill": "implement"},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("%s must still answer the shared key: %+v %v", name, result, err)
		}
	}
}
