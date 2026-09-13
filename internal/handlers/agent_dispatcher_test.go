package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"

	"github.com/gorilla/websocket"
)

func TestAgentDispatcher_RegisterAndLookup(t *testing.T) {
	dispatcher := handlers.NewAgentDispatcher()

	// Initial lookup should return nil
	if ac := dispatcher.Lookup("user1", "proj1"); ac != nil {
		t.Fatalf("expected nil agent, got %v", ac)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn.Close()

	ac := dispatcher.Register("user1", "proj1", "laptop", conn)
	if ac == nil {
		t.Fatalf("expected non-nil AgentConn")
	}

	if dispatcher.Lookup("default", "proj1") != nil {
		t.Fatal("default user matched another user agent")
	}
	if dispatcher.Lookup("user1", "other-project") != nil {
		t.Fatal("scoped agent received another project")
	}
	found := dispatcher.Lookup("user1", "proj1")
	if found == nil || found.DeviceID != "laptop" {
		t.Errorf("expected to find agent with device laptop, got %v", found)
	}

	agents := dispatcher.ConnectedAgents()
	if len(agents) != 1 || agents[0].UserID != "user1" || agents[0].ProjectID != "proj1" {
		t.Errorf("expected 1 agent for user1/proj1, got %v", agents)
	}

	// Unregister
	dispatcher.Unregister("user1", "proj1", conn)
	if ac := dispatcher.Lookup("user1", "proj1"); ac != nil {
		t.Errorf("expected agent to be unregistered, got %v", ac)
	}
}

func TestAgentDispatcher_SessionRebound(t *testing.T) {
	dispatcher := handlers.NewAgentDispatcher()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// keep connection alive
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"

	conn1, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn2.Close()

	// Register device 1
	dispatcher.Register("user1", "proj1", "device1", conn1)

	// Register device 2 on same user/project -> should rebind and close device 1 with 4001
	dispatcher.Register("user1", "proj1", "device2", conn2)

	active := dispatcher.Lookup("user1", "proj1")
	if active == nil || active.DeviceID != "device2" {
		t.Errorf("expected active agent to be device2, got %v", active)
	}

	// Verify conn1 received close message or was closed
	_ = conn1.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, _, err = conn1.ReadMessage()
	if err == nil {
		t.Errorf("expected error or close on rebounded conn1")
	}
}

func TestHandleAgentStatus(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("db error: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	req := httptest.NewRequest(http.MethodGet, "/api/agent/status", nil)
	rr := httptest.NewRecorder()
	h.HandleAgentStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var res struct {
		Agents []handlers.AgentConnInfo `json:"agents"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(res.Agents) != 0 {
		t.Errorf("expected 0 agents initially, got %d", len(res.Agents))
	}
}

func TestHandleAgentDispatch_DisconnectedGuard(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("db error: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	// Attempt dispatch when no agent is connected -> expect 428 Precondition Required
	body := `{"userId":"default","projectId":"default","taskId":"#46","action":"clarify"}`
	req := httptest.NewRequest(http.MethodPost, "/api/agent/dispatch", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleAgentDispatch(rr, req)

	if rr.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected status 428 Precondition Required, got %d: %s", rr.Code, rr.Body.String())
	}

	var errRes map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &errRes)
	if !strings.Contains(errRes["error"], "No local agent connected") {
		t.Errorf("expected error message to indicate disconnected agent, got: %v", errRes)
	}
}

func TestHandleAgentConnect_WebSocketHandshake(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("db error: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	server := httptest.NewServer(http.HandlerFunc(h.HandleAgentConnect))
	defer server.Close()

	// 1. Missing token -> should fail with 401 Unauthorized
	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"
	_, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for missing token, got status: %v", resp)
	}

	// 2. Valid token -> successful upgrade and registration
	u.RawQuery = "token=test-secret&deviceId=test-macbook&projectId=default"
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("expected successful connection with token, got error: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101 Switching Protocols, got %d", resp.StatusCode)
	}

	// Check status API now reports the connected agent
	statusReq := httptest.NewRequest(http.MethodGet, "/api/agent/status", nil)
	statusRR := httptest.NewRecorder()
	h.HandleAgentStatus(statusRR, statusReq)

	var statusRes struct {
		Agents []handlers.AgentConnInfo `json:"agents"`
	}
	_ = json.Unmarshal(statusRR.Body.Bytes(), &statusRes)
	if len(statusRes.Agents) != 1 {
		t.Fatalf("expected 1 connected agent, got %d", len(statusRes.Agents))
	}
	if statusRes.Agents[0].DeviceID != "test-macbook" {
		t.Errorf("expected deviceId test-macbook, got %s", statusRes.Agents[0].DeviceID)
	}

	// 3. Dispatch command to connected agent -> should succeed and receive message over WS
	dispatchBody := `{"userId":"default","projectId":"default","taskId":"#46","action":"dispatch_step","payload":{"taskKey":"#46","action":"clarify"}}`
	dispatchReq := httptest.NewRequest(http.MethodPost, "/api/agent/dispatch", strings.NewReader(dispatchBody))
	dispatchRR := httptest.NewRecorder()
	h.HandleAgentDispatch(dispatchRR, dispatchReq)

	if dispatchRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from dispatch, got %d: %s", dispatchRR.Code, dispatchRR.Body.String())
	}

	// Verify the agent WebSocket received the dispatched message
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read dispatched message on agent conn: %v", err)
	}

	var agentMsg handlers.AgentMessage
	if err := json.Unmarshal(msgBytes, &agentMsg); err != nil {
		t.Fatalf("failed to unmarshal agent message: %v", err)
	}

	if agentMsg.Type != "dispatch_step" || agentMsg.TaskID != "#46" {
		t.Errorf("expected dispatch_step for task #46, got: %+v", agentMsg)
	}
}

func TestHandleTaskDetail_RunSkill_DispatchesToAgent(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("db error: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)

	task, err := database.CreateTask(models.CreateTaskRequest{
		Title:       "Test task",
		Description: "Testing agent dispatch",
		ProjectID:   "custom-proj-uuid-123",
		Priority:    "high",
	})
	if err != nil {
		t.Fatalf("create task error: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws/agent-connect" {
			h.HandleAgentConnect(w, r)
			return
		}
		h.HandleTaskDetail(w, r)
	}))
	defer server.Close()

	// Connect agent daemon WebSocket
	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"
	u.Path = "/ws/agent-connect"
	u.RawQuery = "token=secret&deviceId=my-laptop&projectId=default"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("agent dial error: %v", err)
	}
	defer conn.Close()

	// Trigger run-skill via HTTP POST /api/tasks/:id/run-skill
	runSkillBody := `{"skillId":"clarify"}`
	runSkillReq, _ := http.NewRequest(http.MethodPost, server.URL+"/api/tasks/"+task.ID+"/run-skill", strings.NewReader(runSkillBody))
	runSkillReq.Header.Set("Content-Type", "application/json")
	responses := make(chan *http.Response, 1)
	requestErrors := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(runSkillReq)
		if err != nil {
			requestErrors <- err
			return
		}
		responses <- resp
	}()

	// Verify the agent received dispatch_step message
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("agent did not receive dispatched step: %v", err)
	}

	var agentMsg handlers.AgentMessage
	if err := json.Unmarshal(msgBytes, &agentMsg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{"status": "completed", "summary": "Native CLI opened"})
	if err := conn.WriteJSON(handlers.AgentMessage{MsgID: agentMsg.MsgID, TaskID: task.ID, Type: "step_status", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case resp := <-responses:
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("run-skill status %d", resp.StatusCode)
		}
	case err := <-requestErrors:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("launch acknowledgement not handled")
	}

	var dispatch struct {
		RunID string `json:"runId"`
	}
	if err := json.Unmarshal(agentMsg.Payload, &dispatch); err != nil || dispatch.RunID == "" {
		t.Fatalf("missing remote run identity: %s %v", agentMsg.Payload, err)
	}
	active, err := database.GetActivityByID(dispatch.RunID)
	if err != nil || active.Status != "running" || active.SkillID != "remote_run" {
		t.Fatalf("launch acknowledgement finished execution: %+v %v", active, err)
	}

	if agentMsg.Type != "dispatch_step" {
		t.Errorf("expected msg type dispatch_step, got %s", agentMsg.Type)
	}
	if agentMsg.TaskID != task.ID {
		t.Errorf("expected taskID %s, got %s", task.ID, agentMsg.TaskID)
	}
	cancelBody := strings.NewReader(`{"runId":"` + dispatch.RunID + `"}`)
	cancelRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/api/tasks/"+task.ID+"/cancel-run", cancelBody)
	cancelRequest.Header.Set("Content-Type", "application/json")
	go func() {
		resp, err := http.DefaultClient.Do(cancelRequest)
		if err != nil {
			requestErrors <- err
			return
		}
		responses <- resp
	}()
	var stopMessage handlers.AgentMessage
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.ReadJSON(&stopMessage); err != nil {
		t.Fatal(err)
	}
	var stopPayload struct {
		Action string `json:"action"`
		RunID  string `json:"runId"`
	}
	if err := json.Unmarshal(stopMessage.Payload, &stopPayload); err != nil || stopPayload.Action != "cancel_run" || stopPayload.RunID != dispatch.RunID {
		t.Fatalf("bad stop payload: %s %v", stopMessage.Payload, err)
	}
	active, err = database.GetActivityByID(dispatch.RunID)
	if err != nil || active.Status != "running" {
		t.Fatal("run canceled before exit acknowledgement")
	}
	if err := conn.WriteJSON(handlers.AgentMessage{MsgID: stopMessage.MsgID, TaskID: task.ID, Type: "step_status", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case resp := <-responses:
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.StatusCode)
		}
	case err := <-requestErrors:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("cancel acknowledgement not handled")
	}
	active, err = database.GetActivityByID(dispatch.RunID)
	if err != nil || active.Status != "canceled" {
		t.Fatalf("run not canceled: %+v %v", active, err)
	}

}
