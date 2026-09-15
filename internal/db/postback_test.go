package db

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	tmpDir, err := os.MkdirTemp("", "sectile-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := NewDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create test db: %v", err)
	}

	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return database, cleanup
}

func TestPostBackTaskMutationsAndIdempotency(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Create initial task
	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID:   "default",
		Title:       "Original Title",
		Description: "Original Description",
		Status:      models.StatusToClarify,
		Priority:    models.PriorityMedium,
	})
	if err != nil || task == nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 1. Test field mutations via PostBackTask
	newTitle := "Post-Back Title"
	newDesc := "Updated via post-back"
	newStage := "specified"
	newAssignee := "Alice Dev"
	newBranch := "feature/21-post-back"
	newPr := "https://github.com/owner/repo/pull/42"
	newTrackerStatus := "In Progress"

	payload := models.TaskPostBackPayload{
		TaskID:        task.ID,
		TaskKey:       task.Key,
		Title:         &newTitle,
		Description:   &newDesc,
		Stage:         &newStage,
		Assignee:      &newAssignee,
		BranchName:    &newBranch,
		PrURL:         &newPr,
		TrackerStatus: &newTrackerStatus,
	}

	updatedTask, _, err := database.PostBackTask(payload)
	if err != nil {
		t.Fatalf("PostBackTask failed: %v", err)
	}

	if updatedTask.Title != newTitle {
		t.Errorf("expected Title %q, got %q", newTitle, updatedTask.Title)
	}
	if updatedTask.Description != newDesc {
		t.Errorf("expected Description %q, got %q", newDesc, updatedTask.Description)
	}
	if updatedTask.Assignee != newAssignee {
		t.Errorf("expected Assignee %q, got %q", newAssignee, updatedTask.Assignee)
	}
	if updatedTask.BranchName == nil || *updatedTask.BranchName != newBranch {
		t.Errorf("expected BranchName %q, got %v", newBranch, updatedTask.BranchName)
	}
	if updatedTask.PrURL == nil || *updatedTask.PrURL != newPr {
		t.Errorf("expected PrURL %q, got %v", newPr, updatedTask.PrURL)
	}
	if updatedTask.TrackerStatus != newTrackerStatus {
		t.Errorf("expected TrackerStatus %q, got %q", newTrackerStatus, updatedTask.TrackerStatus)
	}
	if updatedTask.Status != models.StatusToImplement {
		t.Errorf("expected Status %q for stage 'specified', got %q", models.StatusToImplement, updatedTask.Status)
	}

	// 2. Test Idempotency (re-running identical payload)
	secondTask, _, err := database.PostBackTask(payload)
	if err != nil {
		t.Fatalf("second PostBackTask execution failed: %v", err)
	}
	if secondTask.Title != newTitle || secondTask.Assignee != newAssignee {
		t.Errorf("idempotent post-back altered unexpected state: %+v", secondTask)
	}
}

func TestPostBackTaskFailureAuditing(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: "default",
		Title:     "Auditing Task",
		Status:    models.StatusToClarify,
	})
	if err != nil || task == nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Test failure auditing when tracker operation fails
	errMsg := "tracker API rate limit exceeded"
	payload := models.TaskPostBackPayload{
		TaskID:  task.ID,
		TaskKey: task.Key,
		Error:   &errMsg,
		OpKind:  "assign",
	}

	updatedTask, activity, pbErr := database.PostBackTask(payload)
	if pbErr == nil || pbErr.Error() != errMsg {
		t.Errorf("expected error %q, got %v", errMsg, pbErr)
	}
	if updatedTask == nil {
		t.Fatalf("expected task object returned even on failure, got nil")
	}

	if activity == nil {
		t.Fatalf("expected audit TaskActivity created on failure, got nil")
	}
	if activity.Status != string(models.ActivityStatusFailed) {
		t.Errorf("expected activity status 'failed', got %q", activity.Status)
	}
	if activity.Error != errMsg {
		t.Errorf("expected activity error %q, got %q", errMsg, activity.Error)
	}
}

func TestPostBackListenerEmissions(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: "default",
		Title:     "Listener Test Task",
		Status:    models.StatusToClarify,
	})
	if err != nil || task == nil {
		t.Fatalf("failed to create task: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	var receivedTask *models.Task
	var receivedErr error

	database.RegisterPostBackListener(func(tk *models.Task, act *models.TaskActivity, err error) {
		receivedTask = tk
		receivedErr = err
		wg.Done()
	})

	newTitle := "Emitted Title"
	_, _, _ = database.PostBackTask(models.TaskPostBackPayload{
		TaskID: task.ID,
		Title:  &newTitle,
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if receivedTask == nil || receivedTask.Title != newTitle {
			t.Errorf("listener did not receive updated task with title %q", newTitle)
		}
		if receivedErr != nil {
			t.Errorf("unexpected error in listener: %v", receivedErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for PostBackListener emission")
	}
}
