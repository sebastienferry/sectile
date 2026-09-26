package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/db"
	"tasks/internal/models"
)

func TestExternalTerminalDispatchWithoutSkillAndFailureFeedback(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "External terminal"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(h.HandleAgentConnect))
	defer server.Close()
	agent, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?projectId=default", http.Header{"Authorization": []string{"Bearer " + agentTestKey(t, h)}})
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	// A heartbeat proves the server has registered the agent before dispatch.
	if err = agent.WriteJSON(AgentMessage{Type: "heartbeat"}); err != nil {
		t.Fatal(err)
	}
	var heartbeat AgentMessage
	if err = agent.ReadJSON(&heartbeat); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"completed", "failed"} {
		finished := make(chan error, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		go func() {
			_, err := h.launchTaskExternalTerminal(ctx, ImplicitUser, task.ID, "", "")
			finished <- err
		}()
		var message AgentMessage
		if err = agent.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err = json.Unmarshal(message.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["action"] != "open_terminal" || payload["schemaVersion"] != float64(1) || payload["skillId"] != nil || payload["terminalOverride"] != nil {
			t.Fatalf("bad terminal dispatch: %+v", payload)
		}
		select {
		case err := <-finished:
			t.Fatalf("returned before agent confirmation: %v", err)
		default:
		}
		raw, _ := json.Marshal(map[string]string{"status": status, "summary": "terminal-unavailable"})
		if err = agent.WriteJSON(AgentMessage{MsgID: message.MsgID, Type: "step_status", Payload: raw}); err != nil {
			t.Fatal(err)
		}
		err = <-finished
		cancel()
		if status == "failed" && (err == nil || !strings.Contains(err.Error(), "terminal-unavailable")) {
			t.Fatalf("failure hidden: %v", err)
		}
		if status == "completed" && err != nil {
			t.Fatal(err)
		}
	}
}

// The terminal belongs to the workstation (#305): whatever the deployment and
// the owner stored before, the server sends none and the agent resolves it.
func TestExternalTerminalSendsNoServerTerminal(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	if _, err := database.UpdateSettings(models.Settings{ExternalTerminalCommand: "iTerm"}); err != nil {
		t.Fatal(err)
	}
	owner, err := database.SignInLocal("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.UpdateUserSettings(owner.ID, models.Settings{ExternalTerminalCommand: "Ghostty"}); err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Owner terminal"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(h.HandleAgentConnect))
	defer server.Close()
	key, _, err := database.CreateAPIKey(owner.ID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	agent, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?projectId=default", http.Header{"Authorization": []string{"Bearer " + key}})
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	if err = agent.WriteJSON(AgentMessage{Type: "heartbeat"}); err != nil {
		t.Fatal(err)
	}
	var heartbeat AgentMessage
	if err = agent.ReadJSON(&heartbeat); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := h.launchTaskExternalTerminal(ctx, owner.ID, task.ID, "", "")
		finished <- err
	}()
	var message AgentMessage
	if err = agent.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(message.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if _, sent := payload["terminalOverride"]; sent {
		t.Fatalf("the server sent a terminal: %v", payload["terminalOverride"])
	}
	raw, _ := json.Marshal(map[string]string{"status": "completed", "summary": "ok"})
	if err = agent.WriteJSON(AgentMessage{MsgID: message.MsgID, Type: "step_status", Payload: raw}); err != nil {
		t.Fatal(err)
	}
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
}
