package db

import (
	"path/filepath"
	"tasks/internal/models"
	"testing"
)

func TestStrictRemoteCreationRejectsUnsupportedTracker(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.CreateTask(models.CreateTaskRequest{Title: "Do not silently create locally", Source: "jira", RequireRemoteCreation: true})
	if err == nil {
		t.Fatal("unsupported remote creation must fail")
	}
	var count int
	if err = database.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE title = ?", "Do not silently create locally").Scan(&count); err != nil || count != 0 {
		t.Fatal("task created despite remote failure", count, err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Local task", Source: "local", RequireRemoteCreation: true})
	if err != nil || task == nil {
		t.Fatal("local projects must remain supported", err)
	}
}
