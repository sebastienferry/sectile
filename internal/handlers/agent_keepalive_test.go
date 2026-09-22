package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/db"

	"github.com/gorilla/websocket"
)

func connectAgent(t *testing.T, h *Handler) (*websocket.Conn, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(h.HandleAgentConnect))
	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"
	u.RawQuery = "token=" + agentTestKey(t, h) + "&deviceId=test-laptop&projectId=default"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial error: %v", err)
	}
	return conn, func() { _ = conn.Close(); server.Close() }
}

// newAgentHandler builds a handler whose ping cycle is short enough for a test
// to observe a dropped connection in milliseconds rather than half a minute.
func newAgentHandler(t *testing.T, ping, read time.Duration) *Handler {
	t.Helper()
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db error: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	h := NewHandler(database)
	h.agentPingInterval, h.agentReadTimeout = ping, read
	return h
}

// waitForAgent polls the dispatcher until the slot reaches the wanted state,
// so the test does not depend on how quickly the read loop unwinds.
func waitForAgent(t *testing.T, h *Handler, want bool) bool {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if (h.agentDispatcher.Lookup("default", "default") != nil) == want {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// A peer that stops reading — a sleeping laptop, a dropped VPN — sends neither
// an error nor a close frame. Without the read deadline the slot stayed
// registered and every operation routed to it burned its full timeout.
func TestAgentKeepalive_UnregistersSilentAgent(t *testing.T) {
	h := newAgentHandler(t, 20*time.Millisecond, 100*time.Millisecond)
	_, cleanup := connectAgent(t, h)
	defer cleanup()

	if !waitForAgent(t, h, true) {
		t.Fatal("agent never registered")
	}
	// The client never calls ReadMessage, so gorilla never answers the pings.
	if !waitForAgent(t, h, false) {
		t.Fatal("silent agent still registered after the read timeout elapsed")
	}
}

// The mirror case: an agent that keeps reading answers the pings automatically
// and must survive well past the read timeout.
func TestAgentKeepalive_KeepsResponsiveAgentRegistered(t *testing.T) {
	h := newAgentHandler(t, 20*time.Millisecond, 100*time.Millisecond)
	conn, cleanup := connectAgent(t, h)
	defer cleanup()

	if !waitForAgent(t, h, true) {
		t.Fatal("agent never registered")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	time.Sleep(400 * time.Millisecond) // several ping cycles
	if h.agentDispatcher.Lookup("default", "default") == nil {
		t.Fatal("responsive agent was unregistered by the keepalive")
	}
	ac := h.agentDispatcher.Lookup("default", "default")
	if since := time.Since(ac.LastSeen()); since > 200*time.Millisecond {
		t.Errorf("pongs did not refresh lastSeen: %s since last frame", since)
	}
	cleanup()
	<-done
}
