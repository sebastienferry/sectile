package handlers

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStopBackgroundWaitsForAcceptedAgentWork(t *testing.T) {
	h := NewHandler(nil)
	started := make(chan struct{})
	release := make(chan struct{})
	h.launchBackground(func() {
		close(started)
		<-release
	})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := h.StopBackground(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("active work did not hold shutdown: %v", err)
	}
	if h.beginBackground() {
		h.endBackground()
		t.Fatal("background work was admitted during shutdown")
	}
	close(release)
	if err := h.StopBackground(context.Background()); err != nil {
		t.Fatal(err)
	}
}
