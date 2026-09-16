package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestLocalQueueLimitsAndSharedCheckout(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		t.Run(map[bool]string{false: "shared", true: "worktrees"}[isolated], func(t *testing.T) {
			d := &agentDaemon{}
			first, _ := d.enqueueRun("task1", agentconfig.Dispatch{RunID: "one"}, "project", "/repo", 2, isolated)
			second, _ := d.enqueueRun("task2", agentconfig.Dispatch{RunID: "two"}, "project", "/repo", 2, isolated)
			if err := d.awaitRunSlot(context.Background(), first); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			err := d.awaitRunSlot(ctx, second)
			if isolated && err != nil {
				t.Fatal("independent worktrees should run concurrently")
			}
			if !isolated && err == nil {
				t.Fatal("shared checkout admitted concurrent run")
			}
			if !isolated {
				close(first.exited)
				if err := d.awaitRunSlot(context.Background(), second); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestLocalQueueCancellationAndFIFO(t *testing.T) {
	d := &agentDaemon{}
	first, _ := d.enqueueRun("task1", agentconfig.Dispatch{RunID: "one"}, "project", "/repo", 1, true)
	second, _ := d.enqueueRun("task2", agentconfig.Dispatch{RunID: "two"}, "project", "/repo", 1, true)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if d.awaitRunSlot(ctx, second) == nil {
		t.Fatal("later run bypassed queue")
	}
	first.canceled = true
	if d.awaitRunSlot(context.Background(), first) == nil {
		t.Fatal("canceled run admitted")
	}
	close(first.exited)
	if err := d.awaitRunSlot(context.Background(), second); err != nil {
		t.Fatal(err)
	}
}

func TestLocalQueueCanceledWaitingRunDoesNotBlockAdmission(t *testing.T) {
	d := &agentDaemon{}
	first, _ := d.enqueueRun("task1", agentconfig.Dispatch{RunID: "one"}, "project", "/repo", 1, true)
	second, _ := d.enqueueRun("task2", agentconfig.Dispatch{RunID: "two"}, "project", "/repo", 1, true)
	first.canceled = true
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := d.awaitRunSlot(ctx, second); err != nil {
		t.Fatal("canceled queued execution blocked admission before cleanup:", err)
	}
}

func TestLocalQueueCanceledActiveRunHoldsSlotUntilExit(t *testing.T) {
	d := &agentDaemon{}
	first, _ := d.enqueueRun("task1", agentconfig.Dispatch{RunID: "one"}, "project", "/repo", 1, true)
	second, _ := d.enqueueRun("task2", agentconfig.Dispatch{RunID: "two"}, "project", "/repo", 1, true)
	if err := d.awaitRunSlot(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	first.canceled = true
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if d.awaitRunSlot(ctx, second) == nil {
		t.Fatal("admitted work before canceled active execution exited")
	}
	close(first.exited)
	if err := d.awaitRunSlot(context.Background(), second); err != nil {
		t.Fatal(err)
	}
}

func TestAgentPullTasks(t *testing.T) {
	d := &agentDaemon{}
	first, _ := d.enqueueRun("task1", agentconfig.Dispatch{RunID: "one", TaskKey: "#1"}, "project", "/repo", 1, true)
	first.desktop.Status = "running"
	_, _ = d.enqueueRun("task2", agentconfig.Dispatch{RunID: "two", TaskKey: "#2"}, "project", "/repo", 1, true)
	// second is queued (default from enqueueRun)
	third, _ := d.enqueueRun("task3", agentconfig.Dispatch{RunID: "three", TaskKey: "#3"}, "project", "/repo", 1, true)
	third.desktop.Status = "completed"
	close(third.exited)

	// Create test websocket connection
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		d.handlePullTasks(context.Background(), conn, agentprotocol.Message{MsgID: "test-pull", Type: "pull_tasks"})
	}))
	defer server.Close()

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	var msg agentprotocol.Message
	if err := clientConn.ReadJSON(&msg); err != nil {
		t.Fatal(err)
	}
	if msg.MsgID != "test-pull" || msg.Type != "running_tasks" {
		t.Fatalf("unexpected message: %+v", msg)
	}

	var tasks []agentprotocol.RunningTask
	if err := json.Unmarshal(msg.Payload, &tasks); err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 active tasks (running and queued), got %d: %+v", len(tasks), tasks)
	}
	statuses := map[string]string{}
	for _, task := range tasks {
		statuses[task.ID] = task.Status
	}
	if statuses["one"] != "running" {
		t.Fatalf("expected run one to be running, got: %s", statuses["one"])
	}
	if statuses["two"] != "queued" {
		t.Fatalf("expected run two to be queued, got: %s", statuses["two"])
	}
}

// A free console holds no background worker capacity: it is neither counted in a
// project's active executions nor queued behind them.
func TestConsoleDoesNotConsumeWorkerCapacity(t *testing.T) {
	d := &agentDaemon{}
	first, _ := d.enqueueRun("task1", agentconfig.Dispatch{RunID: "one"}, "project", "/repo", 1, true)
	if err := d.awaitRunSlot(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	console, _ := d.enqueueRun("", agentconfig.Dispatch{RunID: "console"}, "project", "/repo", 1, false)
	console.desktop.Kind = consoleRunKind
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := d.awaitRunSlot(ctx, console); err != nil {
		t.Fatal("console waited for a background worker slot:", err)
	}
	second, _ := d.enqueueRun("task2", agentconfig.Dispatch{RunID: "two"}, "project", "/repo", 2, true)
	if err := d.awaitRunSlot(ctx, second); err != nil {
		t.Fatal("running console consumed a worker slot:", err)
	}
}

// Two consoles are opened deliberately by the user and do not serialize.
func TestConsolesRunSideBySide(t *testing.T) {
	d := &agentDaemon{}
	first, _ := d.enqueueRun("", agentconfig.Dispatch{RunID: "one"}, "project", "/repo", 1, false)
	first.desktop.Kind = consoleRunKind
	if err := d.awaitRunSlot(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second, _ := d.enqueueRun("", agentconfig.Dispatch{RunID: "two"}, "project", "/repo", 1, false)
	second.desktop.Kind = consoleRunKind
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := d.awaitRunSlot(ctx, second); err != nil {
		t.Fatal("second console serialized behind the first:", err)
	}
}

// The mapped checkout is still serialized: a console waits for an execution that
// works in it, and blocks a shared-checkout execution while it runs.
func TestConsoleStillSerializesSharedCheckout(t *testing.T) {
	d := &agentDaemon{}
	console, _ := d.enqueueRun("", agentconfig.Dispatch{RunID: "console"}, "project", "/repo", 3, false)
	console.desktop.Kind = consoleRunKind
	if err := d.awaitRunSlot(context.Background(), console); err != nil {
		t.Fatal(err)
	}
	task, _ := d.enqueueRun("task1", agentconfig.Dispatch{RunID: "one"}, "project", "/repo", 3, false)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := d.awaitRunSlot(ctx, task); err == nil {
		t.Fatal("shared checkout execution ran alongside a console")
	}
	close(console.exited)
	if err := d.awaitRunSlot(context.Background(), task); err != nil {
		t.Fatal(err)
	}
}
