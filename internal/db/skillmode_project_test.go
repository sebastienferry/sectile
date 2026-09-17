package db

import (
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
)

func modeTestDB(t *testing.T) (*DB, *models.Project) {
	t.Helper()
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	no := false
	// claude is one of the attested headless providers, so a full chain run is
	// not refused for a reason the test is not about.
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Modes", RepoPath: "/not-mounted", IssueTracker: "local", UseWorktrees: &no, AIProvider: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	return d, project
}

func TestSupportsAutonomousRun(t *testing.T) {
	cases := []struct {
		provider, template, autonomous string
		want                           bool
	}{
		{"claude", "", "", true},
		{"codex", "", "", true},
		{"vibe", "", "", true},
		{"agy", "", "", false},
		{"gemini", "", "", false},
		{"cursor", "", "", false},
		{"", "", "", false},
		// A configured template wins over the provider, in both directions.
		{"claude", "agy -i '{prompt}'", "", false},
		{"agy", "agy {mode:-p|-i} '{prompt}'", "", true},
		// A command written for headless use answers for itself: it needs no
		// marker, and it rescues a provider that has no attested mode.
		{"agy", "agy -i '{prompt}'", "agy -p '{prompt}'", true},
		{"gemini", "", "gemini -p '{prompt}'", true},
	}
	for _, tc := range cases {
		if got := models.SupportsAutonomousRun(tc.provider, tc.template, tc.autonomous); got != tc.want {
			t.Fatalf("SupportsAutonomousRun(%q,%q,%q) = %v, want %v", tc.provider, tc.template, tc.autonomous, got, tc.want)
		}
	}
}

// Refusing at enqueue time is what makes "no step is enqueued" true. Letting the
// chain start and fail on the agent would leave a queued step behind and an
// error nobody is watching for.
func TestFullChainRefusesProviderWithoutHeadlessMode(t *testing.T) {
	d, project := modeTestDB(t)
	for _, provider := range []string{"agy", "gemini", "cursor"} {
		value := provider
		if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{AIProvider: &value}); err != nil {
			t.Fatal(err)
		}
		task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "chain on " + provider})
		if err != nil {
			t.Fatal(err)
		}
		before := len(mustActivities(t, d, task.ID))
		_, _, err = d.EnqueueFullChainRun(task.ID)
		if err == nil {
			t.Fatalf("%s: a full chain run should be refused", provider)
		}
		if !strings.Contains(err.Error(), provider) {
			t.Fatalf("%s: the refusal should name the provider: %v", provider, err)
		}
		if after := len(mustActivities(t, d, task.ID)); after != before {
			t.Fatalf("%s: a refused chain enqueued %d step(s)", provider, after-before)
		}
	}

	// A custom template with no mode placeholder is refused for its own reason.
	template := "agy -i '{prompt}'"
	claude := "claude"
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{AIProvider: &claude, AICommandTemplate: &template}); err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "templated"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = d.EnqueueFullChainRun(task.ID)
	if err == nil || !strings.Contains(err.Error(), models.TemplateModePlaceholder) {
		t.Fatalf("a placeholder-less template should be refused by name: %v", err)
	}
}

func mustActivities(t *testing.T, d *DB, taskID string) []models.TaskActivity {
	t.Helper()
	activities, err := d.GetTaskActivities(taskID)
	if err != nil {
		t.Fatal(err)
	}
	return activities
}

// A project saved before these settings existed keeps today's behaviour: runs
// are interactive and the chain stops where it always stopped.
func TestProjectDefaultsPreserveExistingBehaviour(t *testing.T) {
	d, project := modeTestDB(t)
	if project.DefaultSkillMode != models.SkillModeUnset {
		t.Fatalf("a new project should pin no mode, got %q", project.DefaultSkillMode)
	}
	if project.FullChainStopStage != models.FullChainStopReviewed {
		t.Fatalf("stop stage = %q, want reviewed", project.FullChainStopStage)
	}
	if got := d.resolveTaskSkillMode(project.ID, "clarify", models.SkillModeUnset); got != models.SkillModeInteractive {
		t.Fatalf("resolved mode = %q, want interactive", got)
	}
	if got := d.FullChainStopStage(project.ID); got != DefaultFullChainStopStage {
		t.Fatalf("FullChainStopStage = %q, want %q", got, DefaultFullChainStopStage)
	}
}

