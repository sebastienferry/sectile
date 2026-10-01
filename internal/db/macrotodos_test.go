package db

import (
	"strings"
	"testing"

	"tasks/internal/models"
)

// shapedMacro stores macro M-1 on project p1 with a description and two lines,
// the first one linked to a story.
func shapedMacro(t *testing.T, d *DB) {
	t.Helper()
	seedProjectAndUser(t, d)
	description := "The shaping a slicing skill must not touch."
	todos := []models.MacroTodo{
		{ID: "l1", Text: "Linked line", StoryKey: "PE-1", TargetProjectID: "p1"},
		{ID: "l2", Text: "Plain line", Done: true},
	}
	if _, err := d.SaveMacroMeta("p1", "M-1", nil, &description, nil, &todos); err != nil {
		t.Fatalf("storing the macro: %v", err)
	}
}

func storedMacro(t *testing.T, d *DB) models.MacroMeta {
	t.Helper()
	_, macro, err := d.macroOf("p1", "M-1")
	if err != nil {
		t.Fatal(err)
	}
	return *macro
}

func todoTexts(todos []models.MacroTodo) string {
	texts := make([]string, 0, len(todos))
	for _, todo := range todos {
		texts = append(texts, todo.Text)
	}
	return strings.Join(texts, " | ")
}

// A replace keeps a named line's id, story and target, takes its new text,
// drops a plain line it leaves out and gives a new line its own id; the
// shaping is untouched (#647).
func TestUpdateMacroTodosReplaceKeepsNamedLines(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		shapedMacro(t, d)
		saved, err := d.UpdateMacroTodos("p1", "m-1", []MacroTodoLine{{ID: "l1", Text: "Linked line, reworded"}, {Text: "New line"}}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(saved) != 2 || saved[0].ID != "l1" || saved[0].StoryKey != "PE-1" || saved[0].TargetProjectID != "p1" || saved[0].Text != "Linked line, reworded" {
			t.Fatalf("the named line = %+v, want l1 reworded with its story and target", saved)
		}
		if saved[1].ID == "" || saved[1].ID == "l2" || saved[1].Text != "New line" || saved[1].Done {
			t.Fatalf("the new line = %+v, want a fresh unchecked line", saved[1])
		}
		macro := storedMacro(t, d)
		if todoTexts(macro.Todos) != "Linked line, reworded | New line" {
			t.Errorf("stored todos = %q", todoTexts(macro.Todos))
		}
		if macro.Description != "The shaping a slicing skill must not touch." {
			t.Errorf("the description changed: %q", macro.Description)
		}
	})
}

// Append keeps every stored line and adds the new ones after them.
func TestUpdateMacroTodosAppendAddsAfterTheStoredLines(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		shapedMacro(t, d)
		done := true
		saved, err := d.UpdateMacroTodos("p1", "M-1", []MacroTodoLine{{Text: "Third"}, {Text: "Fourth", Done: &done}}, MacroTodosAppend)
		if err != nil {
			t.Fatal(err)
		}
		if todoTexts(saved) != "Linked line | Plain line | Third | Fourth" || !saved[1].Done || !saved[3].Done || saved[0].StoryKey != "PE-1" {
			t.Fatalf("appended todos = %+v", saved)
		}
	})
}

// What would lose data or duplicate lines is refused, and nothing is written.
func TestUpdateMacroTodosRefusals(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		shapedMacro(t, d)
		for _, refused := range []struct {
			why   string
			lines []MacroTodoLine
			mode  string
			want  string
		}{
			{"dropping a line linked to a story", []MacroTodoLine{{ID: "l2", Text: "Plain line"}}, "", "PE-1"},
			{"an unknown id", []MacroTodoLine{{ID: "l1", Text: "Linked line"}, {ID: "gone", Text: "x"}}, "", "gone"},
			{"the same id twice", []MacroTodoLine{{ID: "l1", Text: "a"}, {ID: "l1", Text: "b"}}, "", "second time"},
			{"an id in append mode", []MacroTodoLine{{ID: "l2", Text: "Plain line"}}, MacroTodosAppend, "carry no id"},
			{"an empty line", []MacroTodoLine{{ID: "l1", Text: "Linked line"}, {Text: "  "}}, "", "no text"},
			{"an unknown mode", []MacroTodoLine{{Text: "x"}}, "merge", "mode"},
		} {
			_, err := d.UpdateMacroTodos("p1", "M-1", refused.lines, refused.mode)
			if err == nil || !strings.Contains(err.Error(), refused.want) {
				t.Errorf("%s: err = %v, want one naming %q", refused.why, err, refused.want)
			}
		}
		if got := todoTexts(storedMacro(t, d).Todos); got != "Linked line | Plain line" {
			t.Errorf("a refused call wrote the todos: %q", got)
		}
		if _, err := d.UpdateMacroTodos("p1", "M-404", []MacroTodoLine{{Text: "x"}}, ""); err == nil {
			t.Error("a macro the project does not have was accepted")
		}
	})
}
