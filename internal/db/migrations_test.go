package db

import (
	"path/filepath"
	"slices"
	"testing"

	"tasks/internal/models"
)

// forgetSchemaVersion makes a database look like one written before versions
// existed, by removing every row that says which ones it has applied.
//
// The fixtures that need it all build a legacy shape by hand on a database this
// binary has already created, and creating it is what stamps the baseline. A
// database genuinely written by an earlier binary carries no version at all, and
// that is the state those tests mean to put under test: without this, the
// reopen skips the baseline and the repair they are checking never runs.
func forgetSchemaVersion(t *testing.T, d *DB) {
	t.Helper()
	if _, err := d.conn.Exec("DELETE FROM schema_migrations"); err != nil {
		t.Fatalf("forgetting the schema version: %v", err)
	}
	// A database genuinely written by an earlier binary carries no post-baseline
	// migrations. Drop columns added by migrations so that reopen can replay them.
	_, _ = d.conn.Exec("ALTER TABLE tasks DROP COLUMN creator")
	_, _ = d.conn.Exec("ALTER TABLE tasks DROP COLUMN creator_avatar")
	_, _ = d.conn.Exec("ALTER TABLE projects DROP COLUMN epic_colors")
	_, _ = d.conn.Exec("ALTER TABLE projects DROP COLUMN enabled_views")
	_, _ = d.conn.Exec("ALTER TABLE task_activities DROP COLUMN instance_id")
	_, _ = d.conn.Exec("DROP TABLE server_instances")
	_, _ = d.conn.Exec("DROP TABLE auto_sync_projects")
	_, _ = d.conn.Exec("DROP TABLE auto_sync_state")
	_, _ = d.conn.Exec("DROP TABLE agent_presence")
	_, _ = d.conn.Exec("ALTER TABLE server_instances DROP COLUMN address")
	_, _ = d.conn.Exec("DROP INDEX idx_activities_one_active_run")
	_, _ = d.conn.Exec("ALTER TABLE task_activities DROP COLUMN concurrent")
	_, _ = d.conn.Exec("DROP INDEX IF EXISTS idx_task_activities_macro_running")
	_, _ = d.conn.Exec("DROP INDEX IF EXISTS idx_task_activities_macro")
	_, _ = d.conn.Exec("ALTER TABLE task_activities DROP COLUMN macro_key")
	_, _ = d.conn.Exec("ALTER TABLE projects DROP COLUMN roadmap_projects")
	_, _ = d.conn.Exec("ALTER TABLE web_sessions DROP COLUMN last_seen_at")
	_, _ = d.conn.Exec("DROP TABLE server_tracker_credentials")
	// And it still carries the columns a migration since dropped.
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN github_token TEXT NOT NULL DEFAULT ''")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN gitlab_token TEXT NOT NULL DEFAULT ''")
	dropRepositoryColumns(d)
}

// dropRepositoryColumns removes what migrations 17 to 21 and 23 add, for the
// tests that put a database back before them and reopen it.
func dropRepositoryColumns(d *DB) {
	_, _ = d.conn.Exec("ALTER TABLE projects DROP COLUMN repositories")
	_, _ = d.conn.Exec("ALTER TABLE projects DROP COLUMN repositories_migration")
	_, _ = d.conn.Exec("ALTER TABLE tasks DROP COLUMN repository")
	_, _ = d.conn.Exec("ALTER TABLE tasks DROP COLUMN changed_repositories")
	_, _ = d.conn.Exec("ALTER TABLE task_activities DROP COLUMN waiting_reason")
	_, _ = d.conn.Exec("ALTER TABLE task_activities DROP COLUMN waiting_session")
	dropCredentialAccountColumn(d)
}

// dropCredentialAccountColumn removes what migrations 24 and 25 add. It runs
// with dropRepositoryColumns, since every fixture that rewinds before 21 also
// rewinds before 24.
func dropCredentialAccountColumn(d *DB) {
	_, _ = d.conn.Exec("ALTER TABLE user_tracker_credentials DROP COLUMN account")
	_, _ = d.conn.Exec("ALTER TABLE user_tracker_credentials DROP COLUMN unlock_generation")
}

