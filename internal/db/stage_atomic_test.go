package db

import (
	"path/filepath"
	"tasks/internal/models"
	"testing"
)

func TestStageRollsBackWhenActivityCannotBeRecorded(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Atomic stage", ProjectID: "default", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.conn.Exec(`CREATE TRIGGER reject_stage_report BEFORE INSERT ON task_activities BEGIN SELECT RAISE(ABORT, 'report rejected'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := database.TransitionTaskStage(task.ID, "clarified", "verified", "", "feat/atomic"); err == nil {
		t.Fatal("activity failure was hidden")
	}
	after, err := database.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != task.Status || after.BranchName != nil {
		t.Fatalf("failed transition changed task: %+v", after)
	}
}
