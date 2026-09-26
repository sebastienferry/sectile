package db

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
)

func TestAgentConfigRepositoryMetadata(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.UpdateSettings(models.Settings{GithubRepo: "global/repo", IssueTracker: "github", JiraAPIToken: "secret-token", RepoPath: "/server/private"}); err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Metadata", GithubRepo: "project/repo", IssueTracker: "jira"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ repo, tracker, wantRepo, wantTracker string }{
		{"project/repo", "jira", "project/repo", "jira"},
		{"", "", "global/repo", "github"},
	} {
		if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{GithubRepo: &tc.repo, IssueTracker: &tc.tracker}); err != nil {
			t.Fatal(err)
		}
		config, err := database.AgentConfig(project.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if config.GithubRepo != tc.wantRepo || config.IssueTracker != tc.wantTracker {
			t.Fatalf("metadata: %+v", config)
		}
		raw, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"secret-token", "/server/private"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("config leaked %s", secret)
			}
		}
	}
}

// The configuration carries the method only (#305): whatever a database
// written before the upgrade stores, no execution value reaches the agent, and
// each skill names the stage's standard command.
func TestAgentConfigCarriesNoExecutionSetting(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	setLegacySettings(t, database, map[string]any{"ai_provider": "codex", "ai_command_template": "codex {prompt}", "ai_model": "gpt-5", "external_terminal_command": "iTerm"})
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Execution"})
	if err != nil {
		t.Fatal(err)
	}
	setLegacyProject(t, database, project.ID, map[string]any{
		"ai_provider": "claude", "ai_model": "opus", "use_worktrees": 0, "setup_providers": `["codex"]`,
		"skill_overrides": `{"implement":"build-it"}`, "external_terminal_command": "Ghostty",
	})
	config, err := database.AgentConfig(project.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.AIProvider != "" || config.AICommandTemplate != "" || config.AICommandTemplateAutonomous != "" || config.AIModel != "" ||
		config.AISkillModels != nil || config.ExternalTerminalCommand != "" || config.SetupProviders != nil || config.UseWorktrees {
		t.Fatalf("an execution value reached the configuration: %+v", config)
	}
	for _, skill := range config.Skills {
		if skill.ID == "implement" && skill.Command != "/code-issue" {
			t.Fatalf("implement must run its standard command, got %q", skill.Command)
		}
	}
}

// The seed reproduces what AgentConfig composed before #305, project row over
// deployment, including the legacy bare CLI name that never reached a runner.
func TestLegacyProjectExecutionComposesAsBefore(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	setLegacySettings(t, database, map[string]any{
		"ai_provider": "agy", "ai_command_template": "agy", "ai_command_template_autonomous": "agy -p {prompt}",
		"ai_model": "global", "ai_skill_models": `{"clarify":"global-clarify"}`, "external_terminal_command": "iTerm",
	})
	for _, tc := range []struct {
		name, provider, template, wantProvider, wantTemplate, wantAutonomous string
	}{
		{"inherited bare name", "", "", "agy", "", "agy -p {prompt}"},
		{"project bare name", "claude", "claude", "claude", "", "agy -p {prompt}"},
		{"project template kept", "claude", `claude -p "{prompt}"`, "claude", `claude -p "{prompt}"`, "agy -p {prompt}"},
	} {
		project, err := database.CreateProject(models.CreateProjectRequest{Name: tc.name})
		if err != nil {
			t.Fatal(err)
		}
		setLegacyProject(t, database, project.ID, map[string]any{"ai_provider": tc.provider, "ai_command_template": tc.template, "ai_model": "project"})
		seed, err := database.LegacyProjectExecution(project.ID)
		if err != nil {
			t.Fatal(err)
		}
		if seed.AIProvider != tc.wantProvider || seed.AICommandTemplate != tc.wantTemplate || seed.AICommandTemplateAutonomous != tc.wantAutonomous {
			t.Fatalf("%s: %+v", tc.name, seed)
		}
		// The project's bare model does not silence the deployment's per-skill entry.
		if seed.AIModel != "project" || seed.AISkillModels["clarify"] != "global-clarify" {
			t.Fatalf("%s models: %+v", tc.name, seed)
		}
		// The deployment terminal travels with the defaults, not with each project.
		if seed.Terminal != "" || seed.UseWorktrees == nil || !*seed.UseWorktrees {
			t.Fatalf("%s: %+v", tc.name, seed)
		}
	}
}

func TestLegacyWorkstationExecutionTakesTheCallersTerminalAndEditor(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	setLegacySettings(t, database, map[string]any{"ai_provider": "claude", "external_terminal_command": "iTerm", "editor_command": "code", "ai_provider_models": `{"claude":["opus"]}`})
	owner, err := database.SignInLocal("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	seed, err := database.LegacyWorkstationExecution(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	// No personal row: the deployment's values, and the default editor is not sent.
	if seed.AIProvider != "claude" || seed.Terminal != "iTerm" || seed.EditorCommand != "" || len(seed.AIProviderModels["claude"]) != 1 {
		t.Fatalf("seed: %+v", seed)
	}
	if _, err := database.UpdateUserSettings(owner.ID, models.Settings{Theme: "light"}); err != nil {
		t.Fatal(err)
	}
	setLegacyColumns(t, database, "user_settings", "user_id", owner.ID, map[string]any{"external_terminal_command": "Ghostty", "editor_command": "zed"})
	if seed, _ = database.LegacyWorkstationExecution(owner.ID); seed.Terminal != "Ghostty" || seed.EditorCommand != "zed" {
		t.Fatalf("the caller's own terminal and editor: %+v", seed)
	}
}
