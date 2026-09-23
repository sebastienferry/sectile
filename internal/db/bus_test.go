package db

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
)

// What arrives on the channel is acted on only when another instance sent it:
// an instance's own messages and anything unreadable are ignored.
func TestHandleBusMessageActsOnlyOnOtherInstances(t *testing.T) {
	d, _ := newInstanceTestDB(t)
	var received []BusMessage
	d.OnRelayedEvent(func(m BusMessage) { received = append(received, m) })

	own, _ := json.Marshal(BusMessage{Origin: d.InstanceID(), Kind: busKindEvent, Type: "task_updated", TaskID: "t1"})
	d.handleBusMessage(string(own))
	d.handleBusMessage("not json")
	d.handleBusMessage(`{"k":"event","t":"task_updated"}`)
	if len(received) != 0 {
		t.Fatalf("own, unreadable or unsigned messages were delivered: %+v", received)
	}

	other, _ := json.Marshal(BusMessage{Origin: "other", Kind: busKindEvent, Type: "task_updated", TaskID: "t1", ActivityID: "a1", Error: "boom"})
	d.handleBusMessage(string(other))
	if len(received) != 1 || received[0].Type != "task_updated" || received[0].TaskID != "t1" || received[0].ActivityID != "a1" || received[0].Error != "boom" {
		t.Fatalf("another instance's event was not delivered as sent: %+v", received)
	}
}

// A cancellation published by another instance stops the job this one runs.
func TestACancelFromAnotherInstanceStopsTheLocalJob(t *testing.T) {
	d, _ := newInstanceTestDB(t)
	stopped := false
	d.cancelMu.Lock()
	d.cancelMap["running-here"] = func() { stopped = true }
	d.cancelMu.Unlock()

	msg, _ := json.Marshal(BusMessage{Origin: "other", Kind: busKindCancel, ActivityID: "running-here"})
	d.handleBusMessage(string(msg))
	if !stopped {
		t.Fatal("the local job was not stopped")
	}
	d.cancelMu.Lock()
	_, left := d.cancelMap["running-here"]
	d.cancelMu.Unlock()
	if left {
		t.Error("the stopped job's cancel function is still registered")
	}
}

func TestTruncateRunesKeepsWholeCharacters(t *testing.T) {
	if got := truncateRunes("éééé", 2); got != "éé" {
		t.Errorf("truncateRunes = %q, want %q", got, "éé")
	}
	if got := truncateRunes("short", busErrorMax); got != "short" {
		t.Errorf("a short text was changed: %q", got)
	}
	if got := truncateRunes(strings.Repeat("x", busErrorMax+10), busErrorMax); len(got) != busErrorMax {
		t.Errorf("length = %d, want %d", len(got), busErrorMax)
	}
}

// SQLite serves one process: publishing and starting the bus do nothing, and
// do not fail.
func TestTheBusIsInertOnSQLite(t *testing.T) {
	d, _ := newInstanceTestDB(t)
	d.PublishEvent("task_updated", "t1", "", "")
	stop, err := d.StartEventBus()
	if err != nil {
		t.Fatalf("starting the bus on SQLite: %v", err)
	}
	stop()
}

// A canceled job stays canceled: it does not start late, and its end, when it
// was already running, does not overwrite the cancellation.
func TestACanceledJobStaysCanceled(t *testing.T) {
	t.Run("late start", func(t *testing.T) {
		d, _ := newInstanceTestDB(t)
		seedOwnedActivity(t, d, "late", "custom", "run", "canceled", "")
		d.processSkillJob(SkillJob{ActivityID: "late", SkillID: "custom", TaskID: "t1"})
		if got := activityStatus(t, d, "late"); got != "canceled" {
			t.Fatalf("status = %q, want canceled", got)
		}
		var owner string
		if err := d.conn.QueryRow(`SELECT instance_id FROM task_activities WHERE id = 'late'`).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		if owner != "" {
			t.Errorf("a canceled job was started by %q", owner)
		}
	})

	t.Run("agent launch canceled while dispatching", func(t *testing.T) {
		d, _ := newInstanceTestDB(t)
		seedOwnedActivity(t, d, "launch", "custom", "run", "queued", "")
		d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
			if err := d.CancelActivity("launch"); err != nil {
				t.Errorf("canceling: %v", err)
			}
			return nil, errors.New("agent gone")
		})
		d.processSkillJob(SkillJob{ActivityID: "launch", SkillID: "custom", TaskID: "t1"})
		if got := activityStatus(t, d, "launch"); got != "canceled" {
			t.Fatalf("status = %q after the launch ended, want canceled", got)
		}
	})

	t.Run("tracker operation end", func(t *testing.T) {
		d, _ := newInstanceTestDB(t)
		seedOwnedActivity(t, d, "op", "tracker_op", "run", "canceled", "")
		d.finishTrackerOp("op", []string{"done"}, "", nil)
		if got := activityStatus(t, d, "op"); got != "canceled" {
			t.Fatalf("status = %q, want canceled", got)
		}
	})

	t.Run("tracker update of a missing task", func(t *testing.T) {
		d, _ := newInstanceTestDB(t)
		seedOwnedActivity(t, d, "update", "tracker_update", "run", "canceled", "")
		d.processTrackerUpdateJob(context.Background(), SkillJob{ActivityID: "update", SkillID: "tracker_update", TaskID: "missing"})
		if got := activityStatus(t, d, "update"); got != "canceled" {
			t.Fatalf("status = %q, want canceled", got)
		}
	})
}
