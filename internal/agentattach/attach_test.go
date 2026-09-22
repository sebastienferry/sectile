package agentattach

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/testhome"
	"testing"

	"github.com/gorilla/websocket"
)

func TestResolveConnectionExplicit(t *testing.T) {
	info, err := ResolveConnection("session-123", "http://127.0.0.1:8090", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.SessionID != "session-123" {
		t.Errorf("expected session-123, got %s", info.SessionID)
	}
	if info.URL != "http://127.0.0.1:8090" {
		t.Errorf("expected http://127.0.0.1:8090, got %s", info.URL)
	}
	if info.Token != "test-token" {
		t.Errorf("expected test-token, got %s", info.Token)
	}
}

func TestResolveConnectionMissingSession(t *testing.T) {
	_, err := ResolveConnection("", "http://127.0.0.1:8090", "token")
	if err == nil {
		t.Fatal("expected error when session ID is missing")
	}
	if !strings.Contains(err.Error(), "session ID is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestResolveConnectionEnvVars(t *testing.T) {
	t.Setenv("SECTILE_LOOPBACK_URL", "http://127.0.0.1:9999")
	t.Setenv("SECTILE_AGENT_TOKEN", "env-token")

	info, err := ResolveConnection("run-abc", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.SessionID != "run-abc" || info.URL != "http://127.0.0.1:9999" || info.Token != "env-token" {
		t.Fatalf("unexpected info resolved from env: %+v", info)
	}
}

func TestResolveConnectionFileFallback(t *testing.T) {
	home := t.TempDir()
	testhome.Set(t, home)
	t.Setenv("SECTILE_LOOPBACK_URL", "")
	t.Setenv("SECTILE_AGENT_TOKEN", "")

	taskflowDir := filepath.Join(home, ".taskflow")
	if err := os.MkdirAll(taskflowDir, 0700); err != nil {
		t.Fatal(err)
	}

	rawConn, _ := json.Marshal(map[string]string{
		"url":   "http://127.0.0.1:8888",
		"token": "file-token",
	})
	if err := os.WriteFile(filepath.Join(taskflowDir, "agent-connection.json"), rawConn, 0600); err != nil {
		t.Fatal(err)
	}

	info, err := ResolveConnection("sess-file", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.URL != "http://127.0.0.1:8888" || info.Token != "file-token" {
		t.Fatalf("unexpected connection info from file: %+v", info)
	}
}

func TestFormatWebSocketURL(t *testing.T) {
	tests := []struct {
		raw      string
		session  string
		expected string
	}{
		{
			raw:      "http://127.0.0.1:8090",
			session:  "run-1",
			expected: "ws://127.0.0.1:8090/desktop/terminal?id=run-1",
		},
		{
			raw:      "https://remote.example.com",
			session:  "run-2",
			expected: "wss://remote.example.com/desktop/terminal?id=run-2",
		},
		{
			raw:      "ws://127.0.0.1:8090/desktop/terminal",
			session:  "run-3",
			expected: "ws://127.0.0.1:8090/desktop/terminal?id=run-3",
		},
	}

	for _, tt := range tests {
		got, err := FormatWebSocketURL(tt.raw, tt.session)
		if err != nil {
			t.Fatalf("FormatWebSocketURL error for %s: %v", tt.raw, err)
		}
		if got != tt.expected {
			t.Errorf("FormatWebSocketURL(%q, %q) = %q, want %q", tt.raw, tt.session, got, tt.expected)
		}
	}
}

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestAttachWebSocketExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/desktop/terminal" || r.URL.Query().Get("id") != "test-sess" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send greeting banner
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte("PTY Connected\n"))

		// Read one message from client
		msgType, msg, err := conn.ReadMessage()
		if err == nil {
			// Echo it back
			_ = conn.WriteMessage(msgType, append([]byte("echo: "), msg...))
		}
	}))
	defer server.Close()

	wsURL, err := FormatWebSocketURL(server.URL, "test-sess")
	if err != nil {
		t.Fatal(err)
	}

	header := http.Header{"Authorization": []string{"Bearer secret-123"}}
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()
	defer resp.Body.Close()

	// Verify server greeting
	_, greeting, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading greeting: %v", err)
	}
	if string(greeting) != "PTY Connected\n" {
		t.Fatalf("unexpected greeting: %q", string(greeting))
	}

	// Send test input
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("hello")); err != nil {
		t.Fatalf("failed writing message: %v", err)
	}

	// Read echoed response
	_, echoed, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading echo: %v", err)
	}
	if string(echoed) != "echo: hello" {
		t.Fatalf("unexpected echo: %q", string(echoed))
	}
}
