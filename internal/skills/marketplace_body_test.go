package skills

import (
	"strings"
	"testing"
)

const packBody = "# Acme clarify\n\nAsk the three Acme questions and stop.\n"

// A pack body is a prompt a third party wrote, and it is run by a CLI Sectile
// launches with permissions skipped. Whatever it says, the rendered file still
// carries every contract that attaches the agent to the workflow, or a pack
// could leave tickets stuck by omission.
func TestAPackBodyCannotRemoveAGeneratedContract(t *testing.T) {
	for _, framework := range []string{"openspec", "speckit"} {
		for _, stage := range StageSkills {
			t.Run(framework+"/"+stage.ID, func(t *testing.T) {
				content := RenderSkillWithBody(stage, framework, packBody, nil)

				if !strings.HasPrefix(content, "---\nname: "+stage.DirName+"\n") {
					t.Fatalf("frontmatter is not Sectile's: %.80q", content)
				}
				for _, required := range []string{
					"## Sectile task access",
					"## Session title",
					"rename the current session to",
					"## Project instructions",
					"Ask the three Acme questions",
				} {
					if !strings.Contains(content, required) {
						t.Fatalf("missing %q", required)
					}
				}
				if stage.FromStage != "" && stage.Scope != "macro" {
					if !strings.Contains(content, "Stage: "+stage.FromStage+" -> "+stage.ToStage) {
						t.Fatal("the stage line is gone")
					}
					if !strings.Contains(content, "transition_stage") || !strings.Contains(content, "## Execution and ticket state") {
						t.Fatal("the transition contract is gone")
					}
				}
			})
		}
	}
}

// Sectile generates the only frontmatter of a rendered file; a pack that ships
// its own has it stripped rather than duplicated.
func TestAPackBodyKeepsItsOwnFrontmatterOutOfTheFile(t *testing.T) {
	stage, _ := StageSkillByID("clarify")
	content := RenderSkillWithBody(stage, "speckit", "---\nname: something-else\ndescription: a body that declared itself\n---\n\n"+packBody, nil)

	if got := strings.Count(content, "\n---\n"); got != 1 {
		t.Fatalf("%d frontmatter blocks in the rendered file", got)
	}
	if strings.Contains(content, "something-else") {
		t.Fatal("the pack frontmatter reached the file")
	}
	if !strings.Contains(content, "Ask the three Acme questions") {
		t.Fatal("the body after the frontmatter was lost")
	}
}

// A batch run must not lag behind a stage the pack updated: pickup composes
// from the resolved bodies, not from the catalogue.
func TestPickupComposesFromTheResolvedStageBodies(t *testing.T) {
	baselines := map[string]string{"clarify": "---\nname: clarify-issue\ndescription: acme\n---\n\n" + packBody}

	for _, id := range []string{"pickup", "pickup_issues"} {
		stage, ok := StageSkillByID(id)
		if !ok {
			t.Fatalf("no %s in the catalogue", id)
		}
		content := RenderSkillContent(stage, "speckit")
		if strings.Contains(content, "Acme questions") {
			t.Fatalf("%s embedded a pack body nobody applied", id)
		}

		resolved := ProjectSkillTemplatesOver("speckit", baselines)
		for _, entry := range resolved {
			if entry.ID != id {
				continue
			}
			if !strings.Contains(entry.Content, "Ask the three Acme questions") {
				t.Fatalf("%s did not embed the pack's clarify body", id)
			}
			// The catalogue prose of the replaced stage is gone, while the
			// stages the pack does not supply keep theirs.
			if !strings.Contains(entry.Content, "### Implement Code") {
				t.Fatalf("%s lost a stage the pack does not supply", id)
			}
		}
	}
}

// A directory name is all a pack carries, and it has to land on one step.
func TestStageSkillByDirNameCoversTheWorkflow(t *testing.T) {
	for _, stage := range StageSkills {
		got, ok := StageSkillByDirName(stage.DirName)
		if !ok || got.ID != stage.ID {
			t.Fatalf("%s did not resolve to %s", stage.DirName, stage.ID)
		}
	}
	if _, ok := StageSkillByDirName("docs-writer"); ok {
		t.Fatal("an unknown directory resolved to a workflow step")
	}
}
