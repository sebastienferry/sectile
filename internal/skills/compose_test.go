package skills_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/skills"
)

func stageSkill(t *testing.T, id string) skills.StageSkill {
	t.Helper()
	s, ok := skills.StageSkillByID(id)
	if !ok {
		t.Fatalf("%s skill missing", id)
	}
	return s
}

func exitFragment(t *testing.T, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("fragments", id, "exit.md"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func TestComposedSkillWithoutOverridesIsTheBuiltIn(t *testing.T) {
	for _, framework := range []string{"", "openspec", "speckit"} {
		for _, s := range skills.StageSkills {
			want := skills.RenderSkillContent(s, framework)
			for name, overrides := range map[string]skills.SkillOverrides{
				"empty":          {},
				"blank sections": {s.ID: {Goal: " ", Steps: "\n\n"}},
				"other skill":    {"handoff_unknown": {Steps: "never rendered"}},
			} {
				if got := skills.RenderComposedSkillContent(s, framework, overrides); got != want {
					t.Errorf("%s/%s (%s): composed skill differs from the built-in", framework, s.ID, name)
				}
			}
			if !skills.Overridable(s) {
				continue
			}
			// The editor's prefill reads back as the built-in sections.
			if _, err := skills.ParseWorkSections(s, skills.FormatWorkSections(s, skills.BuiltinWorkSections(s, framework))); err != nil {
				t.Errorf("%s/%s: the built-in prefill does not parse: %v", framework, s.ID, err)
			}
		}
	}
}

func TestWorkOverrideKeepsSectileContracts(t *testing.T) {
	for _, s := range skills.StageSkills {
		if !skills.Overridable(s) || s.FromStage == "" || strings.HasPrefix(s.ID, "pickup") {
			continue
		}
		t.Run(s.ID, func(t *testing.T) {
			builtin := skills.BuiltinWorkSections(s, "speckit")
			content := skills.RenderComposedSkillContent(s, "speckit", skills.SkillOverrides{s.ID: {Steps: "1. Follow the team's own playbook."}})
			for _, required := range []string{
				"1. Follow the team's own playbook.",
				"## Sectile task access",
				"## Session title",
				"start_run",
				"finish_run",
				"transition_stage",
				"Transition " + s.FromStage + " → " + s.ToStage + " only when the exit condition is met: ",
				"noRepositoryChange",
				strings.TrimSpace(builtin.Goal),
				strings.TrimSpace(builtin.Report),
			} {
				if !strings.Contains(content, required) {
					t.Errorf("work override lost %q", required)
				}
			}
			if strings.Contains(content, strings.TrimSpace(builtin.Steps)) {
				t.Error("the built-in steps are still rendered")
			}
		})
	}
	content := skills.RenderComposedSkillContent(stageSkill(t, "clarify"), "speckit", skills.SkillOverrides{"clarify": {Guard: "- Do not guess."}})
	if !strings.Contains(content, exitFragment(t, "clarify")) {
		t.Error("overriding the guard dropped clarify's exit condition")
	}
}

func TestPickupInlinesOverriddenStageWork(t *testing.T) {
	specify := skills.BuiltinWorkSections(stageSkill(t, "specify"), "openspec")
	for _, id := range []string{"pickup", "pickup_issues"} {
		overrides := skills.SkillOverrides{
			"specify": {Steps: "1. Write the specification the team's way."},
			id:        {Report: "- The pull request, nothing else.", Steps: "ignored: pickup steps are its stages"},
		}
		content := skills.RenderComposedSkillContent(stageSkill(t, id), "openspec", overrides)
		for _, required := range []string{
			"1. Write the specification the team's way.",
			"- The pull request, nothing else.",
			"Exit condition before recording clarified: " + exitFragment(t, "clarify"),
			"Exit condition before recording specified: this step is complete.",
			"Exit condition before recording reviewed: " + exitFragment(t, "adjust"),
			strings.TrimSpace(specify.ReadFirst),
		} {
			if !strings.Contains(content, required) {
				t.Errorf("%s is missing %q", id, required)
			}
		}
		for _, absent := range []string{strings.TrimSpace(specify.Steps), "ignored: pickup steps"} {
			if strings.Contains(content, absent) {
				t.Errorf("%s still carries %q", id, firstLines(absent, 1))
			}
		}
	}
	if got := skills.ComposedStageIDs("pickup-issue"); strings.Join(got, ",") != "clarify,specify,implement,adjust" {
		t.Fatalf("ComposedStageIDs(pickup-issue) = %v", got)
	}
	if got := skills.ComposedStageIDs("clarify"); got != nil {
		t.Fatalf("ComposedStageIDs(clarify) = %v", got)
	}
}

func TestParseWorkSectionsRejects(t *testing.T) {
	clarify := stageSkill(t, "clarify")
	for name, tc := range map[string]struct {
		skill   string
		content string
		want    string
	}{
		"preamble":     {"clarify", "Some intro.\n\n## Steps\n1. Do it.", "text before the first section"},
		"frontmatter":  {"clarify", "---\nname: clarify-issue\n---\n## Steps\n1. Do it.", "frontmatter"},
		"unknown":      {"clarify", "## Steps\n1. Do it.\n\n## Notes\nmore", `unknown section "Notes"`},
		"duplicate":    {"clarify", "## Steps\n1. Do it.\n\n## Steps\n2. Again.", "duplicate section ## Steps"},
		"guard alias":  {"implement", "## Recovery and blockers\n- Stop.\n\n## Do not\n- Stop again.", "duplicate section ## Recovery and blockers"},
		"empty":        {"clarify", "## Goal\n\n## Steps\n1. Do it.", "section ## Goal is empty"},
		"nothing":      {"clarify", "\n\n", "needs at least one"},
		"pickup steps": {"pickup", "## Steps\n1. Do it.", "inlined stages"},
		"macro":        {"refine_macro", "## Steps\n1. Do it.", "no work-only override"},
		"open fence":   {"clarify", "## Steps\n1. Write:\n```\n## Report\nx", "unclosed code fence \"```\""},
		"open tildes":  {"clarify", "## Steps\n1. Write:\n~~~~\nx\n```", "unclosed code fence \"~~~~\""},
		"indented":     {"clarify", "## Steps\n1.\n  ## Notes\nx", `unknown section "Notes"`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := skills.ParseWorkSections(stageSkill(t, tc.skill), tc.content)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := skills.ParseWorkSections(clarify, "## Steps\n1. Do it."); err != nil {
		t.Fatalf("a single section is refused: %v", err)
	}
}

func TestParseWorkSectionsIgnoresFencedHeadings(t *testing.T) {
	implement := stageSkill(t, "implement")
	content := "\r\n## Steps\r\n1. Write the file:\r\n\r\n   ```markdown\r\n## Not a section\r\n   ```\r\n\r\n### A subsection\r\n2. Done.\r\n\r\n~~~~\r\n## Still not one\r\n```\r\n~~~~\r\n\r\n## Do not\r\n- Push on red.\r\n"
	w, err := skills.ParseWorkSections(implement, content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.Steps, "## Not a section") || !strings.Contains(w.Steps, "### A subsection") || !strings.Contains(w.Steps, "## Still not one") {
		t.Fatalf("steps lost their fenced or nested headings: %q", w.Steps)
	}
	if w.Guard != "- Push on red." {
		t.Fatalf("guard = %q", w.Guard)
	}
	if w.Goal != "" || w.ReadFirst != "" || w.Report != "" {
		t.Fatalf("sections not in the override are set: %+v", w)
	}
}

func TestParseWorkSectionsReadsIndentedHeadings(t *testing.T) {
	w, err := skills.ParseWorkSections(stageSkill(t, "clarify"), "## Steps\n1. Do it.\n\n   ## Do not\n- Stop.\n\n    ## Not a heading")
	if err != nil {
		t.Fatal(err)
	}
	if w.Steps != "1. Do it." || w.Guard != "- Stop.\n\n    ## Not a heading" {
		t.Fatalf("steps = %q, guard = %q", w.Steps, w.Guard)
	}
}

func TestDirectSkillCarriesProjectVariants(t *testing.T) {
	clarify := stageSkill(t, "clarify")
	builtin := skills.BuiltinWorkSections(clarify, "")
	projects := []skills.ProjectOverrides{
		{ProjectID: "zeta", Skills: skills.SkillOverrides{"clarify": {Steps: "1. Zeta's clarification."}}},
		{ProjectID: "alpha", Skills: skills.SkillOverrides{"clarify": {Steps: "1. Alpha's clarification."}}},
		{ProjectID: "beta", Skills: skills.SkillOverrides{"handoff": {Steps: "1. Beta's handoff."}}},
	}
	content := skills.RenderDirectComposedSkillContent(clarify, nil, projects)
	alpha := strings.Index(content, "### When get_project_context reports projectId \"alpha\"\n1. Alpha's clarification.")
	zeta := strings.Index(content, "### When get_project_context reports projectId \"zeta\"\n1. Zeta's clarification.")
	otherwise := strings.Index(content, "### Otherwise\n"+strings.TrimRight(builtin.Steps, "\n"))
	if alpha < 0 || zeta < 0 || otherwise < 0 || !(alpha < zeta && zeta < otherwise) {
		t.Fatalf("variants missing or out of order (alpha %d, zeta %d, otherwise %d):\n%s", alpha, zeta, otherwise, content)
	}
	for _, required := range []string{
		"## Steps\nRead projectId from get_project_context and follow the subsection that matches it.",
		"transition_stage",
		exitFragment(t, "clarify"),
		"http://localhost:8090",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("direct skill is missing %q", required)
		}
	}
	if strings.Contains(content, "beta") {
		t.Error("a project without a clarify override got a variant")
	}
	if !strings.HasPrefix(skills.RenderDirectComposedSkillCommand(clarify, nil, projects), "---\ndescription: ") {
		t.Error("the direct command has no command frontmatter")
	}

	// Inside pickup, the variants nest under the inlined stage, the framework
	// variants of the fallback one level lower.
	specify := []skills.ProjectOverrides{{ProjectID: "alpha", Skills: skills.SkillOverrides{"specify": {ReadFirst: "- Alpha's design notes."}}}}
	pickup := skills.RenderDirectComposedSkillContent(stageSkill(t, "pickup"), nil, specify)
	for _, required := range []string{
		"#### When get_project_context reports projectId \"alpha\"\n- Alpha's design notes.",
		"#### Otherwise\nRead specFramework from get_project_context",
		"##### When get_project_context reports specFramework \"openspec\"",
	} {
		if !strings.Contains(pickup, required) {
			t.Errorf("direct pickup is missing %q", required)
		}
	}
}

func TestDirectSkillWorkstationOverrideIsTheOtherwise(t *testing.T) {
	specify := stageSkill(t, "specify")
	workstation := skills.SkillOverrides{"specify": {ReadFirst: "- My own reading list."}}
	alone := skills.RenderDirectComposedSkillContent(specify, workstation, nil)
	if !strings.Contains(alone, "## Read first\n- My own reading list.\n\n") || strings.Contains(alone, "projectId") {
		t.Fatalf("a workstation override alone is not the section itself:\n%s", alone)
	}
	projects := []skills.ProjectOverrides{{ProjectID: "alpha", Skills: skills.SkillOverrides{"specify": {ReadFirst: "- Alpha's reading list."}}}}
	both := skills.RenderDirectComposedSkillContent(specify, workstation, projects)
	if !strings.Contains(both, "### When get_project_context reports projectId \"alpha\"\n- Alpha's reading list.\n\n### Otherwise\n- My own reading list.") {
		t.Fatalf("the workstation section is not the Otherwise:\n%s", both)
	}
}

func TestDirectSkillWithoutOverridesIsUnchanged(t *testing.T) {
	unrelated := skills.SkillOverrides{"handoff": {Steps: "1. Something else."}}
	projects := []skills.ProjectOverrides{{ProjectID: "alpha", Skills: skills.SkillOverrides{"handoff": {Goal: "Another goal."}}}}
	for _, s := range skills.StageSkills {
		if s.ID == "handoff" {
			continue
		}
		if got := skills.RenderDirectComposedSkillContent(s, unrelated, projects); got != skills.RenderDirectSkillContent(s) {
			t.Errorf("%s changed for another skill's override", s.ID)
		}
		if got := skills.RenderDirectComposedSkillCommand(s, nil, nil); got != skills.RenderDirectSkillCommand(s) {
			t.Errorf("%s command differs without overrides", s.ID)
		}
	}
	// A project overriding one section leaves the others as the built-in renders them.
	specify := stageSkill(t, "specify")
	goalOnly := []skills.ProjectOverrides{{ProjectID: "alpha", Skills: skills.SkillOverrides{"specify": {Goal: "Alpha's goal."}}}}
	builtin := skills.RenderDirectSkillContent(specify)
	_, readFirst, _ := strings.Cut(builtin, "## Read first\n")
	readFirst, _, _ = strings.Cut(readFirst, "\n## Steps\n")
	if !strings.Contains(skills.RenderDirectComposedSkillContent(specify, nil, goalOnly), "## Read first\n"+readFirst+"\n## Steps\n") {
		t.Fatal("an untouched section is no longer the built-in")
	}
	// A macro skill is never composed.
	refine := stageSkill(t, "refine_macro")
	if skills.RenderDirectComposedSkillContent(refine, skills.SkillOverrides{"refine_macro": {Steps: "x"}}, nil) != skills.RenderDirectSkillContent(refine) {
		t.Fatal("a macro skill took a work-only override")
	}
}

func TestStageLaunchContractCarriesTheExitCondition(t *testing.T) {
	for _, s := range skills.StageSkills {
		contract := skills.StageLaunchContract(s)
		if s.Scope == "macro" || s.HandTransition {
			if contract != "" {
				t.Errorf("%s: a macro skill or a hand transition got a stage contract", s.ID)
			}
			continue
		}
		if !strings.HasPrefix(contract, "## Sectile stage contract\n") || !strings.Contains(contract, "start_run") || !strings.Contains(contract, "finish_run") {
			t.Errorf("%s: contract lacks the run lifecycle:\n%s", s.ID, contract)
		}
		if strings.Contains(contract, "## Execution and ticket state") {
			t.Errorf("%s: contract kept the skill's heading", s.ID)
		}
		if has := strings.Contains(contract, "transition_stage"); has != (s.FromStage != "") {
			t.Errorf("%s: transition_stage present = %v", s.ID, has)
		}
	}
	for id, exit := range map[string]string{
		"clarify": exitFragment(t, "clarify"),
		"adjust":  exitFragment(t, "adjust"),
		"pickup":  exitFragment(t, "pickup"),
		"specify": "only when the exit condition is met: this step is complete.",
	} {
		if !strings.Contains(skills.StageLaunchContract(stageSkill(t, id)), exit) {
			t.Errorf("%s: contract lacks its exit condition %q", id, exit)
		}
	}
}

func TestProjectSkillPolicy(t *testing.T) {
	if skills.ProjectSkillPolicy("clarify", "implemented") != "" || skills.ProjectSkillPolicy("clarify", "") != "" {
		t.Fatal("clarify hears about pull requests it does not open")
	}
	if skills.ProjectSkillPolicy("clarify", "clarified") != skills.ProjectPullRequestPolicy("clarified") {
		t.Fatal("clarify lacks the policy when it opens the pull request")
	}
	if skills.ProjectSkillPolicy("handoff", "implemented") != "" {
		t.Fatal("handoff got a pull-request policy")
	}
	if skills.ProjectSkillPolicy("implement", "") != skills.ProjectPullRequestPolicy("implemented") {
		t.Fatal("an unknown timing does not fall back to implementation")
	}
}
