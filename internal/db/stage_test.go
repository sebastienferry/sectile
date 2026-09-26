package db_test

import (
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

func TestTransitionTaskStage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize db: %v", err)
	}
	defer database.Close()

	// 1. Create a project
	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Project Stage Test",
		Slug:         "stage-test",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// 2. Create a task
	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: proj.ID,
		Title:     "Feature authentication flow",
		Labels:    []string{"#new", "backend"},
	})
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 3. Transition from new -> clarified
	updated, act, err := database.TransitionTaskStage(task.ID, "clarified", "Clarification complete: JWT tokens chosen", "", "")
	if err != nil {
		t.Fatalf("Transition to clarified failed: %v", err)
	}

	if updated.Status != models.StatusClarified {
		t.Errorf("Expected internal status %s, got %s", models.StatusClarified, updated.Status)
	}

	hasClarified := false
	hasNew := false
	for _, l := range updated.Labels {
		clean := strings.TrimPrefix(l, "#")
		if clean == "clarified" {
			hasClarified = true
		}
		if clean == "new" {
			hasNew = true
		}
	}
	if !hasClarified {
		t.Errorf("Expected labels to contain #clarified, got %v", updated.Labels)
	}
	if hasNew {
		t.Errorf("Expected previous #new label to be removed, got %v", updated.Labels)
	}

	if act == nil {
		t.Fatalf("Expected activity to be returned")
	}

	// 4. Transition by Key: clarified -> specified
	updated2, _, err := database.TransitionTaskStage(task.Key, "specified", "Spec created in openspec/...", "", "feat/auth")
	if err != nil {
		t.Fatalf("Transition by key to specified failed: %v", err)
	}

	if updated2.Status != models.StatusToImplement {
		t.Errorf("Expected internal status %s, got %s", models.StatusToImplement, updated2.Status)
	}
	if updated2.BranchName == nil || *updated2.BranchName != "feat/auth" {
		t.Errorf("Expected branch feat/auth, got %v", updated2.BranchName)
	}

	// 5. Transition to finished
	updated3, _, err := database.TransitionTaskStage(task.ID, "finished", "Merged and cleaned", "https://github.com/org/repo/pull/42", "")
	if err != nil {
		t.Fatalf("Transition to finished failed: %v", err)
	}
	if updated3.Status != models.StatusFinished && updated3.Status != models.StatusDone && updated3.Status != models.StatusToClose {
		t.Errorf("Expected finished/done/to_close status, got %s", updated3.Status)
	}
	if updated3.PrURL == nil || *updated3.PrURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("Expected PrURL to be recorded, got %v", updated3.PrURL)
	}
}
