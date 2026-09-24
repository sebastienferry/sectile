package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// sqliteDialect reproduces, exactly, the behaviour this package had when SQLite
// was the only engine. Nothing here is new; it is the old code given a name so
// the PostgreSQL path can differ from it.
type sqliteDialect struct{}

func (sqliteDialect) Name() string { return "SQLite" }

// LowerASCII is SQLite's own LOWER: it lowers A-Z and leaves every other
// character untouched, on every build of the engine.
func (sqliteDialect) LowerASCII(expr string) string { return "LOWER(" + expr + ")" }

func (sqliteDialect) Open(cfg Config) (*sql.DB, error) {
	// _time_format=sqlite: without it the driver stores a time.Time as
	// time.Time.String(), which prints the zone abbreviation last. A date parsed
	// from a tracker carries an offset that rarely matches the server's own
	// zone, Go gives it a location with no name, and String() then writes the
	// numeric offset where the abbreviation belongs — a form the driver cannot
	// read back, so the Scan fails and the endpoint answers 500. The requested
	// format ends with the offset itself and round trips in any zone. See
	// repairNumericZoneTimestamps for the rows written before this was set.
	conn, err := sql.Open("sqlite", cfg.Path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_time_format=sqlite")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	return conn, nil
}

// Rebind is the identity: the queries in this package are written with SQLite's
// own "?" placeholders.
func (sqliteDialect) Rebind(query string) string { return query }

func (sqliteDialect) ColumnsQuery() string {
	return "SELECT name FROM pragma_table_info(?) ORDER BY cid"
}

// RewriteDDL is the identity: the schema is written in SQLite's own spelling.
func (sqliteDialect) RewriteDDL(stmt string) string { return stmt }

// SecretKeyDir is the database file's own directory, which is where the key has
// always been generated: an operator who backs up the database without it ends
// up with a copy that opens nothing, which ADR 0014 says is the intended shape.
func (sqliteDialect) SecretKeyDir(cfg Config) string { return filepath.Dir(cfg.Path) }

func (sqliteDialect) Engine() Driver { return DriverSQLite }

// LockForMigration has nothing to take. A SQLite database is opened by one
// process, and the connection already carries busy_timeout for the writes that
// cross within it.
func (sqliteDialect) LockForMigration(*sqlConn) (func(), error) { return func() {}, nil }

// RunsLegacyMigrations is true: a SQLite file may have been created by any
// earlier version, and the additive migrations are what bring it up to date.
func (sqliteDialect) RunsLegacyMigrations() bool { return true }

// ServesOneProcess is true: a SQLite file belongs to one server process. Nothing
// enforces it; it is the assumption the deployment makes, and the one the
// restart recovery relies on.
func (sqliteDialect) ServesOneProcess() bool { return true }

// MigrateActivityAttachment rebuilds task_activities: SQLite can neither relax
// a NOT NULL, nor add a foreign key, nor add a CHECK through ALTER TABLE.
//
// The copy runs before the backfill, the opposite of PostgreSQL's order, and it
// is safe here for the one reason that made the old convention survive so long:
// this package never turns foreign keys on, so SQLite accepts the "sync-<x>"
// rows into the new table and the backfill then cleans them in place. The CHECK
// is enforced from the start, and the copied rows satisfy it — none of them
// carries a project_id yet.
func (sqliteDialect) MigrateActivityAttachment(conn *sqlConn, backfill func(*sqlConn) error) error {
	migrated, err := hasColumn(conn, "task_activities", "project_id")
	if err != nil || migrated {
		return err
	}
	if _, err := conn.Exec(fmt.Sprintf(`
		%s
		INSERT INTO task_activities_new (%s) SELECT %s FROM task_activities;
		DROP TABLE task_activities;
		ALTER TABLE task_activities_new RENAME TO task_activities;
		CREATE INDEX IF NOT EXISTS idx_activities_task ON task_activities(task_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_activities_project ON task_activities(project_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_activities_status ON task_activities(status);
		CREATE INDEX IF NOT EXISTS idx_activities_created ON task_activities(created_at DESC);
	`, taskActivitiesSchema("task_activities_new"), taskActivityColumns, taskActivityColumns)); err != nil {
		return fmt.Errorf("rebuilding task_activities: %w", err)
	}
	return backfill(conn)
}
