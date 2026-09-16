package db

import (
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
)

func TestResolveSkillModePrecedence(t *testing.T) {
	interactive := &models.Project{DefaultSkillMode: models.SkillModeInteractive}
	headless := &models.Project{DefaultSkillMode: models.SkillModeNonInteractive}

	cases := []struct {
		name     string
		override string
		skill    string
		project  *models.Project
		want     string
	}{
		{"override wins over everything", models.SkillModeInteractive, models.SkillModeNonInteractive, headless, models.SkillModeInteractive},
		{"skill wins over the project", "", models.SkillModeNonInteractive, interactive, models.SkillModeNonInteractive},
		{"project decides when the skill has no opinion", "", "", headless, models.SkillModeNonInteractive},
		{"interactive when nothing is set", "", "", &models.Project{}, models.SkillModeInteractive},
		{"interactive with no project at all", "", "", nil, models.SkillModeInteractive},
		{"an unknown override falls through", "sideways", "", headless, models.SkillModeNonInteractive},
		{"an unknown skill value falls through", "", "sideways", headless, models.SkillModeNonInteractive},
		{"an unknown project value falls through to interactive", "", "", &models.Project{DefaultSkillMode: "sideways"}, models.SkillModeInteractive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveSkillMode(tc.override, tc.skill, tc.project); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCheckHeadlessProviderRefusesByName(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "vibe"} {
		if err := checkHeadlessProvider(&models.Project{}, provider); err != nil {
			t.Fatalf("%s should support a headless run: %v", provider, err)
		}
	}
	for _, provider := range []string{"agy", "gemini", "cursor", ""} {
		err := checkHeadlessProvider(&models.Project{}, provider)
		if err == nil {
			t.Fatalf("provider %q should be refused", provider)
		}
		if provider != "" && !strings.Contains(err.Error(), provider) {
			t.Fatalf("refusal should name the provider, got %q", err)
		}
	}
}

func TestCheckHeadlessProviderTemplateNeedsModePlaceholder(t *testing.T) {
	// A template owns the command line, so it owns the mode. Without the
	// placeholder there is no way to express headless, and falling back to
	// interactive would open a window inside an unattended chain.
	withoutPlaceholder := &models.Project{AIProvider: "claude", AICommandTemplate: `mycli "{prompt}"`}
	if err := checkHeadlessProvider(withoutPlaceholder, "claude"); err == nil {
		t.Fatal("a template without {mode} should be refused")
	}
	withPlaceholder := &models.Project{AIProvider: "agy", AICommandTemplate: `mycli {mode} "{prompt}"`}
	if err := checkHeadlessProvider(withPlaceholder, "agy"); err != nil {
		t.Fatalf("a template carrying {mode} owns the mode: %v", err)
	}
}

func TestPerSkillModeOverridesProjectDefault(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Modes", RepoPath: t.TempDir(), DefaultSkillMode: models.SkillModeNonInteractive, AIProvider: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if p.DefaultSkillMode != models.SkillModeNonInteractive {
		t.Fatalf("project default not persisted: %q", p.DefaultSkillMode)
	}
	if got := d.skillModeFor(p.ID, "clarify", ""); got != models.SkillModeNonInteractive {
		t.Fatalf("project default should apply, got %q", got)
	}
	if _, err := d.SaveProjectSkillMode(p.ID, "clarify", models.SkillModeInteractive); err != nil {
		t.Fatal(err)
	}
	if got := d.skillModeFor(p.ID, "clarify", ""); got != models.SkillModeInteractive {
		t.Fatalf("skill setting should win, got %q", got)
	}
	if got := d.skillModeFor(p.ID, "clarify", models.SkillModeNonInteractive); got != models.SkillModeNonInteractive {
		t.Fatalf("launch override should win, got %q", got)
	}
	// Clearing hands the decision back to the project, which a bool could not express.
	if _, err := d.SaveProjectSkillMode(p.ID, "clarify", ""); err != nil {
		t.Fatal(err)
	}
	if got := d.skillModeFor(p.ID, "clarify", ""); got != models.SkillModeNonInteractive {
		t.Fatalf("cleared skill setting should fall through, got %q", got)
	}
	if _, err := d.SaveProjectSkillMode(p.ID, "clarify", "sideways"); err == nil {
		t.Fatal("an unknown mode should be rejected rather than stored")
	}
}

func TestAutonomousRunStopStage(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	newTaskAt := func(projectID, stage string) string {
		task, err := d.CreateTask(models.CreateTaskRequest{Title: "Auto " + stage, Source: "local", ProjectID: projectID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.conn.Exec("UPDATE tasks SET labels=? WHERE id=?", `["#`+stage+`"]`, task.ID); err != nil {
			t.Fatal(err)
		}
		return task.ID
	}

	byDefault, err := d.CreateProject(models.CreateProjectRequest{Name: "Default stop", RepoPath: t.TempDir(), AIProvider: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if byDefault.AutonomousStopStage != models.StopStageReviewed {
		t.Fatalf("default stop stage should be reviewed, got %q", byDefault.AutonomousStopStage)
	}
	if _, _, err := d.EnqueueAutonomousRun(newTaskAt(byDefault.ID, "reviewed")); err == nil {
		t.Fatal("a task already at the default stop stage should be refused")
	}
	// The adjust step has its own prerequisites, which are not what this test
	// covers: what matters is that `implemented` is not refused as a stop stage.
	if _, _, err := d.EnqueueAutonomousRun(newTaskAt(byDefault.ID, "implemented")); err != nil && strings.Contains(err.Error(), "revue humaine") {
		t.Fatalf("implemented should not be the stop stage by default: %v", err)
	}

	early, err := d.CreateProject(models.CreateProjectRequest{Name: "Early stop", RepoPath: t.TempDir(), AIProvider: "claude", AutonomousStopStage: models.StopStageImplemented})
	if err != nil {
		t.Fatal(err)
	}
	if early.AutonomousStopStage != models.StopStageImplemented {
		t.Fatalf("configured stop stage not persisted: %q", early.AutonomousStopStage)
	}
	if _, _, err := d.EnqueueAutonomousRun(newTaskAt(early.ID, "implemented")); err == nil {
		t.Fatal("a task at the configured stop stage should be refused")
	}
	if _, _, err := d.EnqueueAutonomousRun(newTaskAt(early.ID, "reviewed")); err == nil {
		t.Fatal("a task past the configured stop stage should be refused")
	}

	unsupported, err := d.CreateProject(models.CreateProjectRequest{Name: "No headless", RepoPath: t.TempDir(), AIProvider: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	err = func() error { _, _, err := d.EnqueueAutonomousRun(newTaskAt(unsupported.ID, "new")); return err }()
	if err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("an autonomous run on a provider without a headless mode should be refused by name, got %v", err)
	}
}

func TestNormalizeAutonomousStopStage(t *testing.T) {
	for _, value := range []string{"", "reviewed", "sideways", "REVIEWED"} {
		if got := models.NormalizeAutonomousStopStage(value); got != models.StopStageReviewed {
			t.Fatalf("%q should read as reviewed, got %q", value, got)
		}
	}
	if got := models.NormalizeAutonomousStopStage(" Implemented "); got != models.StopStageImplemented {
		t.Fatalf("implemented should be accepted, got %q", got)
	}
}
