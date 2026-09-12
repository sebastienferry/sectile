package db_test

import (
	"encoding/json"
	"os"
	"path/filepath"
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
					if strings.Contains(content, "/api/tasks/stage") {
						t.Fatal("non-workflow skill received a task transition")
					}
				}
				if stage.ID == "implement" || stage.ID == "specify" {
					if !strings.Contains(content, "http://localhost:8090/api/tasks/stage") || !strings.Contains(content, `"branch":"<ACTUAL_BRANCH>"`) {
						t.Fatal("transition does not record the actual assigned branch")
					}
				}
				if stage.FromStage != "" && stage.Scope != "macro" && strings.Contains(content, "taskflow stage") {
					t.Fatal("workflow skill must call the local handler instead of a CLI")
				}
			})
		}
	}
}

func TestCreatePRSkillIntegratesRemoteDefaultBranchBeforePublishing(t *testing.T) {
	skill, ok := db.StageSkillByID("create_pr")
	if !ok {
		t.Fatal("create_pr skill missing")
	}
	content := db.RenderSkillContent(skill, "openspec")
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

func TestRefineMacroSkillTemplate(t *testing.T) {
	// 1. Verify StageSkillByID lookup for refine_macro and its aliases
	aliases := []string{"refine_macro", "refine-macro", "refine"}
	for _, alias := range aliases {
		skill, ok := db.StageSkillByID(alias)
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
		if !skill.Interactive {
			t.Errorf("Expected skill.Interactive to be true for alias %q", alias)
		}
	}

	// 2. Verify SkillDirNames mapping
	dirName, ok := models.SkillDirNames["refine_macro"]
	if !ok || dirName != "refine-macro" {
		t.Errorf("Expected SkillDirNames['refine_macro'] to be 'refine-macro', got %q (ok=%v)", dirName, ok)
	}

	// 3. Verify ProjectSkillTemplates contains refine_macro and interactive steps
	for _, fw := range []string{"speckit", "openspec"} {
		templates := db.ProjectSkillTemplates(fw)
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

func TestUpdateWorkspaceSkills(t *testing.T) {
	root := "../.."
	for _, stage := range db.StageSkills {
		content := db.RenderSkillContent(stage, "openspec")
		for _, dir := range db.SkillDirsFor(root, stage.DirName) {
			if _, err := os.Stat(dir); err == nil {
				_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)
			}
		}
	}
}