// undoServerCredentialsMigration puts back the schema migration 22 changed, for the
// fixtures that forget the versions after one before it and replay them.
func undoServerCredentialsMigration(t *testing.T, d *DB) {
	t.Helper()
	for _, stmt := range []string{
		"DROP TABLE server_tracker_credentials",
		"ALTER TABLE projects ADD COLUMN github_token TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE projects ADD COLUMN gitlab_token TEXT NOT NULL DEFAULT ''",
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// appliedVersions is what the database says it has applied, in order.
func appliedVersions(t *testing.T, d *DB) []int {
	t.Helper()
	rows, err := d.conn.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("reading schema_migrations: %v", err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scanning a version: %v", err)
		}
		out = append(out, v)
	}
	return out
}

// TestMigrationsAreNumberedInOrder guards the list itself: versions start after
// the baseline, rise strictly, and are never renumbered once merged, because a
// database decides what to run next by number alone.
func TestMigrationsAreNumberedInOrder(t *testing.T) {
	previous := baselineVersion
	for _, m := range migrations {
		if m.version <= previous {
			t.Errorf("migration %q has version %d, which does not follow %d", m.name, m.version, previous)
		}
		if m.name == "" {
			t.Errorf("migration %d has no name", m.version)
		}
		if len(m.statements) == 0 && len(m.sqlite) == 0 && len(m.postgres) == 0 {
			t.Errorf("migration %d (%s) has no statement", m.version, m.name)
		}
		previous = m.version
	}
}

// TestANewDatabaseIsStampedAtTheBaseline is the state every other case starts
// from: a database created today has applied the baseline, and says so.
func TestANewDatabaseIsStampedAtTheBaseline(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "new.db"))
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	defer d.Close()

	if got := appliedVersions(t, d); !slices.Contains(got, baselineVersion) {
		t.Fatalf("applied versions = %v, want the baseline %d among them", got, baselineVersion)
	}
	version, err := d.schemaVersion()
	if err != nil {
		t.Fatalf("schemaVersion: %v", err)
	}
	if want := latestVersion(); version != want {
		t.Fatalf("schema version = %d, want %d", version, want)
	}
}

// latestVersion is what a database fully migrated by this binary carries.
func latestVersion() int {
	version := baselineVersion
	for _, m := range migrations {
		if m.version > version {
			version = m.version
		}
	}
	return version
}

// TestAMigrationIsAppliedOnceAndRecorded is the property the scheme exists for.
// The migration is deliberately not idempotent: running it twice fails, which is
// what proves it ran once.
func TestAMigrationIsAppliedOnceAndRecorded(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "once.db"))
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	defer d.Close()

	startVersion := latestVersion()
	nextVersion := startVersion + 1
	list := []migration{{
		version:    nextVersion,
		name:       "test.marker",
		statements: []string{"ALTER TABLE tasks ADD COLUMN test_marker TEXT NOT NULL DEFAULT '';"},
	}}
	if err := d.applyMigrations(list, startVersion); err != nil {
		t.Fatalf("applying: %v", err)
	}
	version, err := d.schemaVersion()
	if err != nil || version != nextVersion {
		t.Fatalf("version = %d (%v), want %d", version, err, nextVersion)
	}

	// A second pass starts from the recorded version and therefore does nothing.
	// Were it to run the statement again, SQLite would refuse the duplicate
	// column and the error would surface here.
	if err := d.applyMigrations(list, version); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if got := appliedVersions(t, d); len(got) != 1+len(migrations)+1 {
		t.Fatalf("applied versions = %v, want exactly the baseline and migrations plus one", got)
	}
}

