package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// launchConnection registers an agent and hands back the client side of the
// socket, so a test can decide whether to confirm a launch or stay silent.
func launchConnection(t *testing.T, d *AgentDispatcher) *websocket.Conn {
	t.Helper()
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		ac := d.Register("default", "project", "workstation-7", conn)
		close(ready)
		defer conn.Close()
		defer d.Unregister(ac.UserID, ac.ProjectID, conn)
		for {
			var msg AgentMessage
			if conn.ReadJSON(&msg) != nil {
				return
			}
			d.ReportLaunchStatus(ac, msg)
		}
	}))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	<-ready
	return conn
}

func TestUnconfirmedLaunchIsDistinctFromAFailedOne(t *testing.T) {
	d := NewAgentDispatcher()
	conn := launchConnection(t, d)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := d.DispatchAndWait(ctx, "default", "project", "task",
		agentconfig.Dispatch{SchemaVersion: agentconfig.Version, TaskID: "task", SkillID: "clarify", Action: "clarify"})
	if err == nil {
		t.Fatal("an unconfirmed launch reported success")
	}
	// The caller must be able to tell "never confirmed" from "the agent said it
	// failed": only the first leaves the run for the agent to finish.
	if !errors.Is(err, ErrLaunchUnconfirmed) {
		t.Fatalf("not reported as unconfirmed: %v", err)
	}
	for _, want := range []string{"clarify", "workstation-7"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error does not name %q: %v", want, err)
		}
	}
	// The agent is still there and still working; nothing about it has been closed.
	if err := conn.WriteJSON(AgentMessage{Type: "ping"}); err != nil {
		t.Fatalf("agent connection closed by an unconfirmed launch: %v", err)
	}
}

func TestReportedLaunchFailureIsNotUnconfirmed(t *testing.T) {
	d := NewAgentDispatcher()
	conn := launchConnection(t, d)
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- d.DispatchAndWait(ctx, "default", "project", "task",
			agentconfig.Dispatch{SchemaVersion: agentconfig.Version, TaskID: "task", SkillID: "clarify", Action: "clarify"})
	}()
	var request AgentMessage
	if err := conn.ReadJSON(&request); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(agentLaunchStatus{Status: "failed", Summary: "terminal missing"})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(AgentMessage{Type: "launch_status", MsgID: request.MsgID, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	err = <-done
	if err == nil || errors.Is(err, ErrLaunchUnconfirmed) {
		t.Fatalf("a reported failure must close the run: %v", err)
	}
	if !strings.Contains(err.Error(), "terminal missing") {
		t.Fatalf("error drops the agent's reason: %v", err)
	}
}

func TestConfirmedLaunchSucceeds(t *testing.T) {
	d := NewAgentDispatcher()
	conn := launchConnection(t, d)
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- d.DispatchAndWait(ctx, "default", "project", "task",
			agentconfig.Dispatch{SchemaVersion: agentconfig.Version, TaskID: "task", SkillID: "clarify", Action: "clarify"})
	}()
	var request AgentMessage
	if err := conn.ReadJSON(&request); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(agentLaunchStatus{Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(AgentMessage{Type: "launch_status", MsgID: request.MsgID, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestUnconfirmedOperationNamesWhatItWaitedFor(t *testing.T) {
	d := NewAgentDispatcher()
	_, conn := operationConnection(t, d)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := d.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "git_evidence"})
		done <- err
	}()
	var request AgentMessage
	if err := conn.ReadJSON(&request); err != nil {
		t.Fatal(err)
	}
	err := <-done
	if err == nil {
		t.Fatal("unconfirmed operation reported success")
	}
	// Without the action and the device, a recurrence cannot be triaged from the log.
	for _, want := range []string{"git_evidence", "test"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error does not name %q: %v", want, err)
		}
	}
	if !strings.Contains(err.Error(), "after ") {
		t.Fatalf("error does not report how long it waited: %v", err)
	}
}
