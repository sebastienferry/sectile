package skills_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/skills"
)

func TestGeneratedSkillContracts(t *testing.T) {
	for _, framework := range []string{"openspec", "speckit"} {
		for _, stage := range skills.StageSkills {
			t.Run(framework+"/"+stage.ID, func(t *testing.T) {
				content := skills.RenderSkillContent(stage, framework)
				// A JSON-quoted description is a valid YAML scalar even with a
				// colon, as in handoff's "properly: confirm the merge".
				for _, document := range []string{content, skills.RenderSkillCommand(stage, framework)} {
					lines := strings.SplitN(document, "\n", 5)
					found := false
					for _, line := range lines {
						if scalar, ok := strings.CutPrefix(line, "description: "); ok {
							var desc string
							if err := json.Unmarshal([]byte(scalar), &desc); err != nil || desc == "" {
								t.Fatalf("unsafe YAML description: %s (%v)", scalar, err)
							}
							found = true
						}
					}
					if !found {
						t.Fatal("missing description")
					}
				}
				// report_stage owns no stage but is the one skill that describes transitions.
				if (stage.FromStage == "" || stage.Scope == "macro") && stage.ID != "report_stage" {
					if strings.Contains(content, "transition_stage") {
						t.Fatal("non-workflow skill received a task transition")
					}
				}
				if stage.ID == "implement" || stage.ID == "specify" {
					if !strings.Contains(content, "transition_stage") || !strings.Contains(content, "actual branch") {
						t.Fatal("transition does not record the actual assigned branch")
					}
				}
				if stage.FromStage != "" && stage.Scope != "macro" && (strings.Contains(content, "sectile stage") || strings.Contains(content, "curl --")) {
					t.Fatal("workflow skill must use native MCP tools")
				}
				if stage.ID == "clarify" {
					for _, required := range []string{
						"docs/clarifications/",
						"docs(spec):",
						"Round 1",
						"Round N",
						"satisfactory",
						"Never transition new → clarified while",
					} {
						if !strings.Contains(content, required) {
							t.Fatalf("clarify skill content is missing %q", required)
						}
					}
					command := skills.RenderSkillCommand(stage, framework)
					for _, required := range []string{
						"docs/clarifications/",
						"docs(spec):",
						"Round 1",
						"Round N",
						"satisfactory",
						"Never transition new → clarified while",
					} {
						if !strings.Contains(command, required) {
							t.Fatalf("clarify skill command is missing %q", required)
						}
					}
				}
			})
		}
	}
}

// Every skill and every slash command must ask the agent to name its session
// after the work item, whichever agent runs it.
func TestGeneratedSkillsRenameTheSessionAfterTheWorkItem(t *testing.T) {
	for _, framework := range []string{"openspec", "speckit"} {
		for _, stage := range skills.StageSkills {
			// report_stage runs inside its caller's session and must not rename it.
			if stage.ID == "report_stage" {
				continue
			}
			t.Run(framework+"/"+stage.ID, func(t *testing.T) {
				item := "ticket"
				if stage.Scope == "macro" {
					item = "macro"
				}
				for _, document := range []string{
					skills.RenderSkillContent(stage, framework),
					skills.RenderSkillCommand(stage, framework),
				} {
					for _, required := range []string{
						"## Session title",
						"rename the current session to `<" + item + " ID> - <" + item + " title>`",
						"not only Claude Code",
					} {
						if !strings.Contains(document, required) {
							t.Fatalf("generated document is missing %q", required)
						}
					}
				}
			})
		}
	}
}

func TestCreatePRSkillIntegratesRemoteDefaultBranchBeforePublishing(t *testing.T) {
	skill, ok := skills.StageSkillByID("create_pr")
	if !ok {
		t.Fatal("create_pr skill missing")
	}
	content := skills.RenderSkillContent(skill, "openspec")
	for _, required := range []string{
		"git fetch origin",
		"origin/main",
		"prefer rebase when the branch is private",
		"git push --force-with-lease",
		"behind the remote default branch",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("create_pr skill is missing %q", required)
		}
	}
}

