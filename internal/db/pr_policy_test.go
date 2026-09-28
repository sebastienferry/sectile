package db

import (
	"path/filepath"
	"strings"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"testing"
)

func TestProjectPRPolicy(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
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
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
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

// The clarification skill carries the pull request policy only when it opens
// the pull request; every later stage skill then keeps the same draft (#580).
func TestProjectPRPolicyTextFollowsTheCreationStage(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	contents := func(timing string) map[string]string {
		project, err := database.CreateProject(models.CreateProjectRequest{Name: "Policy " + timing, PRCreationStage: timing})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, skill := range database.EffectiveProjectSkills(project.ID, "") {
			out[skill.ID] = skill.Content
		}
		return out
	}
	early := contents("clarified")
	for _, id := range []string{"clarify", "specify", "implement", "adjust", "pickup", "pickup_issues"} {
		for _, phrase := range []string{"PR creation stage: clarified.", "In the final clarification round only", "clarified transition", "never force", "Intermediate rounds open no PR", "When the clarification report or the specification files are ignored by Git"} {
			if !strings.Contains(early[id], phrase) {
				t.Errorf("%s under clarified lacks %q", id, phrase)
			}
		}
	}
	for _, timing := range []string{"specified", "implemented"} {
		later := contents(timing)
		if strings.Contains(later["clarify"], "Project pull request policy") {
			t.Errorf("clarify under %s must not carry the pull request policy", timing)
		}
		if !strings.Contains(later["specify"], "PR creation stage: "+timing+".") || strings.Contains(later["specify"], "final clarification round") {
			t.Errorf("specify under %s has the wrong policy:\n%s", timing, later["specify"])
		}
	}
}
