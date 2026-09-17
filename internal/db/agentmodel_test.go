package db

import (
	"path/filepath"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// The contract the agent downloads must already carry the model the project
// resolves, level by level, a global skill entry surviving a bare project model.
func TestAgentConfigResolvesModelAcrossLevels(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.UpdateSettings(models.Settings{
		AIProvider: "claude", AIModel: "global", AISkillModels: map[string]string{"implement": "global-implement"},
	}); err != nil {
		t.Fatal(err)
	}

	inherited, err := database.CreateProject(models.CreateProjectRequest{Name: "Inherited"})
	if err != nil {
		t.Fatal(err)
	}
	config, err := database.AgentConfig(inherited.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := agentconfig.ResolveModel(*config, "implement"); got != "global-implement" {
		t.Fatalf("global skill entry lost: %q", got)
	}
	if got := agentconfig.ResolveModel(*config, "clarify"); got != "global" {
		t.Fatalf("global model lost: %q", got)
	}

	overriding, err := database.CreateProject(models.CreateProjectRequest{Name: "Overriding", AIModel: "project"})
	if err != nil {
		t.Fatal(err)
	}
	config, err = database.AgentConfig(overriding.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := agentconfig.ResolveModel(*config, "implement"); got != "global-implement" {
		t.Fatalf("a bare project model must not silence the global skill entry: %q", got)
	}
	if got := agentconfig.ResolveModel(*config, "clarify"); got != "project" {
		t.Fatalf("project model must govern the skills no level singles out: %q", got)
	}

	perSkill, err := database.CreateProject(models.CreateProjectRequest{
		Name: "PerSkill", AISkillModels: map[string]string{"clarify": "project-clarify"},
	})
	if err != nil {
		t.Fatal(err)
	}
	config, err = database.AgentConfig(perSkill.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := agentconfig.ResolveModel(*config, "clarify"); got != "project-clarify" {
		t.Fatalf("project skill entry ignored: %q", got)
	}
	if got := agentconfig.ResolveModel(*config, "implement"); got != "global-implement" {
		t.Fatalf("a project skill entry must not erase the global one: %q", got)
	}
}

// Nothing configured anywhere must leave the contract exactly as it was before.
func TestAgentConfigWithoutModelStaysEmpty(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Plain", AIProvider: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	config, err := database.AgentConfig(project.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.AIModel != "" || len(config.AISkillModels) != 0 {
		t.Fatalf("model appeared from nowhere: %+v", config.Models())
	}
}

// A model must survive the round trip through both stores, and clearing it must
// actually clear it rather than fall back to the stored value.
func TestModelRoundTripAndClearing(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	saved, err := database.UpdateSettings(models.Settings{
		AIProvider: "claude", AIModel: "claude-opus-5", AISkillModels: map[string]string{"implement": "strong"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.AIModel != "claude-opus-5" || saved.AISkillModels["implement"] != "strong" {
		t.Fatalf("settings round trip lost the model: %+v", saved)
	}

	project, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Round trip", AIModel: "gemini-2.5-pro", AISkillModels: map[string]string{"clarify": "cheap"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if project.AIModel != "gemini-2.5-pro" || project.AISkillModels["clarify"] != "cheap" {
		t.Fatalf("project round trip lost the model: %+v", project)
	}

	empty, emptyMap := "", map[string]string{}
	cleared, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{AIModel: &empty, AISkillModels: &emptyMap})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.AIModel != "" || len(cleared.AISkillModels) != 0 {
		t.Fatalf("clearing the project model did not take effect: %+v", cleared)
	}
}

// ValidModel trims before matching, so a padded identifier passes the handler and
// reaches storage. The project writers trim it; the settings writer has to trim
// it too, or the same value round-trips clean on a project and padded globally.
func TestSettingsStoreTrimTheModel(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	saved, err := database.UpdateSettings(models.Settings{AIProvider: "claude", AIModel: "  claude-opus-5  "})
	if err != nil {
		t.Fatal(err)
	}
	if saved.AIModel != "claude-opus-5" {
		t.Fatalf("settings kept the padding: %q", saved.AIModel)
	}
	reread, err := database.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if reread.AIModel != "claude-opus-5" {
		t.Fatalf("stored model kept the padding: %q", reread.AIModel)
	}
}
