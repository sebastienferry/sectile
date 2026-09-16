package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/models"
)

func setupTestHandler(t *testing.T) (*Handler, *db.DB, func()) {
	tmpDir, err := os.MkdirTemp("", "sectile-handler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.NewDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create test db: %v", err)
	}

	h := NewHandler(database)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return h, database, cleanup
}

func TestHandleTaskPostBackHTTP(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()

	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: "default",
		Title:     "HTTP PostBack Task",
		Status:    models.StatusToClarify,
	})
	if err != nil || task == nil {
		t.Fatalf("failed to create task: %v", err)
	}

	updatedTitle := "Updated Title via HTTP"
	updatedAssignee := "Bob Handler"
	payload := models.TaskPostBackPayload{
		TaskID:   task.ID,
		Title:    &updatedTitle,
		Assignee: &updatedAssignee,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/postback", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleTaskPostBack(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var res models.TaskPostBackResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success true, got false. Error: %s", res.Error)
	}
	if res.Task == nil || res.Task.Title != updatedTitle || res.Task.Assignee != updatedAssignee {
		t.Errorf("unexpected task in response: %+v", res.Task)
	}
}

func TestEventBroadcasting(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()

	ch := h.SubscribeEvents()
	defer h.UnsubscribeEvents(ch)

	testTask := &models.Task{
		ID:    "task-test-1",
		Key:   "TEST-1",
		Title: "Broadcast Task",
	}

	h.BroadcastEvent(Event{
		Type: "task_updated",
		Task: testTask,
	})

	select {
	case evt := <-ch:
		if evt.Type != "task_updated" || evt.Task == nil || evt.Task.ID != "task-test-1" {
			t.Errorf("unexpected event received: %+v", evt)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for broadcast event")
	}
}
