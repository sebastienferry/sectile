package db

import (
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/skills"
	"tasks/internal/testsqlite"
)

// fullOverride is the kind of a whole-SKILL.md save, which a save that names no
// kind no longer defaults to for a skill that takes a work-only override (#732).
func fullOverride() *models.SkillOverrideKind {
	kind := models.SkillOverrideFull
	return &kind
}

func workOverride() *models.SkillOverrideKind {
	kind := models.SkillOverrideWork
	return &kind
}

func kindTestProject(t *testing.T) (*DB, *models.Project) {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Kinds"})
	if err != nil {
		t.Fatal(err)
	}
	return database, project
}

func effectiveSkill(t *testing.T, d *DB, projectID, id string) skills.ProjectSkillTemplate {
	t.Helper()
	for _, skill := range d.EffectiveProjectSkills(projectID, "") {
		if skill.ID == id {
			return skill
		}
	}
	t.Fatalf("no effective skill %s", id)
	return skills.ProjectSkillTemplate{}
}

// A new override names no kind: it is work-only when the skill takes one, and a
// full replacement for a macro skill. A work-only override must parse into the
// work sections, and only a skill that takes one accepts it.
func TestSaveWorkOverrideDefaultsAndValidates(t *testing.T) {
	d, project := kindTestProject(t)

	entry, err := d.SaveProjectSkillContent(project.ID, "clarify", "## Steps\nProject steps.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry.OverrideKind != models.SkillOverrideWork || !entry.Overridable || !entry.IsCustom {
		t.Fatalf("a new override of a stage skill: %+v", entry)
	}
	if entry.DefaultWorkContent == "" {
		t.Fatal("the editor has no built-in work sections to start from")
	}
	stage, _ := skills.StageSkillByID("clarify")
	if _, err := skills.ParseWorkSections(stage, entry.DefaultWorkContent); err != nil {
		t.Fatalf("the built-in work sections do not parse: %v", err)
	}

	entry, err = d.SaveProjectSkillContent(project.ID, "refine_macro", "---\nname: refine-macro\n---\nWhole macro skill.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry.OverrideKind != models.SkillOverrideFull || entry.Overridable || entry.DefaultWorkContent != "" {
		t.Fatalf("a macro skill is replaced whole: %+v", entry)
	}

	refused := []struct {
		name, skill, content string
		kind                 *models.SkillOverrideKind
	}{
		{"preamble", "specify", "Intro.\n## Steps\nSteps.", workOverride()},
		{"frontmatter", "specify", "---\nname: specify-issue\n---\n## Steps\nSteps.", workOverride()},
		{"unknown section", "specify", "## Notes\nSomething.", workOverride()},
		{"duplicate section", "specify", "## Steps\nOne.\n## Steps\nTwo.", workOverride()},
		{"empty section", "specify", "## Goal\n\n## Steps\nSteps.", workOverride()},
		{"pickup steps", "pickup", "## Steps\nMy own chain.", workOverride()},
		{"macro skill", "realign_macro", "## Steps\nSteps.", workOverride()},
		{"unknown kind", "specify", "## Steps\nSteps.", func() *models.SkillOverrideKind { k := models.SkillOverrideKind("partial"); return &k }()},
		{"new full blob defaulting to work", "implement", "---\nname: implement-issue\n---\nWhole skill.", nil},
	}
	for _, c := range refused {
		if _, err := d.SaveProjectSkillContent(project.ID, c.skill, c.content, c.kind); err == nil {
			t.Errorf("%s: saved", c.name)
		}
	}
	if _, ok := d.projectSkillOverrides(project.ID)["specify"]; ok {
		t.Fatal("a refused save stored a row")
	}
	if _, err := d.SaveProjectSkillContent(project.ID, "specify", "Intro.\n## Steps\nSteps.", workOverride()); err == nil || !strings.Contains(err.Error(), "surcharge du travail invalide") {
		t.Fatalf("the parse error is not surfaced to the editor: %v", err)
	}
}

