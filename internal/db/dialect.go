package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// Driver names the engine backing the store. The zero value is SQLite, so a
// Config nobody filled in behaves exactly like the single-engine code that came
// before it.
type Driver string

const (
	DriverSQLite   Driver = "sqlite"
	DriverPostgres Driver = "postgres"
)

// Config says which engine to open and how to reach it. Path is the SQLite
// database file and is ignored under PostgreSQL; DSN is the PostgreSQL
// connection string and is ignored under SQLite. Keeping both on one struct
// rather than in an interface lets cmd/server read its environment once and
// hand the result over without knowing what the engine will do with it.
type Config struct {
	Driver Driver
	Path   string
	DSN    string
}

// SQLiteConfig builds the configuration the code used before this file existed:
// one file, no DSN. It is what NewDB's path-shaped helper passes, so the test
// call sites keep working unchanged.
func SQLiteConfig(path string) Config {
	return Config{Driver: DriverSQLite, Path: path}
}

// Validate reports a configuration that cannot open, before anything tries to.
// A PostgreSQL configuration with no DSN must fail here rather than fall back to
// SQLite: falling back would serve an empty board out of an unexpected store,
// which reads as data loss to whoever is looking at it.
func (c Config) Validate() error {
	switch c.Driver {
	case DriverSQLite, "":
		if strings.TrimSpace(c.Path) == "" {
			return fmt.Errorf("no SQLite database path")
		}
		return nil
	case DriverPostgres:
		if strings.TrimSpace(c.DSN) == "" {
			return fmt.Errorf("driver %q needs a connection string: set DATABASE_URL", DriverPostgres)
		}
		return nil
	default:
		return fmt.Errorf("unknown database driver %q: expected %q or %q", c.Driver, DriverSQLite, DriverPostgres)
	}
}

// dialect carries the engine-specific fragments, and nothing else. It stays
// unexported and deliberately small: every query, and all 210 methods of *DB,
// are shared. See docs/adrs/0016 for why the seam is here rather than above
// *DB.
type dialect interface {
	// Open dials the engine and returns a pool ready to use.
	Open(cfg Config) (*sql.DB, error)
	// Rebind turns the shared "?" placeholders into whatever the engine wants.
	Rebind(query string) string
	// RewriteDDL adapts a schema statement to the engine. The schema is written
	// as literal SQL in SQLite's spelling, and only its type names differ: the
	// whole schema uses four types, two of which PostgreSQL does not have. A
	// statement that is not DDL is returned unchanged.
	RewriteDDL(stmt string) string
	// SecretKeyDir is the directory that may hold the generated encryption key.
	// An empty string means the engine has no such directory and the operator
	// supplies the key through the environment or does without it.
	SecretKeyDir(cfg Config) string
	// RunsLegacyMigrations reports whether the repairs written for databases
	// created by older versions apply. They only ever applied to SQLite files
	// that predate a schema change; a database created today starts complete.
	RunsLegacyMigrations() bool
	// Name is what the startup log calls this engine.
	Name() string
}

// newDialect picks the dialect for a validated configuration.
func newDialect(cfg Config) (dialect, error) {
	switch cfg.Driver {
	case DriverSQLite, "":
		return sqliteDialect{}, nil
	case DriverPostgres:
		return postgresDialect{}, nil
	default:
		return nil, fmt.Errorf("unknown database driver %q", cfg.Driver)
	}
}

// sqliteNoLegacy is the SQLite dialect with the historical repairs turned off.
// It exists for one test: the schema a database gets without the legacy
// migrations must equal the schema it gets with them, because that equality is
// exactly what PostgreSQL relies on. Without the check, a column added by an
// ALTER and never backported to its CREATE TABLE ships a PostgreSQL schema that
// is quietly missing it, and every SQLite test stays green.
type sqliteNoLegacy struct{ sqliteDialect }

func (sqliteNoLegacy) RunsLegacyMigrations() bool { return false }
