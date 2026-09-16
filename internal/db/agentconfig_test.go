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

func TestAgentConfigLegacyBareCommandTemplate(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	// Rows written before templates carried {prompt} hold the CLI name alone.
	if _, err = database.UpdateSettings(models.Settings{AIProvider: "agy", AICommandTemplate: "agy"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, provider, template, wantProvider, wantTemplate, wantErr string }{
		{"inherited bare name", "", "", "agy", "", ""},
		{"project bare name", "claude", "claude", "claude", "", ""},
		{"project template kept", "claude", `claude -p "{prompt}"`, "claude", `claude -p "{prompt}"`, ""},
		{"custom stays strict", "custom", "/opt/cli run", "", "", "must contain {prompt}"},
	} {
		project, err := database.CreateProject(models.CreateProjectRequest{Name: tc.name, AIProvider: tc.provider, AICommandTemplate: tc.template})
		if err != nil {
			t.Fatal(err)
		}
		config, err := database.AgentConfig(project.ID, "")
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("%s: err=%v", tc.name, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if config.AIProvider != tc.wantProvider || config.AICommandTemplate != tc.wantTemplate {
			t.Fatalf("%s: provider=%q template=%q", tc.name, config.AIProvider, config.AICommandTemplate)
		}
	}
}
