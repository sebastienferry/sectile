package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
)

func TestProjectSaveWritesTaskflowContextFiles(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "taskflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("# Repository instructions\n"), 0644); err != nil {
		t.Fatal(err)
	}
	withoutWorktrees := false
	project, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "TaskFlow",
		RepoPath:     repo,
		IssueTracker: "github",
		GitRemoteUrl: "git@github.com:acme/taskflow.git",
		GithubRepo:   "acme/taskflow",
		TrackerUrl:   "https://github.com/acme/taskflow/issues",
		UseWorktrees: &withoutWorktrees,
	})
	if err != nil {
		t.Fatal(err)
	}

	config := readProjectContextConfig(t, repo)
	if config.ProjectID != project.ID || config.ProjectName != "TaskFlow" {
		t.Fatalf("unexpected project identity: %#v", config)
	}
	if config.GitRemoteURL != "git@github.com:acme/taskflow.git" || config.GithubRepo != "acme/taskflow" || config.TrackerURL != "https://github.com/acme/taskflow/issues" {
		t.Fatalf("remote tracker details are incomplete: %#v", config)
	}
	if config.Workflow.Operator != "TaskFlow" || config.Workflow.UseWorktrees || len(config.Workflow.Stages) != 5 {
		t.Fatalf("unexpected workflow configuration: %#v", config.Workflow)
	}

	agents := readProjectAgents(t, repo)
	if !strings.Contains(agents, "# Repository instructions") || !strings.Contains(agents, taskflowAgentsBlockStart) {
		t.Fatalf("AGENTS.md did not preserve instructions and add the TaskFlow block:\n%s", agents)
	}
	if !strings.Contains(agents, "TaskFlow operates the development workflow") || !strings.Contains(agents, "human merge and handoff") {
		t.Fatalf("AGENTS.md does not explain the managed workflow:\n%s", agents)
	}

	newTrackerURL := "https://github.com/acme/taskflow/projects/1"
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{TrackerUrl: &newTrackerURL}); err != nil {
		t.Fatal(err)
	}
	updatedConfig := readProjectContextConfig(t, repo)
	if updatedConfig.TrackerURL != newTrackerURL {
		t.Fatalf("config was not refreshed after update: %q", updatedConfig.TrackerURL)
	}
	if _, err := database.InstallProjectSkills(project.ID); err != nil {
		t.Fatal(err)
	}
	if configAfterSkillInstall := readProjectContextConfig(t, repo); configAfterSkillInstall.TrackerURL != newTrackerURL || configAfterSkillInstall.GithubRepo != "acme/taskflow" {
		t.Fatalf("skill installation erased the tracker context: %#v", configAfterSkillInstall)
	}
	if count := strings.Count(readProjectAgents(t, repo), taskflowAgentsBlockStart); count != 1 {
		t.Fatalf("expected one TaskFlow block after update, got %d", count)
	}
}

func readProjectContextConfig(t *testing.T, repo string) taskflowProjectConfig {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, ".taskflow", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config taskflowProjectConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func readProjectAgents(t *testing.T, repo string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
