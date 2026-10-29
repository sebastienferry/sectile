package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

func TestCloseContextKeepsSQLAliveWhileAListenerIsRunning(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	callbackResult := make(chan error, 1)
	database.RegisterPostBackListener(func(*models.Task, *models.TaskActivity, error) {
		close(started)
		<-release
		_, err := database.GetSettings()
		callbackResult <- err
	})
	database.notifyPostBackListeners(nil, nil, nil)
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := database.CloseContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("close with active listener returned %v", err)
	}
	if _, err := database.GetSettings(); err != nil {
		t.Fatalf("timed-out close released SQL: %v", err)
	}
	close(release)
	if err := <-callbackResult; err != nil {
		t.Fatalf("listener lost SQL before completion: %v", err)
	}
	if err := database.CloseContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := database.CloseContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCloseStopsAutoSyncWithoutWaitingForItsNextTick(t *testing.T) {
	database, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	database.StartAutoSync()
	started := time.Now()
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("auto-sync loop took %s to stop", elapsed)
	}
}
