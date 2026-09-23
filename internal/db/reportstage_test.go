package db

import (
	"strings"
	"testing"

	"tasks/internal/models"
)

// report_stage is a helper the other skills load: the board must not offer it,
// while the editor, the provisioned files and the project context still carry it.
func TestReportStageIsHiddenFromTheBoardButProvisioned(t *testing.T) {
	d, project := modeTestDB(t)

	for _, s := range d.GetAvailableSkills() {
		if s.ID == "report_stage" {
			t.Fatal("the board catalogue offers report_stage as a launch action")
		}
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Hidden helper"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.EnqueueSkillOnTask(task.ID, "report_stage", ""); err == nil || !strings.Contains(err.Error(), "skill not found") {
		t.Fatalf("report_stage launched as a job: %v", err)
	}

	entries, err := d.ListProjectSkillEditor(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	listed := false
	for _, entry := range entries {
		if entry.ID == "report_stage" {
			listed = entry.DirName == "report-stage" && entry.Command == "/report-stage" && strings.TrimSpace(entry.DefaultContent) != ""
		}
	}
	if !listed {
		t.Fatal("the skill editor does not list report_stage with its default content")
	}

	config, err := d.AgentConfig(project.ID, "", "speckit")
	if err != nil {
		t.Fatal(err)
	}
	provisioned := false
	for _, skill := range config.Skills {
		if skill.ID == "report_stage" {
			provisioned = skill.Directory == "report-stage" && skill.Command == "/report-stage" && strings.Contains(skill.Content, "transition_stage") && strings.Contains(skill.CommandContent, "transition_stage")
		}
	}
	if !provisioned {
		t.Fatal("the agent configuration does not provision report_stage")
	}
}
