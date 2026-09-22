package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

// inUTC runs fn with the process zone set to UTC, which is what the deployment
// runs in and the condition under which a tracker date used to be written in a
// form the driver could not read back. The workstations that never reproduced
// the bug simply ran in the tracker's own zone.
func inUTC(t *testing.T, fn func()) {
	t.Helper()
	previous := time.Local
	time.Local = time.UTC
	defer func() { time.Local = previous }()
	fn()
}

// A tracker date whose offset is not the server's must survive the round trip.
// It did not: the task list answered 500 as soon as one row carried one.
func TestTrackerDateRoundTripsOutsideItsOwnZone(t *testing.T) {
	inUTC(t, func() {
		database := testDB(t)

		trackerCreated, err := time.Parse(time.RFC3339, "2025-09-15T12:34:56.789+02:00")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.conn.Exec(
			`INSERT INTO tasks (id, project_id, key, title, tracker_created_at) VALUES (?, ?, ?, ?, ?)`,
			"t-1", "default", "KEY-1", "A ticket from a tracker two hours ahead", &trackerCreated,
		); err != nil {
			t.Fatal(err)
		}

		var got sql.NullTime
		if err := database.conn.QueryRow(`SELECT tracker_created_at FROM tasks WHERE id = ?`, "t-1").Scan(&got); err != nil {
			t.Fatalf("reading back a tracker date must not fail: %v", err)
		}
		if !got.Valid || !got.Time.Equal(trackerCreated) {
			t.Errorf("expected %s, got %v", trackerCreated, got)
		}

		tasks, err := database.GetTasks("", "", "", "", "default", "", "", "", "", nil, nil, false)
		if err != nil {
			t.Fatalf("listing the tasks must not fail: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].TrackerCreatedAt == nil || !tasks[0].TrackerCreatedAt.Equal(trackerCreated) {
			t.Errorf("expected %s on the listed task, got %v", trackerCreated, tasks[0].TrackerCreatedAt)
		}
	})
}

// The rows written before the format was fixed are still on disk, and one of
// them is enough to break the list. Opening the database must repair them.
func TestOpeningRepairsTimestampsWrittenWithANumericZone(t *testing.T) {
	inUTC(t, func() {
		path := filepath.Join(t.TempDir(), "tasks.db")

		database, err := NewDB(path)
		if err != nil {
			t.Fatal(err)
		}
		// Exactly what time.Time.String() produced for an offset the server's
		// zone does not name, written as text the way the old driver did.
		if _, err := database.conn.Exec(
			`INSERT INTO tasks (id, project_id, key, title, tracker_created_at, tracker_updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			"t-1", "default", "KEY-1", "A ticket imported by the old build",
			"2025-09-15 12:34:56.789 +0200 +0200", "2025-09-16 08:00:00 +0200 +0200",
		); err != nil {
			t.Fatal(err)
		}
		// A value that came from time.Now() carries a monotonic reading after
		// the zone, and breaks the same way once the zone has no name.
		if _, err := database.conn.Exec(
			`INSERT INTO tasks (id, project_id, key, title, tracker_created_at) VALUES (?, ?, ?, ?, ?)`,
			"t-3", "default", "KEY-3", "A ticket stamped from a fixed-offset clock",
			"2025-09-15 12:34:56.789 +0200 +0200 m=+15787.311877001",
		); err != nil {
			t.Fatal(err)
		}
		// A value the driver has always read back must be left untouched.
		if _, err := database.conn.Exec(
			`INSERT INTO tasks (id, project_id, key, title, tracker_created_at) VALUES (?, ?, ?, ?, ?)`,
			"t-2", "default", "KEY-2", "A ticket imported in the tracker's own zone",
			"2025-09-15 12:34:56.789 +0200 CEST",
		); err != nil {
			t.Fatal(err)
		}

		var scanned sql.NullTime
		if err := database.conn.QueryRow(`SELECT tracker_created_at FROM tasks WHERE id = ?`, "t-1").Scan(&scanned); err == nil {
			t.Fatal("expected the unrepaired value to fail the scan, which is the bug being fixed")
		}
		forgetSchemaVersion(t, database)
		database.Close()

		repaired, err := NewDB(path)
		if err != nil {
			t.Fatal(err)
		}
		defer repaired.Close()

		expected, err := time.Parse(time.RFC3339, "2025-09-15T12:34:56.789+02:00")
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"t-1", "t-2", "t-3"} {
			var got sql.NullTime
			if err := repaired.conn.QueryRow(`SELECT tracker_created_at FROM tasks WHERE id = ?`, id).Scan(&got); err != nil {
				t.Fatalf("%s: reading back must not fail after the repair: %v", id, err)
			}
			if !got.Valid || !got.Time.Equal(expected) {
				t.Errorf("%s: expected %s, got %v", id, expected, got)
			}
		}

		tasks, err := repaired.GetTasks("", "", "", "", "default", "", "", "", "", nil, nil, false)
		if err != nil {
			t.Fatalf("listing the tasks must not fail after the repair: %v", err)
		}
		if len(tasks) != 3 {
			t.Fatalf("expected 3 tasks, got %d", len(tasks))
		}
	})
}

// The sweep must not rewrite a value it does not understand, and must not
// choke on a column that holds something other than a date.
func TestRepairLeavesUnrelatedValuesAlone(t *testing.T) {
	inUTC(t, func() {
		database := testDB(t)

		if _, err := database.conn.Exec(
			`INSERT INTO tasks (id, project_id, key, title, tracker_created_at) VALUES (?, ?, ?, ?, ?)`,
			"t-1", "default", "KEY-1", "A ticket with a due date and no tracker date", "not a date at all +0200",
		); err != nil {
			t.Fatal(err)
		}
		database.repairNumericZoneTimestamps()

		var raw string
		if err := database.conn.QueryRow(`SELECT CAST(tracker_created_at AS TEXT) FROM tasks WHERE id = ?`, "t-1").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if raw != "not a date at all +0200" {
			t.Errorf("expected the value to be left as is, got %q", raw)
		}
	})
}

// An import under a zone that is not the tracker's is the path that produced
// the broken rows in the first place.
func TestImportedTaskReadsBackOutsideTheTrackersZone(t *testing.T) {
	inUTC(t, func() {
		database := testDB(t)

		trackerCreated, err := time.Parse(time.RFC3339, "2025-09-15T12:34:56.789+02:00")
		if err != nil {
			t.Fatal(err)
		}
		trackerUpdated := trackerCreated.Add(time.Hour)
		if err := database.ImportOrUpdateTasks([]models.Task{{
			ProjectID:        "default",
			Key:              "KEY-1",
			Title:            "A ticket synced from a tracker two hours ahead",
			Status:           models.StatusToClarify,
			Priority:         models.PriorityMedium,
			Source:           "local",
			TrackerCreatedAt: &trackerCreated,
			TrackerUpdatedAt: &trackerUpdated,
		}}); err != nil {
			t.Fatal(err)
		}

		tasks, err := database.GetTasks("", "", "", "", "default", "", "", "", "", nil, nil, false)
		if err != nil {
			t.Fatalf("listing the tasks must not fail: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].TrackerCreatedAt == nil || !tasks[0].TrackerCreatedAt.Equal(trackerCreated) {
			t.Errorf("expected %s, got %v", trackerCreated, tasks[0].TrackerCreatedAt)
		}
	})
}
