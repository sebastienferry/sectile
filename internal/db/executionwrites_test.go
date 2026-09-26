package db

import (
	"path/filepath"
	"reflect"
	"testing"

	"tasks/internal/models"
)

// No request writes an execution setting any more (#305): the stored values
// stay as they are, read only for the seed, and the rest of each request is
// applied.
func TestExecutionSettingsAreNeverWritten(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Execution"})
	if err != nil {
		t.Fatal(err)
	}
	// A project created after the upgrade holds the column defaults.
	if project.AIProvider != "" || project.RepoPath != "" || !project.UseWorktrees || len(project.SetupProviders) != 0 || len(project.SkillOverrides) != 0 {
		t.Fatalf("a new project holds execution values: %+v", project)
	}
	legacy := map[string]any{
		"ai_provider": "claude", "ai_command_template": "claude {prompt}", "ai_model": "opus", "ai_skill_models": `{"implement":"sonnet"}`,
		"repo_path": "/srv/app", "use_worktrees": 0, "setup_providers": `["codex"]`, "skill_overrides": `{"implement":"build-it"}`,
		"external_terminal_command": "Ghostty",
	}
	setLegacyProject(t, database, project.ID, legacy)
	before, _ := database.GetProjectByID(project.ID)
	name := "Renamed"
	updated, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := database.GetProjectByID(project.ID)
	if updated.Name != "Renamed" {
		t.Fatal("the rest of the update was not applied")
	}
	// In particular a save from the web no longer resets the setup providers.
	if after.AIProvider != before.AIProvider || after.AICommandTemplate != before.AICommandTemplate || after.AIModel != before.AIModel ||
		!reflect.DeepEqual(after.AISkillModels, before.AISkillModels) || after.RepoPath != before.RepoPath || after.UseWorktrees != before.UseWorktrees ||
		!reflect.DeepEqual(after.SetupProviders, before.SetupProviders) || !reflect.DeepEqual(after.SkillOverrides, before.SkillOverrides) ||
		after.ExternalTerminalCommand != before.ExternalTerminalCommand {
		t.Fatalf("a project execution value changed:\n before %+v\n after  %+v", before, after)
	}

	setLegacySettings(t, database, map[string]any{"ai_provider": "codex", "ai_model": "gpt-5", "repo_path": "/srv", "editor_command": "zed", "external_terminal_command": "iTerm", "ai_provider_models": `{"codex":["gpt-5"]}`})
	deployment, _ := database.GetSettings()
	saved, err := database.UpdateSettings(models.Settings{
		Theme: "light", AIProvider: "claude", AICommandTemplate: "x {prompt}", AIModel: "opus", RepoPath: "/elsewhere",
		EditorCommand: "code", ExternalTerminalCommand: "Warp", AIProviderModels: map[string][]string{"claude": {"opus"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := database.GetSettings()
	if saved.Theme != "light" || stored.AIProvider != "codex" || stored.AICommandTemplate != deployment.AICommandTemplate || stored.AIModel != "gpt-5" || stored.RepoPath != "/srv" ||
		stored.EditorCommand != "zed" || stored.ExternalTerminalCommand != deployment.ExternalTerminalCommand || len(stored.AIProviderModels["codex"]) != 1 || stored.AIProviderModels["claude"] != nil {
		t.Fatalf("a deployment execution value changed:\n before %+v\n after  %+v", deployment, stored)
	}

	owner, err := database.SignInLocal("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.UpdateUserSettings(owner.ID, models.Settings{Theme: "dark"}); err != nil {
		t.Fatal(err)
	}
	setLegacyColumns(t, database, "user_settings", "user_id", owner.ID, map[string]any{"editor_command": "cursor", "external_terminal_command": "kitty"})
	if _, err := database.UpdateUserSettings(owner.ID, models.Settings{Theme: "light", EditorCommand: "vim", ExternalTerminalCommand: "Warp"}); err != nil {
		t.Fatal(err)
	}
	var editor, terminal, theme string
	if err := database.conn.QueryRow("SELECT editor_command, external_terminal_command, theme FROM user_settings WHERE user_id = ?", owner.ID).Scan(&editor, &terminal, &theme); err != nil {
		t.Fatal(err)
	}
	if editor != "cursor" || terminal != "kitty" || theme != "light" {
		t.Fatalf("personal execution values changed, or the rest was lost: %q %q %q", editor, terminal, theme)
	}
}
