package db

import (
	"reflect"
	"strings"
	"testing"

	"tasks/internal/models"
)

func TestNormalizeRoadmapProjects(t *testing.T) {
	got := NormalizeRoadmapProjects([]string{"abc, DEF abc", " SFE ;xyz"}, "sfe")
	if want := []string{"ABC", "DEF", "XYZ"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := NormalizeRoadmapProjects(nil, "PE"); len(got) != 0 {
		t.Fatalf("an empty declaration stays empty, got %v", got)
	}
}

func TestRoadmapProjectsAreStoredNormalised(t *testing.T) {
	database, proj, _ := sddProject(t, "speckit")
	declared := []string{"abc, def", "PE"}
	updated, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{RoadmapProjects: &declared})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"ABC", "DEF"}; !reflect.DeepEqual(updated.RoadmapProjects, want) {
		t.Fatalf("stored %v, want %v", updated.RoadmapProjects, want)
	}
}

// A story key of a declared project attaches to its line on import; a key of
// an undeclared one stays in the text, as before.
func TestSlicingAttachesDeclaredRoadmapKeys(t *testing.T) {
	database, proj, repo := sddProject(t, "speckit")
	declared := []string{"ABC"}
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{RoadmapProjects: &declared}); err != nil {
		t.Fatal(err)
	}
	writeSpecDir(t, repo, "specs", "PE-460-roadmap", map[string]string{"tasks.md": "## 1. ABC-12 Do the thing\n\n## 2. XYZ-3 Not declared\n\n## 3. PE-4 Our own\n"})
	seedMacro(t, database, proj.ID, "PE-460")

	meta, _, err := database.TodosFromSDD(proj.ID, "PE-460", SlicingFromTasks)
	if err != nil {
		t.Fatalf("slicing: %v", err)
	}
	keys := map[string]string{}
	for _, todo := range meta.Todos {
		keys[todo.Text] = todo.StoryKey
	}
	if keys["Do the thing"] != "ABC-12" || keys["Our own"] != "PE-4" {
		t.Fatalf("declared and own keys must attach, got %v", keys)
	}
	for text, key := range keys {
		if strings.Contains(text, "Not declared") && key != "" {
			t.Fatalf("an undeclared key must not attach, got %q on %q", key, text)
		}
	}

	// Nothing is ever written to a roadmap project from the slicing.
	var line string
	for _, todo := range meta.Todos {
		if todo.StoryKey == "ABC-12" {
			line = todo.ID
		}
	}
	if _, _, _, err := database.CreateStoryFromMacroTodo(proj.ID, "PE-460", line); err == nil || !strings.Contains(err.Error(), "projet de roadmap") {
		t.Fatalf("a roadmap project's line must be refused as read-only, got %v", err)
	}
}
