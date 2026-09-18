package db

import (
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

func TestPinnedLabelHelpers(t *testing.T) {
	if !HasPinnedLabel([]string{"pinned"}) {
		t.Errorf("expected HasPinnedLabel to match 'pinned'")
	}
	if !HasPinnedLabel([]string{"Pinned"}) {
		t.Errorf("expected HasPinnedLabel to match 'Pinned'")
	}
	if !HasPinnedLabel([]string{"#pinned"}) {
		t.Errorf("expected HasPinnedLabel to match '#pinned'")
	}
	if !HasPinnedLabel([]string{"#Pinned"}) {
		t.Errorf("expected HasPinnedLabel to match '#Pinned'")
	}
	if HasPinnedLabel([]string{"unrelated", "feature"}) {
		t.Errorf("expected HasPinnedLabel to return false for unrelated labels")
	}

	added := AddPinnedLabel([]string{"feature"})
	if len(added) != 2 || !HasPinnedLabel(added) {
		t.Errorf("expected AddPinnedLabel to add 'pinned', got %v", added)
	}
	addedTwice := AddPinnedLabel(added)
	if len(addedTwice) != 2 {
		t.Errorf("expected AddPinnedLabel not to duplicate label, got %v", addedTwice)
	}

	removed := RemovePinnedLabel([]string{"feature", "pinned", "#Pinned"})
	if len(removed) != 1 || removed[0] != "feature" {
		t.Errorf("expected RemovePinnedLabel to strip all pinned labels, got %v", removed)
	}
}

func TestSetTaskPinnedAndToggle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	task, err := database.CreateTask(models.CreateTaskRequest{
		Title:    "Test Pinned Task",
		Status:   models.StatusToClarify,
		Priority: models.PriorityMedium,
		Labels:   []string{"frontend"},
		Source:   "local",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if task.Pinned {
		t.Errorf("new task should not be pinned")
	}

	// 1. Pin the task
	if err := database.SetTaskPinned(task.ID, true); err != nil {
		t.Fatalf("SetTaskPinned(true) failed: %v", err)
	}

	updated, err := database.GetTaskByID(task.ID)
	if err != nil || updated == nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if !updated.Pinned {
		t.Errorf("expected task.Pinned=true")
	}
	if !HasPinnedLabel(updated.Labels) {
		t.Errorf("expected task.Labels to contain 'pinned', got %v", updated.Labels)
	}

	// Check DB pinned flag
	var dbPinned int
	_ = database.conn.QueryRow("SELECT pinned FROM tasks WHERE id = ?", task.ID).Scan(&dbPinned)
	if dbPinned != 1 {
		t.Errorf("expected tasks.pinned = 1 in db, got %d", dbPinned)
	}

	// Check PinnedTasks returns it
	pinnedList, err := database.PinnedTasks()
	if err != nil {
		t.Fatalf("PinnedTasks failed: %v", err)
	}
	if len(pinnedList) != 1 || pinnedList[0].ID != task.ID {
		t.Errorf("expected PinnedTasks to return the task, got %v", pinnedList)
	}

	// 2. Toggle pin (should unpin)
	newPinned, err := database.ToggleTaskPinned(task.ID)
	if err != nil {
		t.Fatalf("ToggleTaskPinned failed: %v", err)
	}
	if newPinned {
		t.Errorf("expected ToggleTaskPinned to return false when unpinning")
	}

	updated, _ = database.GetTaskByID(task.ID)
	if updated.Pinned {
		t.Errorf("expected task.Pinned=false after toggle")
	}
	if HasPinnedLabel(updated.Labels) {
		t.Errorf("expected task.Labels not to contain 'pinned', got %v", updated.Labels)
	}

	// Check PinnedTasks is now empty
	pinnedList, _ = database.PinnedTasks()
	if len(pinnedList) != 0 {
		t.Errorf("expected PinnedTasks to be empty after unpinning, got %v", pinnedList)
	}

	// 3. Toggle pin again (should pin)
	newPinned, err = database.ToggleTaskPinned(task.ID)
	if err != nil {
		t.Fatalf("ToggleTaskPinned 2 failed: %v", err)
	}
	if !newPinned {
		t.Errorf("expected ToggleTaskPinned to return true when pinning")
	}

	pinnedList, _ = database.PinnedTasks()
	if len(pinnedList) != 1 {
		t.Errorf("expected PinnedTasks to contain 1 task, got %v", pinnedList)
	}
}

func TestImportOrUpdateTasksPinnedSync(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	now := time.Now()
	syncedTasks := []models.Task{
		{
			ID:        "sync-task-1",
			Key:       "JIRA-101",
			Title:     "Remote Synced Task",
			Status:    models.StatusToClarify,
			Priority:  models.PriorityHigh,
			Labels:    []string{"pinned", "backend"},
			Source:    "jira",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// Import with 'pinned' label
	if err := database.ImportOrUpdateTasks(syncedTasks); err != nil {
		t.Fatalf("ImportOrUpdateTasks failed: %v", err)
	}

	// A synced work item takes the identity its tracker gives it, whatever the
	// importer proposed: jira-<KEY> here, the default project being nameless.
	jira, _ := database.TrackerRegistry().Get("jira")
	taskID := jira.FormatTaskID("default", "JIRA-101", "sync-task-1")
	if taskID != "jira-JIRA-101" {
		t.Fatalf("unexpected canonical identity %q", taskID)
	}
	task, err := database.GetTaskByID(taskID)
	if err != nil || task == nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if !task.Pinned {
		t.Errorf("expected imported task with 'pinned' label to be Pinned=true")
	}

	pinnedList, _ := database.PinnedTasks()
	if len(pinnedList) != 1 || pinnedList[0].Key != "JIRA-101" {
		t.Errorf("expected PinnedTasks to include synced task, got %v", pinnedList)
	}

	// Re-import without 'pinned' label
	syncedTasks[0].Labels = []string{"backend"}
	if err := database.ImportOrUpdateTasks(syncedTasks); err != nil {
		t.Fatalf("ImportOrUpdateTasks (update) failed: %v", err)
	}

	task, _ = database.GetTaskByID(taskID)
	if task.Pinned {
		t.Errorf("expected task to no longer be Pinned after remote unpin")
	}

	pinnedList, _ = database.PinnedTasks()
	if len(pinnedList) != 0 {
		t.Errorf("expected PinnedTasks to be empty after remote unpin, got %v", pinnedList)
	}
}
