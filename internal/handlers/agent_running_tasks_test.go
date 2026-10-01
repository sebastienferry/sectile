package handlers

import (
	"testing"
	"time"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
)

// The agent keeps listing a run until its console exits, after the skill
// finished it, and every listing goes through ApplyAgentRunningTasksFor. That
// is the path that wrote "Execution running on agent" over the finished runs of
// #393; a finished launcher run must come out of it unchanged.
func TestAgentRunningListDoesNotReopenAFinishedLauncherRun(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Clarified", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	launched, err := database.StartAgentRun(task.ID, "clarify-issue", db.RunLaunch{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.StartRemoteRunBy("", task.ID, "clarify-issue", launched.ID); err != nil {
		t.Fatalf("adopting the launcher run: %v", err)
	}
	finished, err := database.FinishRemoteRunAs(db.Actor{}, false, task.ID, launched.ID, "completed", "Clarification done")
	if err != nil {
		t.Fatal(err)
	}

	for _, status := range []string{"running", "queued"} {
		h.ApplyAgentRunningTasksFor("", []agentprotocol.RunningTask{{
			ID: launched.ID, TaskID: task.ID, TaskKey: task.Key, ProjectID: task.ProjectID,
			Skill: "clarify-issue", Status: status, StartedAt: time.Now().Add(-time.Hour),
		}})
		got, err := database.GetActivityByID(launched.ID)
		if err != nil || got == nil {
			t.Fatalf("reading the run back: %v", err)
		}
		if got.Status != "completed" || got.CompletedAt == nil || got.Summary != finished.Summary {
			t.Errorf("after an agent listing %s, run = %s / %v / %q, want completed / %v / %q",
				status, got.Status, got.CompletedAt, got.Summary, finished.CompletedAt, finished.Summary)
		}
	}
}
