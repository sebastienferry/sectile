package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPTemplatesPreserveCustomReferences(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.ensureProjectSkillsTable()
	custom := "Personal instructions: call taskflow_get_task; do not rewrite my text."
	if _, err := database.conn.Exec(`INSERT INTO project_skills(project_id,skill_id,content,updated_at) VALUES ('default','implement',?,'2026-09-13')`, custom); err != nil {
		t.Fatal(err)
	}
	for _, skill := range database.EffectiveProjectSkills("default", "openspec") {
		if skill.ID == "implement" {
			if !strings.HasPrefix(skill.Content, custom) || !strings.Contains(skill.Content, "from get_project_context") {
				t.Fatal("custom override or canonical appended policy lost")
			}
		} else if strings.Contains(skill.Content, "taskflow_") {
			t.Fatalf("legacy MCP reference in built-in %s", skill.ID)
		}
	}
	overrides := database.projectSkillOverrides("default")
	if overrides["implement"].content != custom {
		t.Fatal("stored custom instructions modified")
	}
}
