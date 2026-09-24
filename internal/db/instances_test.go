package db

import (
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

// seedInstances registers a live and a stale instance, as two other processes
// sharing the database would have.
func seedInstances(t *testing.T, d *DB, now time.Time) {
	t.Helper()
	for _, row := range []struct {
		id       string
		lastSeen time.Time
	}{
		{"live", now},
		{"stale", now.Add(-2 * instanceDeadAfter)},
	} {
		if _, err := d.conn.Exec(`INSERT INTO server_instances (id, hostname, pid, started_at, last_seen) VALUES (?, 'h', 1, ?, ?)`,
			row.id, row.lastSeen, row.lastSeen); err != nil {
			t.Fatalf("seeding instance %s: %v", row.id, err)
		}
	}
}

// seedOwnedActivity puts several active runs on one task, which only concurrent
// runs may share; the restart rules do not look at the flag.
func seedOwnedActivity(t *testing.T, d *DB, id, skillID, action, status, instanceID string) {
	t.Helper()
	if _, err := d.conn.Exec(`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, instance_id, concurrent) VALUES (?, 't1', ?, 'S', ?, ?, ?, 1)`,
		id, skillID, action, status, instanceID); err != nil {
		t.Fatalf("seeding activity %s: %v", id, err)
	}
}

func activityStatus(t *testing.T, d *DB, id string) string {
	t.Helper()
	var status string
	if err := d.conn.QueryRow(`SELECT status FROM task_activities WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("reading %s: %v", id, err)
	}
	return status
}

func instanceIDs(t *testing.T, d *DB) map[string]bool {
	t.Helper()
	rows, err := d.conn.Query(`SELECT id FROM server_instances`)
	if err != nil {
		t.Fatalf("listing instances: %v", err)
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[id] = true
	}
	return ids
}

func newInstanceTestDB(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "instances.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	seedProjectAndUser(t, d)
	seedTask(t, d)
	return d, path
}

// The rule every instance applies to the others: the work of an instance that
// is gone ends as a restart always ended it, the work of a live one is left
// alone, and so is whatever an agent supervises.
func TestReclaimDeadInstancesOnlyTouchesTheWorkOfTheGone(t *testing.T) {
	d, _ := newInstanceTestDB(t)
	now := time.Now().UTC()
	seedInstances(t, d, now)

	seedOwnedActivity(t, d, "job-live", "implement", "run", "running", "live")
	seedOwnedActivity(t, d, "job-stale", "implement", "run", "running", "stale")
	seedOwnedActivity(t, d, "job-queued-stale", "tracker_op", "run", "queued", "stale")
	seedOwnedActivity(t, d, "job-unknown", "implement", "run", "running", "ghost")
	seedOwnedActivity(t, d, "job-legacy", "implement", "run", "pending", "")
	seedOwnedActivity(t, d, "job-done-stale", "implement", "run", "completed", "stale")
	seedOwnedActivity(t, d, "client-live", "remote_run", RunActionClient, "running", "live")
	seedOwnedActivity(t, d, "client-stale", "remote_run", RunActionClient, "running", "stale")
	seedOwnedActivity(t, d, "agent-stale", "remote_run", RunActionAgent, "running", "stale")

	ended, err := d.reclaimDeadInstances(now)
	if err != nil {
		t.Fatalf("reclaiming: %v", err)
	}
	if ended != 5 {
		t.Errorf("ended = %d, want 5 (four jobs and one client run)", ended)
	}
	for id, want := range map[string]string{
		"job-live":         "running",
		"job-stale":        "failed",
		"job-queued-stale": "failed",
		"job-unknown":      "failed",
		"job-legacy":       "failed",
		"job-done-stale":   "completed",
		"client-live":      "running",
		"client-stale":     "canceled",
		"agent-stale":      "running",
	} {
		if got := activityStatus(t, d, id); got != want {
			t.Errorf("%s: status = %q, want %q", id, got, want)
		}
	}
	var reason string
	if err := d.conn.QueryRow(`SELECT error FROM task_activities WHERE id = 'job-stale'`).Scan(&reason); err != nil {
		t.Fatal(err)
	}
	if reason != interruptedByRestart {
		t.Errorf("reclaimed job error = %q, want %q", reason, interruptedByRestart)
	}
	var summary string
	if err := d.conn.QueryRow(`SELECT summary FROM task_activities WHERE id = 'client-stale'`).Scan(&summary); err != nil {
		t.Fatal(err)
	}
	if summary != interruptedClientRun {
		t.Errorf("reclaimed client run summary = %q, want %q", summary, interruptedClientRun)
	}

	if ids := instanceIDs(t, d); !ids["live"] || ids["stale"] {
		t.Errorf("instances after reclaim = %v, want only the live one", ids)
	}

	// A second instance reclaiming at the same moment finds nothing left.
	again, err := d.reclaimDeadInstances(now)
	if err != nil {
		t.Fatalf("reclaiming again: %v", err)
	}
	if again != 0 {
		t.Errorf("second reclaim ended %d activities, want 0", again)
	}
}

// Whatever this process creates, and whatever job it starts, is its own.
func TestActivitiesCarryTheInstanceThatOwnsThem(t *testing.T) {
	d, _ := newInstanceTestDB(t)
	if d.InstanceID() == "" {
		t.Fatal("the store has no instance id")
	}

	if err := d.AddTaskActivity(models.TaskActivity{ID: "created", TaskID: "t1", SkillID: "tracker_update", Status: "running", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	var owner string
	if err := d.conn.QueryRow(`SELECT instance_id FROM task_activities WHERE id = 'created'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != d.InstanceID() {
		t.Errorf("created activity owner = %q, want %q", owner, d.InstanceID())
	}

	// A job queued by another instance and executed here changes hands. The
	// task does not exist, so the job fails at once without dispatching.
	seedOwnedActivity(t, d, "handed-over", "custom", "run", "queued", "other")
	d.processSkillJob(SkillJob{ActivityID: "handed-over", SkillID: "custom", TaskID: "missing"})
	if err := d.conn.QueryRow(`SELECT instance_id FROM task_activities WHERE id = 'handed-over'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != d.InstanceID() {
		t.Errorf("started job owner = %q, want %q", owner, d.InstanceID())
	}
}

// Opening the store is not serving: only StartInstance registers, keeps the
// row fresh, comes back after being taken for dead, and leaves on stop.
func TestStartInstanceKeepsItsRowAlive(t *testing.T) {
	heartbeat, reclaim := instanceHeartbeatEvery, instanceReclaimEvery
	instanceHeartbeatEvery, instanceReclaimEvery = 10*time.Millisecond, time.Hour
	t.Cleanup(func() { instanceHeartbeatEvery, instanceReclaimEvery = heartbeat, reclaim })

	d, _ := newInstanceTestDB(t)
	if ids := instanceIDs(t, d); len(ids) != 0 {
		t.Fatalf("opening the store registered %v", ids)
	}

	stop, err := d.StartInstance()
	if err != nil {
		t.Fatalf("starting the instance: %v", err)
	}
	defer stop()
	if ids := instanceIDs(t, d); !ids[d.InstanceID()] {
		t.Fatalf("instance %s not registered: %v", d.InstanceID(), ids)
	}

	old := time.Now().UTC().Add(-time.Hour)
	if _, err := d.conn.Exec(`UPDATE server_instances SET last_seen = ? WHERE id = ?`, old, d.InstanceID()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the heartbeat to refresh the row", func() bool {
		var seen time.Time
		if err := d.conn.QueryRow(`SELECT last_seen FROM server_instances WHERE id = ?`, d.InstanceID()).Scan(&seen); err != nil {
			return false
		}
		return seen.After(old.Add(time.Minute))
	})

	if _, err := d.conn.Exec(`DELETE FROM server_instances WHERE id = ?`, d.InstanceID()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the instance to register again", func() bool { return instanceIDs(t, d)[d.InstanceID()] })

	stop()
	if ids := instanceIDs(t, d); ids[d.InstanceID()] {
		t.Errorf("stop left the instance registered: %v", ids)
	}
}

// SQLite serves one process: a start reclaims everything unfinished, whoever
// owned it, and forgets every instance, as a restart always did.
func TestSQLiteStartReclaimsEverything(t *testing.T) {
	d, path := newInstanceTestDB(t)
	seedInstances(t, d, time.Now().UTC())
	seedOwnedActivity(t, d, "job-live", "implement", "run", "running", "live")
	seedOwnedActivity(t, d, "client-live", "remote_run", RunActionClient, "running", "live")
	seedOwnedActivity(t, d, "agent-live", "remote_run", RunActionAgent, "running", "live")
	d.Close()

	restarted, err := NewDB(path)
	if err != nil {
		t.Fatalf("restarting: %v", err)
	}
	defer restarted.Close()
	for id, want := range map[string]string{"job-live": "failed", "client-live": "canceled", "agent-live": "running"} {
		if got := activityStatus(t, restarted, id); got != want {
			t.Errorf("%s: status = %q after a restart, want %q", id, got, want)
		}
	}
	if ids := instanceIDs(t, restarted); len(ids) != 0 {
		t.Errorf("instances after a SQLite restart = %v, want none", ids)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
