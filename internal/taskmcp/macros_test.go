package taskmcp

import (
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

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
