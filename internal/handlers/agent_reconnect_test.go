package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"tasks/internal/agentprotocol"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// An agent that drops re-registers within seconds. An operation issued in that
// window used to fail outright, which is what broke the stage transition on
// #123.
func TestOperationWaitsForAReconnectingAgent(t *testing.T) {
	d := NewAgentDispatcher()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := d.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "git_evidence"})
		done <- err
	}()

	// The agent registers only after the call has already started waiting.
	time.Sleep(200 * time.Millisecond)
	_, conn := operationConnection(t, d)

	var request AgentMessage
	if err := conn.ReadJSON(&request); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(AgentMessage{MsgID: request.MsgID, Type: "workspace_result", Payload: []byte(`{"value":{"branch":"local"}}`)}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("operation failed although the agent reconnected: %v", err)
	}
}

func TestAbsentAgentErrorNamesTheRetry(t *testing.T) {
	d := NewAgentDispatcher()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	started := time.Now()
	_, err := d.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "git_evidence"})
	if err == nil {
		t.Fatal("expected a failure with no agent connected")
	}
	if !strings.Contains(err.Error(), "reconnecting") || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("error does not name the reconnection and the retry: %v", err)
	}
	if waited := time.Since(started); waited < defaultAgentReconnectGrace {
		t.Fatalf("gave up after %s, before the grace period", waited)
	}
}

// The wait must never outlive the deadline the caller already chose: a local
// inspection gets 15s in total and the operation itself needs most of it.
func TestWaitForAgentStopsOnTheCallerDeadline(t *testing.T) {
	d := NewAgentDispatcher()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	if ac := d.WaitForAgent(ctx, "default", "project", time.Minute); ac != nil {
		t.Fatal("expected no agent")
	}
	if waited := time.Since(started); waited > time.Second {
		t.Fatalf("waited %s past the caller deadline", waited)
	}
}

// A connection that goes silent is still dropped, and it is told why.
func TestSilentAgentIsClosedWithAReason(t *testing.T) {
	d := NewAgentDispatcher()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		ac := d.Register("default", "project", "silent", conn)
		ac.Close(agentCloseKeepaliveTimeout, "no frame received for 45s")
	}))
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := conn.ReadMessage()
	closeErr, ok := readErr.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected a close frame, got %v", readErr)
	}
	if closeErr.Code != agentCloseKeepaliveTimeout {
		t.Fatalf("close code %d does not distinguish a keepalive timeout", closeErr.Code)
	}
	if !strings.Contains(closeErr.Text, "no frame received") {
		t.Fatalf("close reason does not name the silence: %q", closeErr.Text)
	}
}

// The margin exists so several consecutive keepalives can be lost. Equal values
// put the agent heartbeat exactly on the deadline, which is the bug.
func TestKeepaliveMarginLeavesRoomForMissedProbes(t *testing.T) {
	if defaultAgentReadTimeout <= defaultAgentPingInterval*3 {
		t.Fatalf("read timeout %s leaves fewer than three missed pings of slack", defaultAgentReadTimeout)
	}
	if defaultAgentReadTimeout == 30*time.Second {
		t.Fatal("read timeout still equals the agent heartbeat period it used to collide with")
	}
}

// End to end: a connection dropped for silence really does carry the reason,
// through the read-deadline path rather than a direct Close call.
func TestKeepaliveTimeoutSendsTheReasonToTheAgent(t *testing.T) {
	h := newAgentHandler(t, 20*time.Millisecond, 100*time.Millisecond)
	conn, cleanup := connectAgent(t, h)
	defer cleanup()

	if !waitForAgent(t, h, true) {
		t.Fatal("agent never registered")
	}
	// The client answers no ping, so the server's read deadline fires.
	if !waitForAgent(t, h, false) {
		t.Fatal("silent agent still registered after the read timeout elapsed")
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("the server hung up without a close frame: %v", err)
	}
	if closeErr.Code != agentCloseKeepaliveTimeout || !strings.Contains(closeErr.Text, "no frame received") {
		t.Fatalf("close frame does not name the keepalive timeout: %d %q", closeErr.Code, closeErr.Text)
	}
}
