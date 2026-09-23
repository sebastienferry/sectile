package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/models"
)

// Editing a project saved its interactive command and silently kept the
// autonomous one it already had (#249). Both commands follow the same rule: an
// absent field keeps the stored value, an empty one clears it.
func TestUpdateProjectSavesBothCommands(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	const interactive = `claude "{prompt}"`
	const autonomous = `claude -p "{prompt}"`
	project, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Commands", AIProvider: "claude", AICommandTemplate: interactive, AICommandTemplateAutonomous: `claude -p --old "{prompt}"`,
	})
	if err != nil {
		t.Fatal(err)
	}

	padded := "  " + autonomous + "  "
	updated, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{AICommandTemplateAutonomous: &padded})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AICommandTemplateAutonomous != autonomous || updated.AICommandTemplate != interactive {
		t.Fatalf("saving the autonomous command: interactive=%q autonomous=%q", updated.AICommandTemplate, updated.AICommandTemplateAutonomous)
	}
	reread, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reread.AICommandTemplateAutonomous != autonomous {
		t.Fatalf("the autonomous command did not reach storage: %q", reread.AICommandTemplateAutonomous)
	}
	config, err := database.AgentConfig(project.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.AICommandTemplateAutonomous != autonomous {
		t.Fatalf("the agent config does not carry the saved autonomous command: %q", config.AICommandTemplateAutonomous)
	}

	name := "Commands renamed"
	kept, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if kept.AICommandTemplate != interactive || kept.AICommandTemplateAutonomous != autonomous {
		t.Fatalf("an update naming neither command changed them: interactive=%q autonomous=%q", kept.AICommandTemplate, kept.AICommandTemplateAutonomous)
	}

	empty := ""
	cleared, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{AICommandTemplate: &empty, AICommandTemplateAutonomous: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.AICommandTemplate != "" || cleared.AICommandTemplateAutonomous != "" {
		t.Fatalf("clearing both commands did not take effect: interactive=%q autonomous=%q", cleared.AICommandTemplate, cleared.AICommandTemplateAutonomous)
	}
}
