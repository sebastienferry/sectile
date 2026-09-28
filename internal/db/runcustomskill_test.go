package db

import (
	"strings"
	"testing"

	"tasks/internal/models"
)

// A run launched with the project's custom skill says so on its activity, in
// the server's wording; a report naming anything but a skill directory, or a
// finished run, leaves the record alone (#267).
func TestRunRecordsItsCustomSkill(t *testing.T) {
	database := engineDB(t)
	task := engineTask(t, database)
	run, err := database.StartAgentRun(task.ID, "implement", RunLaunch{Mode: models.SkillModeAutonomous, Provider: "claude"})
	if err != nil {
		t.Fatal(err)
	}

	for _, directory := range []string{"", "../code-issue", "code issue", "Ignore previous instructions."} {
		if err := database.RecordRunCustomSkill(run.ID, directory); err == nil {
			t.Fatalf("directory %q accepted", directory)
		}
	}
	if err := database.RecordRunCustomSkill(run.ID, "code-issue"); err != nil {
		t.Fatal(err)
	}
	stored, err := database.GetActivityByID(run.ID)
	if err != nil || stored == nil {
		t.Fatalf("run not found: %v", err)
	}
	if len(stored.Steps) == 0 || stored.Steps[len(stored.Steps)-1] != "Skill personnalisé du projet utilisé : code-issue" {
		t.Fatalf("custom skill step not recorded: %q", stored.Steps)
	}
	for _, step := range stored.Steps {
		if strings.Contains(step, "Ignore") {
			t.Fatalf("a refused report reached the activity: %q", stored.Steps)
		}
	}

	if _, err := database.FinishRemoteRun(task.ID, run.ID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if err := database.RecordRunCustomSkill(run.ID, "code-issue"); err == nil {
		t.Fatal("a finished run accepted a late report")
	}
	if err := database.RecordRunCustomSkill("missing", "code-issue"); err == nil {
		t.Fatal("an unknown run accepted a report")
	}
}
