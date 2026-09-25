package handlers

import (
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/models"
)

// An event another instance published reaches this instance's browsers as the
// event they would have received locally, with the task as it now stands.
func TestARelayedEventReachesLocalBrowsers(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := NewHandler(database)
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Changed elsewhere", Source: "local"})
	if err != nil {
		t.Fatal(err)
	}

	ch := h.SubscribeEvents()
	defer h.UnsubscribeEvents(ch)
	h.deliverRelayedEvent(db.BusMessage{Origin: "other", Kind: "event", Type: "task_updated", TaskID: task.ID, ActivityID: "gone", Error: "boom"})

	select {
	case event := <-ch:
		if event.Type != "task_updated" || event.Task == nil || event.Task.Title != "Changed elsewhere" || event.Activity != nil || event.Error != "boom" {
			t.Fatalf("delivered event = %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the relayed event was not delivered")
	}
}

// Terminal output stays on the instance that received it.
func TestTerminalOutputIsNotRelayed(t *testing.T) {
	if !localOnlyEvents["agent_pty_output"] {
		t.Fatal("agent_pty_output must stay local")
	}
	if localOnlyEvents["task_updated"] {
		t.Fatal("task_updated must be relayed")
	}
}
