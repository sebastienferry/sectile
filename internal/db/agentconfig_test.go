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
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Metadata", GithubRepo: "project/repo", IssueTracker: "linear"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ repo, tracker, wantRepo, wantTracker string }{
		{"project/repo", "linear", "project/repo", "linear"},
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
