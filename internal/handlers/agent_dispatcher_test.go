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
