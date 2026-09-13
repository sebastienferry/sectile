package db

import (
	"os"
	"path/filepath"
	"tasks/internal/models"
	"testing"
)

func TestProjectSaveNeverWritesRepositoryFiles(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := t.TempDir()
	instructions := filepath.Join(repo, "AGENTS.md")
	if err := os.WriteFile(instructions, []byte("personal instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Headless", RepoPath: repo, GithubRepo: "owner/repo", IssueTracker: "github"})
	if err != nil {
		t.Fatal(err)
	}
	name := "Updated"
	if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{Name: &name}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(instructions)
	if err != nil || string(raw) != "personal instructions" {
		t.Fatalf("repository instructions changed: %s, %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskflow")); !os.IsNotExist(err) {
		t.Fatalf("server provisioned repository: %v", err)
	}
	if _, err := database.InstallProjectSkills(project.ID); err == nil {
		t.Fatal("offline install must require an agent")
	}
}
