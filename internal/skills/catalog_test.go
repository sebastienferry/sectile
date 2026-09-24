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
				if stage.FromStage == "" || stage.Scope == "macro" {
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

// A force push is only needed when published history was rewritten: forcing a branch the remote does not have yet
// fails, so every skill that publishes must pick the push from the state of origin/<branch>.
func TestPublishingSkillsForceOnlyWhenPublishedHistoryWasRewritten(t *testing.T) {
	for _, id := range []string{"create_pr", "adjust", "pickup", "pickup_issues"} {
		t.Run(id, func(t *testing.T) {
			skill, ok := skills.StageSkillByID(id)
			if !ok {
				t.Fatalf("%s skill missing", id)
			}
			content := skills.RenderSkillContent(skill, "openspec")
			for _, required := range []string{
				"does not exist (first publication): run `git push -u origin <branch>`",
				"Never force a branch the remote does not have.",
				"`git merge-base --is-ancestor origin/<branch> HEAD` succeeds (fast-forward): run a plain `git push`",
				"rewrote published history: run `git push --force-with-lease`",
				"`git rebase origin/<branch>`",
				"retry once with the same rule",
				"Never run an unguarded `git push --force`.",
			} {
				if !strings.Contains(content, required) {
					t.Fatalf("%s skill is missing %q", id, required)
				}
			}
			if strings.Contains(content, "only when an authorized private-branch rebase requires it") {
				t.Fatalf("%s skill still carries the vague force-with-lease condition", id)
			}
		})
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
	requiredContracts := []string{"task-access.md", "session-title.md", "transition.md", "pickup-header.md"}
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

// realign-macro is surgical: its body must say every rule that keeps it so,
// in both frameworks, and must not offer a source the slicing no longer has.
func TestRealignMacroSkillTemplate(t *testing.T) {
	for _, alias := range []string{"realign_macro", "realign-macro", "realign"} {
		skill, ok := skills.StageSkillByID(alias)
		if !ok || skill.ID != "realign_macro" || skill.Scope != "macro" || skill.Mode != models.SkillModeInteractive || skill.Command != "/realign-macro" {
			t.Fatalf("StageSkillByID(%q) = %+v, %v", alias, skill, ok)
		}
	}
	if dir := models.SkillDirNames["realign_macro"]; dir != "realign-macro" {
		t.Fatalf("SkillDirNames[realign_macro] = %q", dir)
	}
	skill, _ := skills.StageSkillByID("realign_macro")
	common := []string{
		"(to be removed: no longer in the slicing)",
		"leave its body untouched",
		"Do not delete an entry",
		"do not write on the default branch",
		"prepare_macro_worktree",
		"SECTILE_SPEC_REPO",
		"`macroKey`",
		"Push nothing when you wrote nothing",
	}
	byFramework := map[string][]string{
		"speckit":  {"# Realign Macro (Spec Kit SDD)", "specs/<MACRO-KEY>-<slug>/", "next free number", "Never renumber"},
		"openspec": {"# Realign Macro (OpenSpec SDD)", "openspec/changes/<MACRO-KEY>-<slug>/", "openspec validate <change-id> --strict", "### Requirement:"},
	}
	for framework, specific := range byFramework {
		content := skills.RenderSkillContent(skill, framework)
		for _, want := range append(common, specific...) {
			if !strings.Contains(content, want) {
				t.Errorf("%s: the body must say %q", framework, want)
			}
		}
		if strings.Contains(content, "`scenarios`") {
			t.Errorf("%s: the body must not mention the removed scenarios source", framework)
		}
		if strings.Contains(content, "transition_stage") {
			t.Errorf("%s: a macro skill moves no stage", framework)
		}
	}
}
