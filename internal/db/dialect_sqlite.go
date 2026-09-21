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

// RunsLegacyMigrations is true: a SQLite file may have been created by any
// earlier version, and the additive migrations are what bring it up to date.
func (sqliteDialect) RunsLegacyMigrations() bool { return true }
