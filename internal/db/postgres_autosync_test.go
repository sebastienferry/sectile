package db

import (
	"testing"
	"time"
)

// The claim on the engine several instances actually share: of two stores
// claiming the same due project, one wins, and the backoff and status are
// shared.
func TestPostgresAutoSyncClaimIsExclusive(t *testing.T) {
	first := openPostgres(t)
	seedProjectAndUser(t, first)
	if _, err := first.conn.Exec(`DELETE FROM auto_sync_projects`); err != nil {
		t.Fatalf("clearing pacing: %v", err)
	}
	if _, err := first.conn.Exec(`UPDATE auto_sync_state SET backoff_until = NULL, passes = 0, imported = 0 WHERE id = 1`); err != nil {
		t.Fatalf("resetting state: %v", err)
	}
	second, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("opening a second store: %v", err)
	}
	defer second.Close()

	now := time.Now().UTC()
	_, firstClaimed, err := first.claimAutoSyncPass("p1", 5*time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	_, secondClaimed, err := second.claimAutoSyncPass("p1", 5*time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if !firstClaimed || secondClaimed {
		t.Fatalf("claims = %v, %v; want exactly the first", firstClaimed, secondClaimed)
	}

	first.enterAutoSyncBackoff()
	if status := second.AutoSyncStatus(); status.BackoffUntil == "" {
		t.Errorf("the backoff is not shared: %+v", status)
	}
}
