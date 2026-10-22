package taskmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func TestMacroResourceTools(t *testing.T) {
	database := macroDatabase(t)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Other macros", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := call(t, database, "list_macros", map[string]any{"projectId": project.ID})
	if err != nil || len(out["macros"].([]any)) != 0 {
		t.Fatalf("empty project: %v %v", out, err)
	}
	title := "Other project"
	if _, err := database.UpdateMacro(context.Background(), project.ID, "M-1", &title, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, projectID := range []string{"", "unknown"} {
		if _, err := call(t, database, "list_macros", map[string]any{"projectId": projectID}); err == nil {
			t.Fatalf("accepted project %q", projectID)
		}
	}
	out, err = call(t, database, "update_macro", map[string]any{"projectId": "default", "macroKey": "m-1", "title": "Title", "description": "Description", "closed": true, "priority": "P1", "quarter": "2026.q4", "readiness": "ready"})
	if err != nil {
		t.Fatal(err)
	}
	out, err = call(t, database, "list_macros", map[string]any{"projectId": "default"})
	if err != nil || len(out["macros"].([]any)) != 1 {
		t.Fatalf("list closed: %v %v", out, err)
	}
	out, err = call(t, database, "update_macro", map[string]any{"projectId": "default", "macroKey": "M-1", "description": "", "closed": false})
	if err != nil {
		t.Fatal(err)
	}
	macro := out["macro"].(map[string]any)
	if macro["title"] != "Title" || macro["description"] != "" || macro["closed"] != false || macro["priority"] != "p1" || macro["quarter"] != "2026-Q4" {
		t.Fatalf("partial edit: %v", macro)
	}
	saved, _ := database.GetMacro("default", "M-1")
	if len(saved.Todos) != 2 || saved.Todos[0].StoryKey != "DEFAUL-3" {
		t.Fatalf("todos changed: %+v", saved.Todos)
	}
	other, _ := database.GetMacro(project.ID, "M-1")
	if other.Title != title {
		t.Fatalf("other project changed: %+v", other)
	}
	for _, fields := range []map[string]any{
		{}, {"title": " "}, {"horizon": "invalid", "description": "rejected"}, {"priority": "p99"}, {"quarter": "2026-Q5"}, {"readiness": "invalid"}, {"macroKey": "M-404", "title": "Unknown"},
	} {
		args := map[string]any{"projectId": "default", "macroKey": "M-1"}
		for key, value := range fields {
			args[key] = value
		}
		if _, err := call(t, database, "update_macro", args); err == nil {
			t.Errorf("accepted invalid update: %v", args)
		}
	}
	after, _ := database.GetMacro("default", "M-1")
	if string(mustJSON(t, after)) != string(mustJSON(t, saved)) {
		t.Fatalf("refused edit mutated macro: %+v", after)
	}
	for _, name := range []string{"create_macro", "update_macro"} {
		for _, caller := range []*Caller{nil, {UserID: "default", Anonymous: true}} {
			if _, err := callAs(t, database, caller, name, map[string]any{"projectId": "default", "macroKey": "M-1", "title": "Refused"}); err == nil {
				t.Fatalf("anonymous %s accepted", name)
			}
		}
	}
	out, err = call(t, database, "create_macro", map[string]any{"projectId": project.ID, "title": "Created"})
	if err != nil {
		t.Fatal(err)
	}
	macro = out["macro"].(map[string]any)
	if macro["title"] != "Created" || macro["horizon"] != "now" || macro["todosMirror"] == nil {
		t.Fatalf("created: %v", macro)
	}
	for _, args := range []map[string]any{
		{"projectId": project.ID, "title": " "}, {"projectId": "unknown", "title": "Title"}, {"projectId": project.ID, "title": "Title", "horizon": "invalid"},
	} {
		if _, err := call(t, database, "create_macro", args); err == nil {
			t.Fatalf("invalid create accepted: %v", args)
		}
	}
}

func TestMacroPartialSuccessSurvivesMCPTransport(t *testing.T) {
	trackerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("refused caller wrote to tracker: %s", req.Method)
		}
		_, _ = w.Write([]byte("[]"))
	}))
	defer trackerServer.Close()
	t.Setenv("SECTILE_GITHUB_API_URL", trackerServer.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-token")
	database := macroDatabase(t)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Remote", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return NewServerWithCallers(database, nil, func(http.Header) (Caller, bool) { return tester, true })
	}, &mcp.StreamableHTTPOptions{JSONResponse: true}))
	defer server.Close()
	ctx := context.Background()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	key := ""
	for _, name := range []string{"create_macro", "update_macro"} {
		args := map[string]any{"projectId": project.ID, "title": name}
		if key != "" {
			args["macroKey"] = key
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || !result.IsError {
			t.Fatalf("expected partial error: %+v %v", result, err)
		}
		var out struct {
			Macro        models.MacroMeta `json:"macro"`
			LocalSaved   bool             `json:"localSaved"`
			TrackerError string           `json:"trackerError"`
		}
		if err := json.Unmarshal(mustJSON(t, result.StructuredContent), &out); err != nil {
			t.Fatal(err)
		}
		if !out.LocalSaved || out.TrackerError == "" || out.Macro.Key == "" || out.Macro.Title != name {
			t.Fatalf("partial result lost: %+v", out)
		}
		key = out.Macro.Key
		saved, err := database.GetMacro(project.ID, key)
		if err != nil || saved.Title != name {
			t.Fatalf("local persistence: %+v %v", saved, err)
		}
	}
}

