package db

import (
	"path/filepath"
	"tasks/internal/models"
	"testing"
	"time"
)

func TestLocalLaunchDoesNotOwnStageTransition(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Native launch"})
	if err != nil {
		t.Fatal(err)
	}
	for _, act := range []models.TaskActivity{
		// Migration 9 marks such a leftover concurrent, as it sits next to the
		// managed run below.
		{ID: "legacy", TaskID: task.ID, SkillID: "clarify", Action: "Exécution de clarify sur l'agent local", Status: "running", CreatedAt: time.Now(), Concurrent: true},
		{ID: "new", TaskID: task.ID, SkillID: "agent_launch", Status: "running", CreatedAt: time.Now()},
	} {
		if err := d.AddTaskActivity(act); err != nil {
			t.Fatal(err)
		}
	}
	d.mu.RLock()
	running, err := d.managedStageRunningUnsafe(task.ID)
	d.mu.RUnlock()
	if err != nil || running {
		t.Fatalf("native launch blocked transition: %v %v", running, err)
	}
	if err := d.AddTaskActivity(models.TaskActivity{ID: "managed", TaskID: task.ID, SkillID: "clarify", Action: "Managed clarification", Status: "running", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "clarified", "report", "", ""); err == nil {
		t.Fatal("real managed execution must still block")
	}
}