// TestAFailedMigrationLeavesThePreviousVersion is why the version row is written
// inside the migration's own transaction: a migration that stops halfway must
// leave nothing behind, neither its statements nor its number.
func TestAFailedMigrationLeavesThePreviousVersion(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "failed.db"))
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	defer d.Close()

	startVersion := latestVersion()
	nextVersion := startVersion + 1
	list := []migration{{
		version: nextVersion,
		name:    "test.halfway",
		statements: []string{
			"ALTER TABLE tasks ADD COLUMN test_first TEXT NOT NULL DEFAULT '';",
			"ALTER TABLE tasks ADD COLUMN test_first TEXT NOT NULL DEFAULT '';", // the same column twice: refused
		},
	}}
	if err := d.applyMigrations(list, startVersion); err == nil {
		t.Fatal("a migration whose second statement is refused reported success")
	}

	version, err := d.schemaVersion()
	if err != nil {
		t.Fatalf("schemaVersion: %v", err)
	}
	if version != startVersion {
		t.Fatalf("version = %d after a failed migration, want %d", version, startVersion)
	}
	columns, err := columnNames(d, "tasks")
	if err != nil {
		t.Fatalf("reading the columns of tasks: %v", err)
	}
	if slices.Contains(columns, "test_first") {
		t.Fatal("the first statement of the failed migration survived: the transaction did not roll back")
	}
}

