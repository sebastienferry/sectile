package db

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"tasks/internal/models"
)

// A large import lets the other users of the lock in between its chunks: held
// for a whole full read, it stalled the instance heartbeat and every agent
// connection of the process long enough for both to be dropped.
func TestImportReleasesTheLockBetweenChunks(t *testing.T) {
	d := testDB(t)
	seedProjectAndUser(t, d)

	previousSize, previousPause := importChunkSize, importChunkPause
	t.Cleanup(func() { importChunkSize, importChunkPause = previousSize, previousPause })
	importChunkSize = 2
	pauses, held := 0, 0
	importChunkPause = func() {
		pauses++
		if !d.mu.TryLock() {
			held++
			return
		}
		d.mu.Unlock()
	}

	var synced []models.Task
	for i := 1; i <= 5; i++ {
		synced = append(synced, models.Task{ProjectID: "p1", Key: fmt.Sprintf("#%d", i), Title: fmt.Sprintf("Ticket %d", i), Status: models.StatusToClarify, Priority: models.PriorityMedium})
	}
	if err := d.ImportOrUpdateTasks("", synced); err != nil {
		t.Fatalf("ImportOrUpdateTasks: %v", err)
	}

	if pauses != 2 {
		t.Errorf("five tickets in chunks of two paused %d times, want 2", pauses)
	}
	if held != 0 {
		t.Errorf("the lock was still held at %d of the %d chunk boundaries", held, pauses)
	}
	for _, ticket := range synced {
		if got, err := d.GetTaskByID(ticket.Key); err != nil || got == nil {
			t.Errorf("ticket %s was not imported: %v", ticket.Key, err)
		}
	}
}

// A heartbeat for a session the directory no longer holds says so, so the
// connection can be hung up on rather than left serving nothing.
func TestPingingAForgottenWorkstationSessionSaysItIsGone(t *testing.T) {
	d := testDB(t)
	if err := d.WorkstationPing("forgotten", time.Now().UTC()); !errors.Is(err, ErrWorkstationSessionGone) {
		t.Fatalf("WorkstationPing = %v, want ErrWorkstationSessionGone", err)
	}
}
