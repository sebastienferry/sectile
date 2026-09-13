package main

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
)

func TestDesktopFinishesCanonicalRun(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Desktop completion"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartRemoteRun(task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASKFLOW_SERVER_TOKEN", "desktop-secret")
	server := httptest.NewServer(handlers.NewHandler(database).MCPHandler())
	defer server.Close()
	daemon := &agentDaemon{serverURL: server.URL, token: "desktop-secret"}
	if err := daemon.finishDesktopRun(context.Background(), task.ID, run.ID, "completed"); err != nil {
		t.Fatal(err)
	}
	finished, err := database.GetActivityByID(run.ID)
	if err != nil || finished.Status != "completed" {
		t.Fatalf("original run not completed: %v %v", finished, err)
	}
	activities, err := database.GetTaskActivities(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, activity := range activities {
		if activity.SkillID == "remote_run" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly the original remote run, got %d", count)
	}
	unchanged, err := database.GetTaskByID(task.ID)
	if err != nil || unchanged.Status != task.Status {
		t.Fatal("completion advanced task stage")
	}
}
