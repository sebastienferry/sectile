package taskmcp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionMaintenanceJoinsAfterStop(t *testing.T) {
	registry := NewSessionRegistry(nil)
	registry.SetKeepalive(time.Hour, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := registry.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("maintenance unexpectedly stopped before Stop: %v", err)
	}
	registry.Stop()
	registry.Stop()
	if err := registry.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}
