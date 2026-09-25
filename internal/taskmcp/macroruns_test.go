package taskmcp

import (
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

func TestRunTargetNamesExactlyOneForm(t *testing.T) {
	cases := []struct {
		task, project, macro string
		wantMacro, wantErr   bool
	}{
		{"#426", "", "", false, false},
		{"", "p1", "M-7", true, false},
		{"#426", "p1", "M-7", false, true},
		{"", "", "M-7", false, true},
		{"", "p1", "", false, true},
		{"", "", "", false, true},
	}
	for _, c := range cases {
		macro, err := runTarget(c.task, c.project, c.macro)
		if (err != nil) != c.wantErr || (err == nil && macro != c.wantMacro) {
			t.Errorf("runTarget(%q, %q, %q) = %v, %v", c.task, c.project, c.macro, macro, err)
		}
	}
}

func TestStartAndFinishAMacroRun(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SaveMacroMeta(project.ID, "M-7", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	started, err := call(t, database, "start_run", map[string]any{"projectId": project.ID, "macroKey": "M-7", "skill": "realign_macro"})
	if err != nil {
		t.Fatalf("start_run: %v", err)
	}
	runID, _ := started["id"].(string)
	if runID == "" || started["macroKey"] != "M-7" {
		t.Fatalf("unexpected run %+v", started)
	}
	if _, err := call(t, database, "finish_run", map[string]any{"taskKey": "#1", "projectId": project.ID, "macroKey": "M-7", "runId": runID, "status": "completed", "note": "x"}); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("a mixed form must be refused, got %v", err)
	}
	finished, err := call(t, database, "finish_run", map[string]any{"projectId": project.ID, "macroKey": "M-7", "runId": runID, "status": "completed", "note": "realigned"})
	if err != nil || finished["status"] != "completed" {
		t.Fatalf("finish_run: %+v %v", finished, err)
	}
}