func TestRewriteStorySkillTemplate(t *testing.T) {
	// 1. Verify StageSkillByID lookup for rewrite_story and its aliases
	aliases := []string{"rewrite_story", "rewrite-story", "rewrite"}
	for _, alias := range aliases {
		skill, ok := skills.StageSkillByID(alias)
		if !ok {
			t.Errorf("Expected StageSkillByID(%q) to be found", alias)
			continue
		}
		if skill.ID != "rewrite_story" {
			t.Errorf("Expected skill ID 'rewrite_story' for alias %q, got %q", alias, skill.ID)
		}
		if skill.Command != "/rewrite-story" {
			t.Errorf("Expected command '/rewrite-story', got %q", skill.Command)
		}
	}

	// 2. Verify SkillDirNames mapping
	dirName, ok := models.SkillDirNames["rewrite_story"]
	if !ok || dirName != "rewrite-story" {
		t.Errorf("Expected SkillDirNames['rewrite_story'] to be 'rewrite-story', got %q (ok=%v)", dirName, ok)
	}

	// 3. Verify ProjectSkillTemplates contains rewrite_story
	templates := skills.ProjectSkillTemplates("speckit")
	found := false
	for _, tmpl := range templates {
		if tmpl.ID == "rewrite_story" {
			found = true
			if tmpl.DirName != "rewrite-story" {
				t.Errorf("Expected template DirName 'rewrite-story', got %q", tmpl.DirName)
			}
			if tmpl.Content == "" {
				t.Errorf("Expected non-empty template content for rewrite_story")
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected ProjectSkillTemplates to include 'rewrite_story'")
	}
}

func TestRefineMacroSkillTemplate(t *testing.T) {
	// 1. Verify StageSkillByID lookup for refine_macro and its aliases
	aliases := []string{"refine_macro", "refine-macro", "refine"}
	for _, alias := range aliases {
		skill, ok := skills.StageSkillByID(alias)
		if !ok {
			t.Errorf("Expected StageSkillByID(%q) to be found", alias)
			continue
		}
		if skill.ID != "refine_macro" {
			t.Errorf("Expected skill ID 'refine_macro' for alias %q, got %q", alias, skill.ID)
		}
		if skill.Command != "/refine-macro" {
			t.Errorf("Expected command '/refine-macro', got %q", skill.Command)
		}
		if skill.Mode != models.SkillModeInteractive {
			t.Errorf("Expected skill.Mode to be interactive for alias %q, got %q", alias, skill.Mode)
		}
	}

	// 2. Verify SkillDirNames mapping
	dirName, ok := models.SkillDirNames["refine_macro"]
	if !ok || dirName != "refine-macro" {
		t.Errorf("Expected SkillDirNames['refine_macro'] to be 'refine-macro', got %q (ok=%v)", dirName, ok)
	}

	// 3. Verify ProjectSkillTemplates contains refine_macro and interactive steps
	for _, fw := range []string{"speckit", "openspec"} {
		templates := skills.ProjectSkillTemplates(fw)
		found := false
		for _, tmpl := range templates {
			if tmpl.ID == "refine_macro" {
				found = true
				if tmpl.DirName != "refine-macro" {
					t.Errorf("Expected template DirName 'refine-macro', got %q", tmpl.DirName)
				}
				if tmpl.Content == "" {
					t.Errorf("Expected non-empty template content for refine_macro (%s)", fw)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected ProjectSkillTemplates(%s) to include 'refine_macro'", fw)
		}
	}
}

func TestClarifySkillRoundLoopInvariants(t *testing.T) {
	skill, ok := skills.StageSkillByID("clarify")
	if !ok {
		t.Fatal("clarify skill missing from catalogue")
	}
	if skill.Command != "/clarify-issue" {
		t.Fatalf("expected command /clarify-issue, got %q", skill.Command)
	}
	for _, fw := range []string{"speckit", "openspec"} {
		content := skills.RenderSkillContent(skill, fw)
		command := skills.RenderSkillCommand(skill, fw)

		for _, doc := range []string{content, command} {
			for _, required := range []string{
				"docs/clarifications/<n>.md",
				"docs(spec): clarify #<n> (round 1)",
				"docs(spec): clarify #<n> (round N)",
				"Round 1",
				"Round N",
				"## Round N - answers from the owner (<date>)",
				"satisfactory",
				"add_comment",
				"get_task",
				"Never transition new → clarified while",
			} {
				if !strings.Contains(doc, required) {
					t.Errorf("framework %s missing expected invariant %q", fw, required)
				}
			}
		}

		if !strings.Contains(command, "## Ticket\n$ARGUMENTS") {
			t.Errorf("framework %s command missing $ARGUMENTS placeholder", fw)
		}
	}
}

func TestGoldenSkillParity(t *testing.T) {
	updateGolden := os.Getenv("UPDATE_GOLDEN") == "1"
	goldenDir := filepath.Join("testdata", "golden")
	if updateGolden {
		_ = os.MkdirAll(goldenDir, 0755)
	}

	for _, framework := range []string{"openspec", "speckit"} {
		for _, stage := range skills.StageSkills {
			// 1. Skill document
			skillFile := filepath.Join(goldenDir, stage.ID+"."+framework+".skill.md")
			actualSkill := skills.RenderSkillContent(stage, framework)
			if updateGolden {
				if err := os.WriteFile(skillFile, []byte(actualSkill), 0644); err != nil {
					t.Fatalf("failed to write golden file %s: %v", skillFile, err)
				}
			} else {
				expected, err := os.ReadFile(skillFile)
				if err != nil {
					t.Fatalf("missing golden file %s: %v (run with UPDATE_GOLDEN=1 to generate)", skillFile, err)
				}
				if string(expected) != actualSkill {
					t.Errorf("skill output mismatch for %s (%s)", stage.ID, framework)
					expLines := strings.Split(string(expected), "\n")
					actLines := strings.Split(actualSkill, "\n")
					for i := 0; i < len(expLines) && i < len(actLines); i++ {
						if expLines[i] != actLines[i] {
							t.Errorf("first diff at line %d:\nexp: %q\nact: %q", i+1, expLines[i], actLines[i])
							break
						}
					}
				}
			}

			// 2. Command document
			cmdFile := filepath.Join(goldenDir, stage.ID+"."+framework+".command.md")
			actualCmd := skills.RenderSkillCommand(stage, framework)
			if updateGolden {
				if err := os.WriteFile(cmdFile, []byte(actualCmd), 0644); err != nil {
					t.Fatalf("failed to write golden file %s: %v", cmdFile, err)
				}
			} else {
				expected, err := os.ReadFile(cmdFile)
				if err != nil {
					t.Fatalf("missing golden file %s: %v (run with UPDATE_GOLDEN=1 to generate)", cmdFile, err)
				}
				if string(expected) != actualCmd {
					t.Errorf("command output mismatch for %s (%s)", stage.ID, framework)
					expLines := strings.Split(string(expected), "\n")
					actLines := strings.Split(actualCmd, "\n")
					for i := 0; i < len(expLines) && i < len(actLines); i++ {
						if expLines[i] != actLines[i] {
							t.Errorf("first diff at line %d:\nexp: %q\nact: %q", i+1, expLines[i], actLines[i])
							break
						}
					}
				}
			}
		}
	}
}

func TestSkillFragmentsIntegrity(t *testing.T) {
	requiredContracts := []string{"task-access.md", "session-title.md", "transition.md", "report-pointer.md", "pickup-header.md"}
	for _, c := range requiredContracts {
		path := filepath.Join("fragments", "contracts", c)
		data, err := os.ReadFile(path)
		if err != nil || len(strings.TrimSpace(string(data))) == 0 {
			t.Errorf("contract %s is missing or empty", path)
		}
	}

	for _, s := range skills.StageSkills {
		dir := filepath.Join("fragments", s.ID)

		// Every skill must have a goal and report
		for _, required := range []string{"goal.md", "report.md"} {
			path := filepath.Join(dir, required)
			data, err := os.ReadFile(path)
			if err != nil || len(strings.TrimSpace(string(data))) == 0 {
				t.Errorf("skill %s missing required fragment %s", s.ID, required)
			}
		}

		// Composite skills (pickup, pickup_issues) compose steps dynamically;
		// standalone skills must have steps.md (or framework variants)
		if s.ID != "pickup" && s.ID != "pickup_issues" {
			path := filepath.Join(dir, "steps.md")
			data, err := os.ReadFile(path)
			if err != nil || len(strings.TrimSpace(string(data))) == 0 {
				t.Errorf("skill %s missing steps.md", s.ID)
			}
		}
	}
}

// Every task skill but the helper points to report_stage instead of carrying the
// task-access and execution blocks, and keeps its invariant and its own gate.
func TestTaskSkillsDelegateReportingToReportStage(t *testing.T) {
	gates := map[string][]string{
		"clarify":       {"transition new → clarified only when the exit condition is met", "Never transition new → clarified while"},
		"specify":       {"call `transition_stage` with stage `specified`", "the actual branch only when this step is complete (clarified → specified)"},
		"implement":     {"call `transition_stage` with stage `implemented`", "the actual branch only when this step is complete (specified → implemented)"},
		"adjust":        {"call `transition_stage` with stage `reviewed`", "prUrl set to the verified pull request URL", "(implemented → reviewed)"},
		"handoff":       {"call `transition_stage` with stage `finished`", "(reviewed → finished)"},
		"pickup":        {"record clarified, specified and implemented after each corresponding step", "record reviewed with the PR URL", "never mark unfinished work reviewed"},
		"pickup_issues": {"record clarified, specified and implemented after each corresponding step", "combined PR URL"},
		"create_pr":     {"this skill does not change the workflow stage"},
		"rewrite_story": {"this skill does not change the workflow stage"},
	}
	for _, framework := range []string{"openspec", "speckit"} {
		for _, stage := range skills.StageSkills {
			if stage.Scope == "macro" || stage.ID == "report_stage" {
				continue
			}
			t.Run(framework+"/"+stage.ID, func(t *testing.T) {
				gate, ok := gates[stage.ID]
				if !ok {
					t.Fatalf("no expected gate for %s: add one", stage.ID)
				}
				for _, document := range []string{skills.RenderSkillContent(stage, framework), skills.RenderSkillCommand(stage, framework)} {
					if n := strings.Count(document, "## Sectile reporting"); n != 1 {
						t.Fatalf("expected one reporting section, found %d", n)
					}
					for _, required := range append([]string{
						"`/report-stage`",
						"`report-stage/SKILL.md`",
						"call start_run before work",
						"finish_run when the entire invocation ends",
						"a nested skill never finishes the outer run",
						"submit only through the supplied result contract",
					}, gate...) {
						if !strings.Contains(document, required) {
							t.Fatalf("missing %q", required)
						}
					}
					for _, removed := range []string{"## Sectile task access", "## Execution and ticket state", "localhost:8090"} {
						if strings.Contains(document, removed) {
							t.Fatalf("still carries %q, which belongs to report-stage", removed)
						}
					}
					if strings.Index(document, "## Session title") > strings.Index(document, "## Sectile reporting") {
						t.Fatal("the session title must come before the reporting pointer")
					}
				}
			})
		}
	}
}

// The helper carries every rule the other skills no longer render.
func TestReportStageCarriesTheGenericReportingRules(t *testing.T) {
	for _, alias := range []string{"report_stage", "report-stage"} {
		skill, ok := skills.StageSkillByID(alias)
		if !ok || skill.ID != "report_stage" {
			t.Fatalf("StageSkillByID(%q) = %q, %v", alias, skill.ID, ok)
		}
	}
	skill, _ := skills.StageSkillByID("report_stage")
	if skill.DirName != "report-stage" || skill.Command != "/report-stage" || skill.FromStage != "" || skill.ToStage != "" || skill.Scope == "macro" || !skill.HideFromBoard {
		t.Fatalf("unexpected catalogue entry: %#v", skill)
	}
	for _, framework := range []string{"openspec", "speckit"} {
		for _, document := range []string{skills.RenderSkillContent(skill, framework), skills.RenderSkillCommand(skill, framework)} {
			for _, required := range []string{
				"## Sectile task access",
				"http://localhost:8090",
				"Use the full task ID for mutations",
				"Do not bypass Sectile by writing directly to its database or remote tracker",
				"## Execution and ticket state",
				"call start_run with the full task primary key and skill name",
				"If SECTILE_RUN_ID or a launch runId is supplied, reuse it",
				"Nested skills reuse the outer run; only the owner finishes it",
				"A batch tracks each task separately",
				"Never start a run merely to read a task",
				"invoke `transition_stage` with the task key, completed stage, structured report note and actual branch",
				"A task holds an ordered set of pull requests",
				"Use `add_comment`",
				"If MCP is unavailable, preserve work and report the pending transition",
				"Never merge or delete remote objects",
				"Do not decide the gate",
			} {
				if !strings.Contains(document, required) {
					t.Fatalf("%s: report-stage is missing %q", framework, required)
				}
			}
			for _, absent := range []string{"## Session title", "## Sectile reporting", "Stage: "} {
				if strings.Contains(document, absent) {
					t.Fatalf("%s: report-stage must not carry %q", framework, absent)
				}
			}
		}
	}
}

// The macro skill reports no task stage: it keeps task access inline and gets no pointer.
func TestRefineMacroKeepsTaskAccessInline(t *testing.T) {
	skill, _ := skills.StageSkillByID("refine_macro")
	for _, framework := range []string{"openspec", "speckit"} {
		content := skills.RenderSkillContent(skill, framework)
		if !strings.Contains(content, "## Sectile task access") || strings.Contains(content, "report-stage") {
			t.Fatalf("%s: refine_macro changed shape", framework)
		}
	}
}