func TestProjectSettingsRoundTrip(t *testing.T) {
	d, project := modeTestDB(t)
	mode, stop := models.SkillModeAutonomous, models.FullChainStopImplemented
	updated, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{DefaultSkillMode: &mode, FullChainStopStage: &stop})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DefaultSkillMode != models.SkillModeAutonomous || updated.FullChainStopStage != models.FullChainStopImplemented {
		t.Fatalf("not persisted: %+v", updated)
	}
	reread, err := d.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reread.DefaultSkillMode != models.SkillModeAutonomous || reread.FullChainStopStage != models.FullChainStopImplemented {
		t.Fatalf("not reread: %+v", reread)
	}
	// A skill with no opinion now follows the project.
	if got := d.resolveTaskSkillMode(project.ID, "clarify", models.SkillModeUnset); got != models.SkillModeAutonomous {
		t.Fatalf("resolved mode = %q, want autonomous", got)
	}
	// An unrecognised stored stop stage falls back to reviewed rather than
	// wedging the board.
	bad := "somewhere"
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{FullChainStopStage: &bad}); err != nil {
		t.Fatal(err)
	}
	if got := d.FullChainStopStage(project.ID); got != models.FullChainStopReviewed {
		t.Fatalf("bad stored stage resolved to %q, want reviewed", got)
	}
}

// refine_macro is the one skill the catalogue pins, and it must keep winning
// over a project that defaults to autonomous.
func TestSkillSettingWinsOverProjectDefault(t *testing.T) {
	d, project := modeTestDB(t)
	mode := models.SkillModeAutonomous
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{DefaultSkillMode: &mode}); err != nil {
		t.Fatal(err)
	}
	if got := d.resolveTaskSkillMode(project.ID, "refine_macro", models.SkillModeUnset); got != models.SkillModeInteractive {
		t.Fatalf("refine_macro resolved to %q, want interactive", got)
	}
	// A one-off override still wins over the skill.
	if got := d.resolveTaskSkillMode(project.ID, "refine_macro", models.SkillModeAutonomous); got != models.SkillModeAutonomous {
		t.Fatalf("override resolved to %q, want autonomous", got)
	}
}

func TestSetProjectSkillMode(t *testing.T) {
	d, project := modeTestDB(t)
	if err := d.SetProjectSkillMode(project.ID, "clarify", models.SkillModeAutonomous); err != nil {
		t.Fatal(err)
	}
	if got := d.ProjectSkillMode(project.ID, "clarify"); got != models.SkillModeAutonomous {
		t.Fatalf("stored skill mode = %q, want autonomous", got)
	}
	if got := d.resolveTaskSkillMode(project.ID, "clarify", models.SkillModeUnset); got != models.SkillModeAutonomous {
		t.Fatalf("resolved mode = %q, want autonomous", got)
	}
	// Clearing it puts the skill back on the project default.
	if err := d.SetProjectSkillMode(project.ID, "clarify", models.SkillModeUnset); err != nil {
		t.Fatal(err)
	}
	if got := d.ProjectSkillMode(project.ID, "clarify"); got != models.SkillModeUnset {
		t.Fatalf("cleared skill mode = %q, want unset", got)
	}
	if got := d.resolveTaskSkillMode(project.ID, "clarify", models.SkillModeUnset); got != models.SkillModeInteractive {
		t.Fatalf("resolved mode = %q, want interactive", got)
	}
	if err := d.SetProjectSkillMode(project.ID, "clarify", "headless"); err == nil {
		t.Fatal("an invalid mode should be refused")
	}
	if err := d.SetProjectSkillMode(project.ID, "no-such-skill", models.SkillModeAutonomous); err == nil {
		t.Fatal("an unknown skill should be refused")
	}
}

