package db

import (
	"fmt"
	"strings"

	"tasks/internal/models"
)

// MacroTodoLine is one slicing line as a macro skill writes it (#647). A field
// left nil keeps the value the stored line has, so a session that rewrites a
// line's text does not have to send back its box, its story or its target.
type MacroTodoLine struct {
	ID                   string
	Text                 string
	Done                 *bool
	StoryKey             *string
	TargetProjectID      *string
	TargetTrackerProject *string
}

// The modes of UpdateMacroTodos.
const (
	MacroTodosReplace = "replace"
	MacroTodosAppend  = "append"
)

// UpdateMacroTodos writes a macro's slicing lines and nothing else: its
// shaping, framing, horizon and title stay as they are, and nothing reaches the
// tracker, the todos being Sectile's own (#647).
//
// In replace mode the given lines become the list. A line that names an id
// updates the stored line with that id, an unknown id is refused, and a line
// without one is new. A stored line the list leaves out is dropped, unless it
// is linked to a story: a session cannot see the story the way the person
// editing the macro's panel does, so that drop is refused and the lines are
// named. In append mode the stored lines stay and the given ones, which carry
// no id, are added after them. It returns the saved lines.
func (d *DB) UpdateMacroTodos(projectID, macroKey string, lines []MacroTodoLine, mode string) ([]models.MacroTodo, error) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = MacroTodosReplace
	}
	if mode != MacroTodosReplace && mode != MacroTodosAppend {
		return nil, fmt.Errorf("mode must be %s or %s", MacroTodosReplace, MacroTodosAppend)
	}
	project, macro, err := d.macroOf(projectID, macroKey)
	if err != nil {
		return nil, err
	}
	stored := make(map[string]models.MacroTodo, len(macro.Todos))
	for _, todo := range macro.Todos {
		stored[todo.ID] = todo
	}

	var todos []models.MacroTodo
	if mode == MacroTodosAppend {
		todos = append(todos, macro.Todos...)
	}
	kept := map[string]bool{}
	for i, line := range lines {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			return nil, fmt.Errorf("todo %d has no text", i+1)
		}
		id := strings.TrimSpace(line.ID)
		var todo models.MacroTodo
		switch {
		case id != "" && mode == MacroTodosAppend:
			return nil, fmt.Errorf("todo %d names id %s: appended lines are new and carry no id", i+1, id)
		case id != "":
			existing, ok := stored[id]
			if !ok {
				return nil, fmt.Errorf("todo %d names id %s, which macro %s does not hold: read the todos again with prepare_macro_worktree", i+1, id, macro.Key)
			}
			if kept[id] {
				return nil, fmt.Errorf("todo %d names id %s a second time", i+1, id)
			}
			kept[id] = true
			todo = existing
		}
		todo.Text = text
		if line.Done != nil {
			todo.Done = *line.Done
		}
		if line.StoryKey != nil {
			todo.StoryKey = strings.TrimSpace(*line.StoryKey)
		}
		if line.TargetProjectID != nil {
			todo.TargetProjectID = strings.TrimSpace(*line.TargetProjectID)
		}
		if line.TargetTrackerProject != nil {
			todo.TargetTrackerProject = strings.TrimSpace(*line.TargetTrackerProject)
		}
		todos = append(todos, todo)
	}
	if mode == MacroTodosReplace {
		var linked []string
		for _, todo := range macro.Todos {
			if !kept[todo.ID] && strings.TrimSpace(todo.StoryKey) != "" {
				linked = append(linked, fmt.Sprintf("%s %q (story %s)", todo.ID, todo.Text, todo.StoryKey))
			}
		}
		if len(linked) > 0 {
			return nil, fmt.Errorf("the new list drops lines linked to a story, which only the macro's panel may remove: %s; keep them by id, or ask the user to remove them from the web", strings.Join(linked, "; "))
		}
	}
	if todos == nil {
		todos = []models.MacroTodo{}
	}
	saved, err := d.SaveMacroMeta(project.ID, macro.Key, nil, nil, nil, &todos)
	if err != nil {
		return nil, err
	}
	if saved.Todos == nil {
		return []models.MacroTodo{}, nil
	}
	return saved.Todos, nil
}
