package db_test

import (
	"encoding/json"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

func TestGeneratedSkillContracts(t *testing.T) {
	for _, framework := range []string{"openspec", "speckit"} {
		for _, stage := range db.StageSkills {
			t.Run(framework+"/"+stage.ID, func(t *testing.T) {
				content := db.RenderSkillContent(stage, framework)
				// A JSON-quoted description is a valid YAML scalar even with a
				// colon, as in handoff's "properly: confirm the merge".
				for _, document := range []string{content, db.RenderSkillCommand(stage, framework)} {
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
					if strings.Contains(content, "taskflow stage") {
						t.Fatal("non-workflow skill received a task transition")
					}
				}
				if stage.ID == "implement" || stage.ID == "specify" {
					want := "taskflow stage <KEY> " + stage.ToStage + ` --branch "<ACTUAL_BRANCH>"`
					if !strings.Contains(content, want) {
						t.Fatal("transition does not record the actual assigned branch")
					}
				}
			})
		}
	}
}

func TestRewriteStorySkillTemplate(t *testing.T) {
	// 1. Verify StageSkillByID lookup for rewrite_story and its aliases
	aliases := []string{"rewrite_story", "rewrite-story", "rewrite"}
	for _, alias := range aliases {
		skill, ok := db.StageSkillByID(alias)
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
	templates := db.ProjectSkillTemplates("speckit")
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
