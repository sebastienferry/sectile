package taskmcp

import (
	"path/filepath"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

// The #499 path through the tools a session actually calls: the launcher
// records a run, the agent reports it queued, and the session it launched
// adopts it with start_run and reports it with finish_run.
func TestALauncherRunLeftQueuedIsAdoptedAndFinishedOverMCP(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Handed off", Source: "local"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartAgentRun(task.ID, "handoff", db.RunLaunch{UserID: tester.UserID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SyncRemoteRunStatusFor(tester.UserID, run.ID, task.ID, "default", task.Key, "handoff", "queued", "", nil); err != nil {
		t.Fatal(err)
	}

	started, err := call(t, database, "start_run", map[string]any{"taskKey": task.Key, "skill": "handoff", "runId": run.ID})
	if err != nil {
		t.Fatalf("start_run with the launcher runId: %v", err)
	}
	if started["id"] != run.ID || started["status"] != "running" {
		t.Fatalf("start_run returned %v %v, want %s running", started["id"], started["status"], run.ID)
	}
	finished, err := call(t, database, "finish_run", map[string]any{"taskKey": task.Key, "runId": run.ID, "status": "completed", "note": "handed off"})
	if err != nil {
		t.Fatalf("finish_run: %v", err)
	}
	if finished["status"] != "completed" {
		t.Fatalf("finish_run returned status %v, want completed", finished["status"])
	}
}
