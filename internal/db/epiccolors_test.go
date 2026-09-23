package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

// TestProjectEpicColorsRoundTrip guards the setting that decides whether cards
// carry their epic's colour. A project that never asked for it must come back
// with it off, which is what keeps every existing board as it was.
func TestProjectEpicColorsRoundTrip(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Colours"})
	if err != nil {
		t.Fatal(err)
	}
	if project.EpicColors {
		t.Fatal("a new project shows epic colours, want them off")
	}

	on := true
	updated, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{EpicColors: &on})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.EpicColors {
		t.Fatal("after enabling, the project does not show epic colours")
	}

	// Both read paths must carry it: the board reads the list, the settings
	// modal reads the single project.
	reread, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reread.EpicColors {
		t.Fatal("re-read project lost epic colours")
	}
	projects, err := database.GetProjects()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range projects {
		if p.ID == project.ID {
			found = true
			if !p.EpicColors {
				t.Fatal("listed project lost epic colours")
			}
		}
	}
	if !found {
		t.Fatal("project missing from the list")
	}

	// An update that does not mention the setting leaves it alone.
	name := "Colours renamed"
	renamed, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if !renamed.EpicColors {
		t.Fatal("an unrelated update turned epic colours off")
	}

	off := false
	cleared, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{EpicColors: &off})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.EpicColors {
		t.Fatal("after disabling, the project still shows epic colours")
	}

	created, err := database.CreateProject(models.CreateProjectRequest{Name: "Colours on", EpicColors: true})
	if err != nil {
		t.Fatal(err)
	}
	if !created.EpicColors {
		t.Fatal("a project created with epic colours does not show them")
	}
}
