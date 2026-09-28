package db

import (
	"path/filepath"
	"strings"
	"tasks/internal/models"
	"testing"
)

func TestProjectPRPolicy(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Policy"})
	if err != nil {
		t.Fatal(err)
	}
	if project.PRCreationStage != "implemented" {
		t.Fatal(project.PRCreationStage)
	}
	early := "specified"
	project, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{PRCreationStage: &early})
	if err != nil {
		t.Fatal(err)
	}
	config, err := database.AgentConfig(project.ID, "")
	if err != nil || config.PRCreationStage != "specified" {
		t.Fatalf("%+v %v", config, err)
	}
	found := false
	for _, skill := range config.Skills {
		if skill.ID == "specify" {
			found = true
			if !strings.Contains(skill.Content, "open a draft PR/MR") || !strings.Contains(skill.Content, "specified transition") || !strings.Contains(skill.Content, "When the specification files are ignored by Git (dropped artefacts), open no PR at this stage") {
				t.Fatal(skill.Content)
			}
		}
	}
	if !found {
		t.Fatal("specify skill missing")
	}
	invalid := "new"
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{PRCreationStage: &invalid}); err == nil {
		t.Fatal("invalid policy accepted")
	}
	reread, err := database.GetProjectByID(project.ID)
	if err != nil || reread.PRCreationStage != "specified" {
		t.Fatalf("%+v %v", reread, err)
	}
}

// A project may open its pull request at clarification (#580): the value is
// stored, handed to agents, and an unknown value still leaves it unchanged.
func TestProjectPRPolicyClarified(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	created, err := database.CreateProject(models.CreateProjectRequest{Name: "Early", PRCreationStage: "clarified"})
	if err != nil || created.PRCreationStage != "clarified" {
		t.Fatalf("%+v %v", created, err)
	}
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Policy"})
	if err != nil {
		t.Fatal(err)
	}
	early := "clarified"
	if _, err = database.UpdateProject(project.ID, models.UpdateProjectRequest{PRCreationStage: &early}); err != nil {
		t.Fatal(err)
	}
	config, err := database.AgentConfig(project.ID, "")
	if err != nil || config.PRCreationStage != "clarified" {
		t.Fatalf("%+v %v", config, err)
	}
	invalid := "reviewed"
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{PRCreationStage: &invalid}); err == nil || !strings.Contains(err.Error(), "clarified, specified or implemented") {
		t.Fatalf("invalid policy: %v", err)
	}
	if _, err := database.CreateProject(models.CreateProjectRequest{Name: "Bad", PRCreationStage: "new"}); err == nil {
		t.Fatal("invalid policy accepted on create")
	}
	reread, err := database.GetProjectByID(project.ID)
	if err != nil || reread.PRCreationStage != "clarified" {
		t.Fatalf("%+v %v", reread, err)
	}
}
