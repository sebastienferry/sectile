package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/handlers"
)

func TestLocalWorkGateWaitsForAuthenticatedAgentWithoutBlockingBootstrap(t *testing.T) {
	root := filepath.Join(t.TempDir(), "local")
	runtime, err := Open(Config{
		Database: db.SQLiteConfig(filepath.Join(root, "tasks.db")),
		DataDir:  root, StoreRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			t.Error(err)
		}
	}()
	userID, err := runtime.Database.UpsertUser("local|owner", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := runtime.Database.CreateAPIKey(userID, "local-device", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, runtime.Handler)
	server := httptest.NewServer(runtime.HTTPHandler(mux))
	defer server.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	probe := func(path string) int {
		t.Helper()
		response, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	if got := probe(handlers.HealthPath); got != http.StatusOK {
		t.Fatalf("liveness before agent connection: %d", got)
	}
	for _, path := range []string{handlers.ReadyPath, "/api/projects", "/mcp", "/api/v1/agent/pair", "/api/v1/agent/identity/extra"} {
		if got := probe(path); got != http.StatusServiceUnavailable {
			t.Fatalf("%s bypassed startup gate: %d", path, got)
		}
	}
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agent/identity", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+key)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("agent identity was blocked by startup gate: %d", response.StatusCode)
	}
	if err := runtime.ActivateWork(userID, "local-device"); err == nil {
		t.Fatal("work activated before the agent connected")
	}
	shortCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := runtime.WaitForAgent(shortCtx, userID, "local-device"); err != context.DeadlineExceeded {
		t.Fatalf("wait without agent: %v", err)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/agent-connect?projectId=all&deviceId=local-device"
	bad, badResponse, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": {"Bearer invalid"}})
	if bad != nil {
		bad.Close()
	}
	if err == nil || badResponse == nil || badResponse.StatusCode == http.StatusSwitchingProtocols {
		t.Fatal("unauthenticated agent connected")
	}
	badResponse.Body.Close()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": {"Bearer " + key}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	go func() {
		for {
			var message agentprotocol.Message
			if err := conn.ReadJSON(&message); err != nil {
				return
			}
			if message.Type == "pull_tasks" {
				_ = conn.WriteJSON(agentprotocol.Message{MsgID: message.MsgID, Type: "running_tasks", Payload: json.RawMessage(`[]`)})
			}
		}
	}()
	ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if err := runtime.WaitForAgent(ctx, userID, "local-device"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.ActivateWork("wrong-user", "local-device"); err == nil {
		t.Fatal("another user's agent activated the Local runtime")
	}
	if got := probe(handlers.ReadyPath); got != http.StatusServiceUnavailable {
		t.Fatalf("wrong agent opened readiness: %d", got)
	}
	if err := runtime.ActivateWork(userID, "local-device"); err != nil {
		t.Fatal(err)
	}
	if got := probe(handlers.ReadyPath); got != http.StatusOK {
		t.Fatalf("ready after authenticated agent connection: %d", got)
	}
	if got := probe("/api/projects"); got == http.StatusServiceUnavailable {
		t.Fatal("work route remained gated after activation")
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && probe(handlers.ReadyPath) != http.StatusServiceUnavailable {
		time.Sleep(10 * time.Millisecond)
	}
	if got := probe(handlers.ReadyPath); got != http.StatusServiceUnavailable {
		t.Fatalf("disconnected Local agent left work ready: %d", got)
	}
	runtime.Handler.BeginDrain()
	if got := probe(handlers.ReadyPath); got != http.StatusServiceUnavailable {
		t.Fatalf("draining runtime stayed ready: %d", got)
	}
}
