package main

import (
	"context"
	"tasks/internal/agentconfig"
	"testing"
	"time"
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
