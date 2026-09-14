package db

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"testing"
)

func TestTTYRequestsRequireAgentConfirmation(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Agent console"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.StartAgentInTTY(task.ID, false); err == nil {
		t.Fatal("server accepted local console without an agent")
	}
	var received agentprotocol.Operation
	d.SetAgentOperations(func(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		received = op
		return nil, fmt.Errorf("disconnected")
	})
	if _, err = d.InjectSkillInTTY(task.ID, "clarify"); err == nil {
		t.Fatal("failed dispatch reported success")
	}
	d.SetAgentOperations(func(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		received = op
		return json.RawMessage("null"), nil
	})
	if _, err = d.InjectSkillInTTY(task.ID, "clarify"); err != nil {
		t.Fatal(err)
	}
	if received.TaskID != task.ID || received.SkillID != "clarify" || received.Action != "execute_skill" {
		t.Fatalf("wrong dispatch: %+v", received)
	}
	fresh, _ := d.GetTaskByID(task.ID)
	if d.StageOfTask(fresh) != "new" {
		t.Fatal("launch advanced workflow")
	}
}