// A save that names no kind keeps the kind of the row it updates, and only an
// explicit kind switches it. An adjust reset puts a full built-in back.
func TestSaveKeepsAnExistingRowKind(t *testing.T) {
	d, project := kindTestProject(t)

	if _, err := d.SaveProjectSkillContent(project.ID, "clarify", "---\nname: clarify-issue\n---\nWhole skill.", fullOverride()); err != nil {
		t.Fatal(err)
	}
	entry, err := d.SaveProjectSkillContent(project.ID, "clarify", "---\nname: clarify-issue\n---\nWhole skill, edited.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry.OverrideKind != models.SkillOverrideFull {
		t.Fatalf("a full row turned %q", entry.OverrideKind)
	}

	if _, err := d.SaveProjectSkillContent(project.ID, "clarify", "## Report\nReport it.", workOverride()); err != nil {
		t.Fatal(err)
	}
	if entry, err = d.SaveProjectSkillContent(project.ID, "clarify", "## Report\nReport it again.", nil); err != nil || entry.OverrideKind != models.SkillOverrideWork {
		t.Fatalf("a work row did not keep its kind: %+v %v", entry, err)
	}
	if entry, err = d.SaveProjectSkillContent(project.ID, "clarify", "---\nname: clarify-issue\n---\nWhole again.", fullOverride()); err != nil || entry.OverrideKind != models.SkillOverrideFull {
		t.Fatalf("an explicit full save did not switch the row: %+v %v", entry, err)
	}

	// A mode-only row is no override: the first content saved on it defaults.
	if err := d.SetProjectSkillMode(project.ID, "implement", models.SkillModeAutonomous); err != nil {
		t.Fatal(err)
	}
	if entry, err = d.SaveProjectSkillContent(project.ID, "implement", "## Steps\nImplement it.", nil); err != nil || entry.OverrideKind != models.SkillOverrideWork || entry.Mode != models.SkillModeAutonomous {
		t.Fatalf("a mode-only row: %+v %v", entry, err)
	}

	if _, err := d.SaveProjectSkillContent(project.ID, "adjust", "## Steps\nAdjust it.", nil); err != nil {
		t.Fatal(err)
	}
	if entry, err = d.ResetProjectSkillContent(project.ID, "adjust"); err != nil || entry.OverrideKind != models.SkillOverrideFull || entry.IsCustom {
		t.Fatalf("an adjust reset kept a work kind on the built-in: %+v %v", entry, err)
	}
}

// A work-only override is composed with the Sectile contracts, and pickup, which
// inlines the stage, carries it too.
func TestEffectiveSkillsComposeWorkOverride(t *testing.T) {
	d, project := kindTestProject(t)
	if _, err := d.SaveProjectSkillContent(project.ID, "clarify", "## Steps\nProject clarification steps.", nil); err != nil {
		t.Fatal(err)
	}

	clarify := effectiveSkill(t, d, project.ID, "clarify")
	for _, want := range []string{"Project clarification steps.", "transition_stage", "start_run", "name: clarify-issue"} {
		if !strings.Contains(clarify.Content, want) {
			t.Errorf("the composed clarify lacks %q", want)
		}
	}
	if clarify.OverrideKind != models.SkillOverrideWork || clarify.WorkContent != "## Steps\nProject clarification steps." {
		t.Fatalf("the work row is not carried: kind %q, work %q", clarify.OverrideKind, clarify.WorkContent)
	}

	pickup := effectiveSkill(t, d, project.ID, "pickup")
	if !strings.Contains(pickup.Content, "Project clarification steps.") || pickup.OverrideKind != models.SkillOverrideWork || pickup.WorkContent != "" {
		t.Fatalf("pickup does not inline the overridden stage: kind %q, work %q", pickup.OverrideKind, pickup.WorkContent)
	}

	specify := effectiveSkill(t, d, project.ID, "specify")
	stage, _ := skills.StageSkillByID("specify")
	if specify.Content != skills.RenderSkillContent(stage, "speckit")+skills.ProjectSkillPolicy("specify", models.PRCreationImplemented) || specify.OverrideKind != models.SkillOverrideFull {
		t.Fatal("a skill nobody overrode changed")
	}
}

// A full replacement stays what it was before #732: the stored content, whole,
// and pickup keeps the built-in work of the stage it replaces.
func TestEffectiveSkillsKeepFullReplacement(t *testing.T) {
	d, project := kindTestProject(t)
	custom := "---\nname: clarify-issue\n---\nWhole project clarification."
	if _, err := d.SaveProjectSkillContent(project.ID, "clarify", custom, fullOverride()); err != nil {
		t.Fatal(err)
	}

	clarify := effectiveSkill(t, d, project.ID, "clarify")
	if clarify.Content != custom+skills.ProjectSkillPolicy("clarify", models.PRCreationImplemented) {
		t.Fatalf("the full replacement was not kept whole:\n%s", clarify.Content)
	}
	if clarify.OverrideKind != models.SkillOverrideFull || clarify.WorkContent != "" {
		t.Fatalf("a full row reads as work: kind %q, work %q", clarify.OverrideKind, clarify.WorkContent)
	}
	pickup := effectiveSkill(t, d, project.ID, "pickup")
	if strings.Contains(pickup.Content, "Whole project clarification.") || pickup.OverrideKind != models.SkillOverrideFull {
		t.Fatal("pickup inlined a full replacement")
	}
}
