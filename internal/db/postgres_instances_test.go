package db

import (
	"testing"
	"time"
)

// Two server instances share one PostgreSQL database. One starting must leave
// the other's work alone while it is alive, and reclaim it once it is gone.
// This is the failure #403 removes: every start used to reclaim everything.
func TestPostgresStartLeavesALiveInstanceAlone(t *testing.T) {
	d := openPostgres(t)
	if _, err := d.conn.Exec(`DELETE FROM server_instances`); err != nil {
		t.Fatalf("clearing instances: %v", err)
	}
	t.Cleanup(func() { _, _ = d.conn.Exec(`DELETE FROM server_instances`) })
	seedProjectAndUser(t, d)
	seedTask(t, d)

	stop, err := d.StartInstance()
	if err != nil {
		t.Fatalf("starting the first instance: %v", err)
	}
	defer stop()
	seedOwnedActivity(t, d, "job", "implement", "run", "running", d.InstanceID())
	seedOwnedActivity(t, d, "client", "remote_run", RunActionClient, "running", d.InstanceID())

	second, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("starting a second instance: %v", err)
	}
	for _, id := range []string{"job", "client"} {
		if got := activityStatus(t, d, id); got != "running" {
			t.Errorf("%s: status = %q after another instance started, want running", id, got)
		}
	}
	second.Close()

	// The first instance goes silent past the bound: the next start reclaims.
	stop()
	stale := time.Now().UTC().Add(-2 * instanceDeadAfter)
	if _, err := d.conn.Exec(`INSERT INTO server_instances (id, hostname, pid, started_at, last_seen) VALUES (?, 'h', 1, ?, ?)`,
		d.InstanceID(), stale, stale); err != nil {
		t.Fatalf("marking the instance stale: %v", err)
	}
	third, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("starting after the first instance died: %v", err)
	}
	defer third.Close()
	for id, want := range map[string]string{"job": "failed", "client": "canceled"} {
		if got := activityStatus(t, d, id); got != want {
			t.Errorf("%s: status = %q after its instance died, want %q", id, got, want)
		}
	}
}

// An instance sharing the database says it is alive even while a long import
// holds the store lock. Waiting behind that lock let the heartbeat fall past
// the bound, and the other instances reclaimed the work of an instance that
// was still serving it.
func TestPostgresHeartbeatDoesNotWaitForTheStoreLock(t *testing.T) {
	d := openPostgres(t)
	t.Cleanup(func() { _, _ = d.conn.Exec(`DELETE FROM server_instances`) })
	if err := d.registerInstance(time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("registering the instance: %v", err)
	}

	d.mu.Lock()
	beat := make(chan struct{})
	go func() {
		d.heartbeatInstance(time.Now().UTC())
		close(beat)
	}()
	select {
	case <-beat:
	case <-time.After(5 * time.Second):
		d.mu.Unlock()
		t.Fatal("the heartbeat waited for the store lock")
	}
	d.mu.Unlock()

	var lastSeen time.Time
	if err := d.conn.QueryRow(`SELECT last_seen FROM server_instances WHERE id = ?`, d.InstanceID()).Scan(&lastSeen); err != nil {
		t.Fatalf("reading the instance row: %v", err)
	}
	if time.Since(lastSeen) > 30*time.Second {
		t.Errorf("last_seen = %v, want the heartbeat's time", lastSeen)
	}
}
