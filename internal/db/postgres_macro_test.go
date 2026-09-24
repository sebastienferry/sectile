package db

import (
	"reflect"
	"testing"

	"tasks/internal/models"
)

// TestPostgresMacroWorkflowColumns runs #426's storage on the engine that
// enforces what SQLite does not: a macro run is a project activity whose
// task_id is NULL (the column references tasks), and the two new project
// columns round-trip.
func TestPostgresMacroWorkflowColumns(t *testing.T) {
	d := openPostgres(t)
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform-pg", IssueTracker: "jira", JiraProject: "PE", SpecRepoPath: " /wiki ", RoadmapProjects: []string{"abc, def"}})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if project.SpecRepoPath != "/wiki" || !reflect.DeepEqual(project.RoadmapProjects, []string{"ABC", "DEF"}) {
		t.Fatalf("project columns: %q %v", project.SpecRepoPath, project.RoadmapProjects)
	}
	if _, err := d.SaveMacroMeta(project.ID, "PE-100", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	run, err := d.StartMacroRun(project.ID, "pe-100", "realign_macro", RunLaunch{UserID: "u1", Mode: models.SkillModeInteractive})
	if err != nil {
		t.Fatalf("a macro run must be storable with no task: %v", err)
	}
	if active, err := d.ActiveRunOnMacro(project.ID, "PE-100"); err != nil || active == nil || active.ID != run.ID {
		t.Fatalf("active run: %+v %v", active, err)
	}
	if _, err := d.FinishMacroRunAs(Actor{ID: "u1"}, false, project.ID, "PE-100", run.ID, "completed", "done"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	runs, err := d.MacroRuns(project.ID, "PE-100", 10)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatalf("history: %+v %v", runs, err)
	}
}
