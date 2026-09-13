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
	if config.Workflow.Operator != "Sectile" || config.Workflow.UseWorktrees || len(config.Workflow.Stages) != 5 {
		t.Fatalf("unexpected workflow configuration: %#v", config.Workflow)
	}

	agents := readProjectAgents(t, repo)
	if !strings.Contains(agents, "# Repository instructions") || !strings.Contains(agents, taskflowAgentsBlockStart) {
		t.Fatalf("AGENTS.md did not preserve instructions and add the TaskFlow block:\n%s", agents)
	}
	if !strings.Contains(agents, "Sectile operates the development workflow") || !strings.Contains(agents, "human merge and handoff") {
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

func TestProjectInstructionsIdempotenceAndUnknownKeys(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := t.TempDir()
	initialAgents := "# Header Instructions\n\nSome custom instructions before the block.\n"
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte(initialAgents), 0644); err != nil {
		t.Fatal(err)
	}

	project, err := database.CreateProject(models.CreateProjectRequest{
		Name:     "SectileProject",
		RepoPath: repo,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Inject an unknown key into .taskflow/config.json
	configPath := filepath.Join(repo, ".taskflow", "config.json")
	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var configMap map[string]interface{}
	if err := json.Unmarshal(rawConfig, &configMap); err != nil {
		t.Fatal(err)
	}
	configMap["customPluginConfig"] = map[string]interface{}{
		"active": true,
		"rate":   42,
	}
	injected, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, injected, 0644); err != nil {
		t.Fatal(err)
	}

	// Also add user instructions after the block in AGENTS.md
	agentsContent := readProjectAgents(t, repo)
	agentsContent += "\n## Custom User Section\nDo not delete this section!\n"
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte(agentsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Call writeProjectContextFiles multiple times (simulating updates/regenerations)
	for i := 0; i < 3; i++ {
		if err := writeProjectContextFiles(project); err != nil {
			t.Fatalf("iteration %d failed: %v", i, err)
		}
	}

	// 3. Verify .taskflow/config.json still has the unknown key preserved
	rawAfter, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var configMapAfter map[string]interface{}
	if err := json.Unmarshal(rawAfter, &configMapAfter); err != nil {
		t.Fatal(err)
	}
	if val, exists := configMapAfter["customPluginConfig"]; !exists {
		t.Fatalf("customPluginConfig was dropped from config.json: %#v", configMapAfter)
	} else {
		pluginMap, ok := val.(map[string]interface{})
		if !ok || pluginMap["active"] != true || pluginMap["rate"] != float64(42) {
			t.Fatalf("customPluginConfig content mutated: %#v", val)
		}
	}

	// 4. Verify AGENTS.md:
	// - exactly one start marker and end marker
	// - contains the custom sections before and after
	// - identifies product as Sectile
	agentsAfter := readProjectAgents(t, repo)
	if count := strings.Count(agentsAfter, taskflowAgentsBlockStart); count != 1 {
		t.Fatalf("expected exactly 1 start marker, got %d:\n%s", count, agentsAfter)
	}
	if count := strings.Count(agentsAfter, taskflowAgentsBlockEnd); count != 1 {
		t.Fatalf("expected exactly 1 end marker, got %d:\n%s", count, agentsAfter)
	}
	if !strings.Contains(agentsAfter, "# Header Instructions") {
		t.Fatal("user header instructions were lost")
	}
	if !strings.Contains(agentsAfter, "## Custom User Section") {
		t.Fatal("user section after the block was lost")
	}
	if !strings.Contains(agentsAfter, "## Sectile workflow") {
		t.Fatal("Sectile workflow heading missing")
	}
}