// Editing a skill's content must not clear the mode stored for it: they are two
// independent settings that happen to share a row.
// A skill has several spellings. Writing under one and reading under another
// made a pinned mode silently do nothing, and the launch fell back to the
// project default without saying so.
func TestProjectSkillModeResolvesAliases(t *testing.T) {
	d, project := modeTestDB(t)
	if err := d.SetProjectSkillMode(project.ID, "pickup-issue", models.SkillModeAutonomous); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"pickup", "pickup-issue", "pickup_issue", "pick"} {
		if got := d.ProjectSkillMode(project.ID, alias); got != models.SkillModeAutonomous {
			t.Fatalf("ProjectSkillMode(%q) = %q, want autonomous", alias, got)
		}
		if got := d.resolveTaskSkillMode(project.ID, alias, models.SkillModeUnset); got != models.SkillModeAutonomous {
			t.Fatalf("resolveTaskSkillMode(%q) = %q, want autonomous", alias, got)
		}
	}
	if got := d.ProjectSkillMode(project.ID, "no-such-skill"); got != models.SkillModeUnset {
		t.Fatalf("an unknown skill should pin nothing, got %q", got)
	}
}

func TestSkillContentEditKeepsMode(t *testing.T) {
	d, project := modeTestDB(t)
	if err := d.SetProjectSkillMode(project.ID, "clarify", models.SkillModeAutonomous); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveProjectSkillContent(project.ID, "clarify", "# Clarify\n\nEdited body.\n"); err != nil {
		t.Fatal(err)
	}
	if got := d.ProjectSkillMode(project.ID, "clarify"); got != models.SkillModeAutonomous {
		t.Fatalf("skill mode after a content edit = %q, want autonomous", got)
	}
}

func TestStageAtOrPast(t *testing.T) {
	cases := []struct {
		stage, target string
		want          bool
	}{
		{"new", "reviewed", false},
		{"implemented", "implemented", true},
		{"reviewed", "implemented", true},
		{"finished", "reviewed", true},
		{"specified", "implemented", false},
		{"unknown", "reviewed", false},
		{"reviewed", "unknown", false},
	}
	for _, tc := range cases {
		if got := stageAtOrPast(tc.stage, tc.target); got != tc.want {
			t.Fatalf("stageAtOrPast(%q,%q) = %v, want %v", tc.stage, tc.target, got, tc.want)
		}
	}
}

// A full chain run refuses to start on a task that is already at or past the
// project's stop stage: the rest is a human review.
func TestFullChainRefusesAtOrPastStopStage(t *testing.T) {
	d, project := modeTestDB(t)
	stop := models.FullChainStopImplemented
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{FullChainStopStage: &stop}); err != nil {
		t.Fatal(err)
	}
	// A task still at the start of the workflow is accepted: the chain has work
	// to do before the stop stage.
	fresh, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.EnqueueFullChainRun(fresh.ID); err != nil {
		t.Fatalf("a new task should start a full chain run: %v", err)
	}

	for _, stage := range []string{models.FullChainStopImplemented, models.FullChainStopReviewed} {
		task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "at " + stage})
		if err != nil {
			t.Fatal(err)
		}
		// The stage is set on the record directly: reaching it through the real
		// transition would drag in PR creation, which this test is not about.
		if _, err := d.conn.Exec(`UPDATE tasks SET labels = ? WHERE id = ?`, `["`+stage+`"]`, task.ID); err != nil {
			t.Fatal(err)
		}
		if _, _, err := d.EnqueueFullChainRun(task.ID); err == nil {
			t.Fatalf("a task at %q should be refused when the stop stage is implemented", stage)
		}
	}
}
