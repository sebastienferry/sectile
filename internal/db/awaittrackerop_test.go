package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

// transition_stage reports what its tracker job did, so the wait must return
// as soon as the worker closed the activity, and give up on time otherwise
// (#870).
func TestAwaitTrackerOp(t *testing.T) {
	t.Run("returns the outcome once the worker closed the activity", func(t *testing.T) {
		d, _ := newInstanceTestDB(t)
		seedOwnedActivity(t, d, "op", "tracker_op", "stage", "queued", "")
		go func() {
			time.Sleep(150 * time.Millisecond)
			d.finishTrackerOp("op", nil, "", errors.New("tracker returned HTTP 400: INVALID_INPUT"))
		}()
		start := time.Now()
		activity, err := d.AwaitTrackerOp(context.Background(), "op", 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if activity.Status != "failed" || activity.Error != "tracker returned HTTP 400: INVALID_INPUT" {
			t.Fatalf("outcome = %s %q, want the failure", activity.Status, activity.Error)
		}
		if waited := time.Since(start); waited > 2*time.Second {
			t.Fatalf("waited %s after the activity closed", waited)
		}
	})

	t.Run("an activity already closed is returned at once", func(t *testing.T) {
		d, _ := newInstanceTestDB(t)
		seedOwnedActivity(t, d, "op", "tracker_op", "stage", "completed", "")
		activity, err := d.AwaitTrackerOp(context.Background(), "op", time.Hour)
		if err != nil || activity.Status != "completed" {
			t.Fatalf("got %v, %v", activity, err)
		}
	})

	t.Run("a job still running at the bound is left queued", func(t *testing.T) {
		d, _ := newInstanceTestDB(t)
		seedOwnedActivity(t, d, "op", "tracker_op", "stage", "running", "")
		activity, err := d.AwaitTrackerOp(context.Background(), "op", 200*time.Millisecond)
		if err != nil || activity.Status != "running" {
			t.Fatalf("got %v, %v", activity, err)
		}
	})

	t.Run("a canceled request stops the wait", func(t *testing.T) {
		d, _ := newInstanceTestDB(t)
		seedOwnedActivity(t, d, "op", "tracker_op", "stage", "queued", "")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		activity, err := d.AwaitTrackerOp(ctx, "op", time.Hour)
		if err != nil || activity.Status != "queued" {
			t.Fatalf("got %v, %v", activity, err)
		}
		if waited := time.Since(start); waited > time.Second {
			t.Fatalf("waited %s on a canceled request", waited)
		}
	})
}
