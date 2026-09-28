package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
)

func waitForDisconnectedAgent(t *testing.T, h *Handler, userID string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for h.agentDispatcher.Route(userID, "default") != nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if h.agentDispatcher.Route(userID, "default") != nil {
		t.Fatal("agent did not unregister")
	}
}

func TestSkillLaunchWaitsForReconnectingAgent(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := batchLaunchServer(t, h)
	userID, cookie := account(t, database, "reconnect@example.com")
	key, _, err := database.CreateAPIKey(userID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	task := guardTask(t, database, "Reconnect launch")
	dropped := connectAgentAs(t, server, key, "default")
	_ = dropped.Close()
	waitForDisconnectedAgent(t, h, userID)

	type response struct {
		status int
		body   string
	}
	answers := make(chan response, 1)
	go func() {
		status, body := call(t, server, cookie, http.MethodPost, "/api/tasks/"+task.ID+"/run-skill", `{"skillId":"clarify"}`)
		answers <- response{status, body}
	}()
	time.Sleep(150 * time.Millisecond)
	select {
	case answer := <-answers:
		t.Fatalf("launch failed before reconnection: %d %s", answer.status, answer.body)
	default:
	}
	conn := connectAgentAs(t, server, key, "default")
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var dispatch AgentMessage
	if err := conn.ReadJSON(&dispatch); err != nil {
		t.Fatalf("recovered agent received no launch: %v", err)
	}
	payload, _ := json.Marshal(agentLaunchStatus{Status: "completed"})
	if err := conn.WriteJSON(AgentMessage{MsgID: dispatch.MsgID, TaskID: task.ID, Type: "step_status", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case answer := <-answers:
		if answer.status != http.StatusOK {
			t.Fatalf("launch after reconnection: %d %s", answer.status, answer.body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("launch was not answered")
	}
}

func TestSkillLaunchReconnectTimeoutRecordsNothing(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := batchLaunchServer(t, h)
	userID, cookie := account(t, database, "timeout@example.com")
	key, _, err := database.CreateAPIKey(userID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	task := guardTask(t, database, "Timeout launch")
	dropped := connectAgentAs(t, server, key, "default")
	_ = dropped.Close()
	waitForDisconnectedAgent(t, h, userID)

	status, body := call(t, server, cookie, http.MethodPost, "/api/tasks/"+task.ID+"/run-skill", `{"skillId":"clarify"}`)
	if status != http.StatusConflict || !strings.Contains(body, "se reconnecte") {
		t.Fatalf("missing reconnect guidance: %d %s", status, body)
	}
	if count := activityCount(t, database, task.ID); count != 0 {
		t.Fatalf("expired launch recorded %d activities", count)
	}
}

func TestCanceledSkillLaunchWaitRecordsNothing(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := batchLaunchServer(t, h)
	userID, cookie := account(t, database, "cancel@example.com")
	key, _, err := database.CreateAPIKey(userID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	task := guardTask(t, database, "Canceled reconnect launch")
	dropped := connectAgentAs(t, server, key, "default")
	_ = dropped.Close()
	waitForDisconnectedAgent(t, h, userID)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/run-skill", strings.NewReader(`{"skillId":"clarify"}`)).WithContext(ctx)
	req.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.HandleTaskDetail(recorder, req)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled launch kept waiting for the full reconnection grace")
	}
	if count := activityCount(t, database, task.ID); count != 0 {
		t.Fatalf("canceled launch recorded %d activities", count)
	}
}