// TestAnOlderDatabaseIsBaselinedRatherThanRebuilt covers the upgrade: a file
// written before versions existed keeps its data, gains the missing schema and
// is stamped.
func TestAnOlderDatabaseIsBaselinedRatherThanRebuilt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "older.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	if _, err := d.conn.Exec(
		`INSERT INTO projects (id, name, slug) VALUES ('p1', 'Kept', 'kept')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}
	forgetSchemaVersion(t, d)
	d.Close()

	reopened, err := NewDB(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer reopened.Close()

	var name string
	if err := reopened.conn.QueryRow(`SELECT name FROM projects WHERE id = ?`, "p1").Scan(&name); err != nil {
		t.Fatalf("the project did not survive the baseline: %v", err)
	}
	if name != "Kept" {
		t.Fatalf("name = %q, want %q", name, "Kept")
	}
	if version, err := reopened.schemaVersion(); err != nil || version != latestVersion() {
		t.Fatalf("version after the baseline = %d (%v), want %d", version, err, latestVersion())
	}
}

// TestRestartRecoveryRunsOnEveryStart holds the line the ADR draws: recovering
// interrupted work is not a migration. It ran once per database it would only
// ever close the runs of the first restart.
func TestRestartRecoveryRunsOnEveryStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	seedProjectAndUser(t, d)
	seedTask(t, d)
	d.Close()

	// Two restarts, each leaving a run behind. The database is already stamped,
	// so nothing here goes through the baseline.
	for pass := 1; pass <= 2; pass++ {
		opened, err := NewDB(path)
		if err != nil {
			t.Fatalf("pass %d: reopening: %v", pass, err)
		}
		if _, err := opened.conn.Exec(
			`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status)
			 VALUES (?, 't1', 'implement', 'Implement', 'run', 'running')`,
			"a"+string(rune('0'+pass))); err != nil {
			t.Fatalf("pass %d: inserting a running activity: %v", pass, err)
		}
		opened.Close()

		reopened, err := NewDB(path)
		if err != nil {
			t.Fatalf("pass %d: restarting: %v", pass, err)
		}
		var status string
		if err := reopened.conn.QueryRow(
			`SELECT status FROM task_activities WHERE id = ?`, "a"+string(rune('0'+pass))).Scan(&status); err != nil {
			t.Fatalf("pass %d: reading the activity back: %v", pass, err)
		}
		reopened.Close()
		if status != "failed" {
			t.Fatalf("pass %d: status = %q after a restart, want %q", pass, status, "failed")
		}
	}
}

// TestAStampedDatabaseStillGainsALaterColumn covers what shipped broken: a
// column added to the baseline CREATE TABLE instead of to a numbered migration
// reaches a database created from nothing and no other, because the baseline
// runs only while the database carries no version. Every database stamped
// beforehand went on without projects.enabled_views, and answered an error to
// every project read, and so broke the whole interface, which lists projects first.
//
// The check is the read the interface makes, not the column list: a column the
// schema has and the query does not name would pass a column check and fail
// here, which is the way round that matters.
func TestAStampedDatabaseStillGainsALaterColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stamped.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	if _, err := d.conn.Exec(
		`INSERT INTO projects (id, name, slug) VALUES ('p1', 'Kept', 'kept')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}
	// The database as an earlier binary left it: stamped, and short of the
	// column that binary knew nothing about.
	if _, err := d.conn.Exec("ALTER TABLE projects DROP COLUMN enabled_views"); err != nil {
		t.Fatalf("removing the column: %v", err)
	}
	// Nor anything the migrations after it add, which reopening replays.
	_, _ = d.conn.Exec("ALTER TABLE task_activities DROP COLUMN instance_id")
	_, _ = d.conn.Exec("DROP TABLE server_instances")
	_, _ = d.conn.Exec("DROP TABLE auto_sync_projects")
	_, _ = d.conn.Exec("DROP TABLE auto_sync_state")
	_, _ = d.conn.Exec("DROP TABLE agent_presence")
	_, _ = d.conn.Exec("DROP INDEX idx_activities_one_active_run")
	_, _ = d.conn.Exec("ALTER TABLE task_activities DROP COLUMN concurrent")
	_, _ = d.conn.Exec("DROP INDEX IF EXISTS idx_task_activities_macro_running")
	_, _ = d.conn.Exec("DROP INDEX IF EXISTS idx_task_activities_macro")
	_, _ = d.conn.Exec("ALTER TABLE task_activities DROP COLUMN macro_key")
	_, _ = d.conn.Exec("ALTER TABLE projects DROP COLUMN roadmap_projects")
	_, _ = d.conn.Exec("ALTER TABLE web_sessions DROP COLUMN last_seen_at")
	dropRepositoryColumns(d)
	undoServerCredentialsMigration(t, d)
	if _, err := d.conn.Exec("DELETE FROM schema_migrations WHERE version >= ?", 5); err != nil {
		t.Fatalf("forgetting the migration: %v", err)
	}
	d.Close()

	reopened, err := NewDB(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer reopened.Close()

	projects, err := reopened.GetProjects()
	if err != nil {
		t.Fatalf("reading the projects back: %v", err)
	}
	if !slices.ContainsFunc(projects, func(p models.Project) bool { return p.ID == "p1" }) {
		t.Fatalf("the seeded project did not survive: %d project(s) read, none of them p1", len(projects))
	}
}

// A database from before #443 carries a server specifications path. The
// upgrade drops the column with its values, which named a directory on the
// server, and the project reads back without it.
func TestMigrationSixteenDropsTheServerSpecificationsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	dropRepositoryColumns(d)
	for _, stmt := range []string{
		"ALTER TABLE projects ADD COLUMN spec_repo_path TEXT NOT NULL DEFAULT ''",
		`INSERT INTO projects (id, name, slug, spec_repo_path) VALUES ('p1', 'Kept', 'kept', '/server/wiki')`,
		"DELETE FROM schema_migrations WHERE version >= 16",
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	undoServerCredentialsMigration(t, d)
	d.Close()

	reopened, err := NewDB(path)
	if err != nil {
		t.Fatalf("upgrading: %v", err)
	}
	defer reopened.Close()
	if _, err := reopened.conn.Exec("SELECT spec_repo_path FROM projects"); err == nil {
		t.Fatal("the column must be gone")
	}
	project, err := reopened.GetProjectByID("p1")
	if err != nil || project == nil || project.Name != "Kept" {
		t.Fatalf("the project must survive the upgrade: %+v %v", project, err)
	}
}
