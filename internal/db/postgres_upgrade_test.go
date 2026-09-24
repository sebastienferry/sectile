package db

import (
	"fmt"
	"slices"
	"testing"
)

// TestPostgresBaselinesADatabaseWrittenByAnOlderBinary covers the one thing a
// schema built from scratch can never cover: the upgrade.
//
// A PostgreSQL database created by an earlier version keeps the schema it was
// born with and carries no version row, so the test has to put the database
// back into that state: there is no older schema to ask for, only a DROP and a
// forgotten version. Opening the store again must baseline it, because
// CREATE TABLE IF NOT EXISTS will not, and because no ALTER is replayed on this
// engine.
//
// It drives the whole of lateColumns rather than one column: that list is what
// carries a PostgreSQL database created before #337 up to version 1, which is
// why the baseline still runs it and why nothing may be added to it.
func TestPostgresBaselinesADatabaseWrittenByAnOlderBinary(t *testing.T) {
	d := openPostgres(t)

	for _, column := range lateColumns {
		if _, err := d.conn.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", column.table, column.name)); err != nil {
			t.Fatalf("dropping %s.%s to simulate an older database: %v", column.table, column.name, err)
		}
	}
	forgetSchemaVersion(t, d)
	// The database is shared with every other test in this package, so it is
	// left complete whatever this one concludes.
	t.Cleanup(func() {
		for _, column := range lateColumns {
			_, _ = d.conn.Exec(column.addStatement())
		}
	})

	again, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("opening the upgraded database: %v", err)
	}
	defer again.Close()

	for _, column := range lateColumns {
		columns, err := columnNames(again, column.table)
		if err != nil {
			t.Fatalf("reading the columns of %s: %v", column.table, err)
		}
		if !slices.Contains(columns, column.name) {
			t.Errorf("%s.%s is still missing after the baseline: the schema declares it, so it also belongs in lateColumns",
				column.table, column.name)
		}
	}
	if version, err := again.schemaVersion(); err != nil || version != latestVersion() {
		t.Fatalf("version after the baseline = %d (%v), want %d", version, err, latestVersion())
	}

	// And the write path that reported the missing column in the first place:
	// blocking an account is the statement that failed with
	// `column "blocked_at" does not exist`.
	seedProjectAndUser(t, again)
	user, err := again.SetUserBlocked("u1", true)
	if err != nil {
		t.Fatalf("SetUserBlocked on the upgraded database: %v", err)
	}
	if !user.Blocked || user.BlockedAt == nil {
		t.Fatalf("the account did not come back blocked: %+v", user)
	}
}

// TestPostgresRecoversInterruptedRuns is the engine that never recovered them.
//
// The two statements that close work interrupted by a restart used to sit
// inside applyLegacyMigrations, which never runs here, so a PostgreSQL
// deployment left its activities `running` for good. They are per-start
// recovery, not a migration, and this is the test that says so on the engine
// that proves it.
func TestPostgresRecoversInterruptedRuns(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)
	seedTask(t, d)

	// A skill the server itself was running, and a remote run owned by a client
	// session the restart destroyed. They end differently on purpose. A client
	// declared the second one, so it is concurrent and may share the task.
	if _, err := d.conn.Exec(
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, concurrent)
		 VALUES ('a-local', 't1', 'implement', 'Implement', 'run', 'running', 0),
		        ('a-remote', 't1', 'remote_run', 'Remote', 'client', 'running', 1)`); err != nil {
		t.Fatalf("inserting the interrupted runs: %v", err)
	}

	again, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("restarting: %v", err)
	}
	defer again.Close()

	for _, want := range []struct{ id, status string }{
		{"a-local", "failed"},
		{"a-remote", "canceled"},
	} {
		var status string
		if err := again.conn.QueryRow(`SELECT status FROM task_activities WHERE id = ?`, want.id).Scan(&status); err != nil {
			t.Fatalf("reading %s back: %v", want.id, err)
		}
		if status != want.status {
			t.Errorf("%s: status = %q after a restart, want %q", want.id, status, want.status)
		}
	}
}
