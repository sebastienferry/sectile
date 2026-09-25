package handlers

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"tasks/internal/agentprotocol"
	"testing"
	"time"
)

func operationConnection(t *testing.T, d *AgentDispatcher) (*AgentConn, *websocket.Conn) {
	t.Helper()
	return operationConnectionBuild(t, d, AgentBuild{})
}

// operationConnectionBuild is operationConnection for an agent that announced
// build when it connected.
func operationConnectionBuild(t *testing.T, d *AgentDispatcher, build AgentBuild) (*AgentConn, *websocket.Conn) {
	t.Helper()
	registered := make(chan *AgentConn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		ac := d.RegisterBuild("default", "project", "test", build, conn)
		registered <- ac
		defer conn.Close()
		defer d.Unregister(ac.UserID, ac.ProjectID, conn)
		for {
			var msg AgentMessage
			if conn.ReadJSON(&msg) != nil {
				return
			}
			d.ReportOperation(ac, msg)
		}
	}))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return <-registered, conn
}

func TestOperationResultsAreBoundToConnection(t *testing.T) {
	d := NewAgentDispatcher()
	ac, conn := operationConnection(t, d)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		raw, err := d.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "git_status"})
		if err == nil && string(raw) != `{"branch":"local"}` {
			t.Errorf("wrong result: %s", raw)
		}
		done <- err
	}()
	var request AgentMessage
	if err := conn.ReadJSON(&request); err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{"value":{"branch":"local"}}`)
	response := AgentMessage{Type: "workspace_result", MsgID: request.MsgID, Payload: payload}
	impostor := &AgentConn{UserID: ac.UserID, ProjectID: ac.ProjectID}
	d.ReportOperation(impostor, response)
	select {
	case err := <-done:
		t.Fatalf("accepted wrong connection: %v", err)
	default:
	}
	if err := conn.WriteJSON(response); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestOperationDisconnectAndCancellation(t *testing.T) {
	for _, disconnect := range []bool{true, false} {
		t.Run(map[bool]string{true: "disconnect", false: "cancel"}[disconnect], func(t *testing.T) {
			d := NewAgentDispatcher()
			_, conn := operationConnection(t, d)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := d.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "git_status"})
				done <- err
			}()
			var request AgentMessage
			if err := conn.ReadJSON(&request); err != nil {
				t.Fatal(err)
			}
			if disconnect {
				conn.Close()
			} else {
				cancel()
				var message AgentMessage
				if err := conn.ReadJSON(&message); err != nil {
					t.Fatal(err)
				}
				if message.Type != "workspace_cancel" || message.MsgID != request.MsgID {
					t.Fatalf("cancel: %#v", message)
				}
			}
			if err := <-done; err == nil {
				t.Fatal("unconfirmed operation succeeded")
			}
			d.mu.RLock()
			count := len(d.operations)
			d.mu.RUnlock()
			if count != 0 {
				t.Fatal("pending operation leaked")
			}
		})
	}
	d := NewAgentDispatcher()
	if _, err := d.CallOperation(context.Background(), agentprotocol.Operation{ProjectID: "missing"}); err == nil {
		t.Fatal("offline operation succeeded")
	}
}