func macroDatabase(t *testing.T) *db.DB {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	todos := []models.MacroTodo{{ID: "a", Text: "Attached", StoryKey: "DEFAUL-3"}, {ID: "b", Text: "Second"}}
	if _, err := database.SaveMacroMeta("default", "M-1", nil, nil, nil, &todos); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestGetMacroAnswersTheOrderedTodosAndTheirCopy(t *testing.T) {
	database := macroDatabase(t)
	out, err := call(t, database, "get_macro", map[string]any{"projectId": "default", "macroKey": "M-1"})
	if err != nil {
		t.Fatal(err)
	}
	macro, _ := out["macro"].(map[string]any)
	todos, _ := macro["todos"].([]any)
	if len(todos) != 2 || todos[0].(map[string]any)["id"] != "a" || todos[0].(map[string]any)["storyKey"] != "DEFAUL-3" {
		t.Fatalf("todos %+v", macro["todos"])
	}
	mirror, _ := macro["todosMirror"].(map[string]any)
	if mirror == nil || mirror["kind"] != "" || !strings.Contains(mirror["reason"].(string), "local") {
		t.Fatalf("a local board's list stays in Sectile, with its reason: %+v", macro["todosMirror"])
	}
	for _, args := range []map[string]any{
		{"projectId": "default", "macroKey": "M-9"},
		{"projectId": "nope", "macroKey": "M-1"},
	} {
		if _, err := call(t, database, "get_macro", args); err == nil {
			t.Errorf("%v must be refused", args)
		}
	}
}

func TestGetMacroAnswersTheFramingCopy(t *testing.T) {
	database := macroDatabase(t)
	out, err := call(t, database, "get_macro", map[string]any{"projectId": "default", "macroKey": "M-1"})
	if err != nil {
		t.Fatal(err)
	}
	macro, _ := out["macro"].(map[string]any)
	mirror, _ := macro["framingMirror"].(map[string]any)
	if mirror == nil || mirror["kind"] != "" || !strings.Contains(mirror["reason"].(string), "local") {
		t.Fatalf("a local board's framing stays in Sectile, with its reason: %+v", macro["framingMirror"])
	}
}

func TestUpdateMacroTodosSavesTheFullList(t *testing.T) {
	database := macroDatabase(t)
	out, err := call(t, database, "update_macro_todos", map[string]any{"projectId": "default", "macroKey": "M-1", "todos": []any{
		map[string]any{"text": "New first"},
		map[string]any{"id": "a", "text": "Attached, reworded", "storyKey": "IGNORED-1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out["todosMirror"] == nil {
		t.Fatalf("the answer carries the copy status: %+v", out)
	}
	saved, err := database.GetMacro("default", "M-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Todos) != 2 || saved.Todos[0].Text != "New first" || saved.Todos[1].ID != "a" || saved.Todos[1].StoryKey != "DEFAUL-3" {
		t.Fatalf("saved %+v", saved.Todos)
	}

	if _, err := call(t, database, "update_macro_todos", map[string]any{"projectId": "default", "macroKey": "M-1", "todos": []any{
		map[string]any{"id": "zzz", "text": "Unknown"},
	}}); err == nil {
		t.Fatal("an unknown id refuses the call")
	}
	// An agent cannot drop a line already linked to a story (#647).
	if _, err := call(t, database, "update_macro_todos", map[string]any{"projectId": "default", "macroKey": "M-1", "todos": []any{
		map[string]any{"text": "New first"},
	}}); err == nil || !strings.Contains(err.Error(), "DEFAUL-3") {
		t.Fatalf("dropping the line linked to DEFAUL-3 must be refused and name it: %v", err)
	}
	if _, err := call(t, database, "update_macro_todos", map[string]any{"projectId": "default", "macroKey": "M-9", "todos": []any{}}); err == nil || !strings.Contains(err.Error(), "M-9") {
		t.Fatalf("an unknown macro is named: %v", err)
	}
	for _, caller := range []*Caller{nil, {UserID: "default", Anonymous: true}} {
		if _, err := callAs(t, database, caller, "update_macro_todos", map[string]any{"projectId": "default", "macroKey": "M-1", "todos": []any{}}); err == nil || !strings.Contains(err.Error(), "not tied to a user") {
			t.Fatalf("an anonymous save is refused: %v", err)
		}
	}
	after, _ := database.GetMacro("default", "M-1")
	if len(after.Todos) != 2 {
		t.Fatalf("a refused call saves nothing: %+v", after.Todos)
	}
}
