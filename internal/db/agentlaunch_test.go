package db

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"testing"
	"time"
)

func TestQueuedUnconfirmedLaunchKeepsRemoteRunOpen(t *testing.T) {
	d, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Unconfirmed launch"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddTaskActivity(models.TaskActivity{ID: "launch", TaskID: task.ID, SkillID: "clarify", Status: "queued", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	var runID string
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		runID = op.RunID
		return nil, fmt.Errorf("%w: lost acknowledgment", agentprotocol.ErrLaunchUnconfirmed)
	})
	d.processSkillJob(SkillJob{ActivityID: "launch", TaskID: task.ID, SkillID: "clarify"})
	run, err := d.GetActivityByID(runID)
	if err != nil || run == nil || run.Status != "running" {
		t.Fatalf("unconfirmed launch closed remote run: %+v, %v", run, err)
	}
	// The agent remains responsible for the terminal outcome.
	if _, err := d.FinishRemoteRun(task.ID, runID, "completed", "Agent reported completion"); err != nil {
		t.Fatal(err)
	}
}

func TestLocalLaunchDoesNotOwnStageTransition(t *testing.T) {
	d, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
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
