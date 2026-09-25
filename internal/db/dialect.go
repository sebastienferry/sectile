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
	// FromEnvironment says the PostgreSQL connection comes from the standard
	// libpq variables (PGHOST, PGDATABASE, PGUSER, PGPASSWORD, ...) rather than
	// from a DSN. It is explicit rather than inferred from an empty DSN: pgx
	// falls back to those variables on its own, and an unconfigured server
	// would then quietly dial localhost instead of refusing to start.
	FromEnvironment bool
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
		if strings.TrimSpace(c.DSN) == "" && !c.FromEnvironment {
			return fmt.Errorf("driver %q needs a connection: set DATABASE_URL, or the standard PGHOST/PGDATABASE/PGUSER/PGPASSWORD variables", DriverPostgres)
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
	// LowerASCII wraps a TEXT expression in a case fold that lowers ASCII
	// letters and leaves every other character alone. Plain LOWER will not do:
	// under PostgreSQL it follows the database's collation, so the same query
	// folds `É` to `é` on one cluster and not on the next, and a comparison
	// against a value folded in Go then matches on one server only.
	LowerASCII(expr string) string
	// FoldSearch wraps a TEXT expression in the fold used by free-text search:
	// case and, where the engine can, diacritics. Both the column and the
	// pattern go through it, so they fold the same characters.
	FoldSearch(expr string) string
	// ColumnsQuery returns a one-argument query listing a table's column names,
	// in declaration order. The catalogue is the one thing every engine spells
	// entirely differently.
	ColumnsQuery() string
	// RewriteDDL adapts a schema statement to the engine. The schema is written
	// as literal SQL in SQLite's spelling, and only its type names differ: the
	// whole schema uses four types, two of which PostgreSQL does not have. A
	// statement that is not DDL is returned unchanged.
	RewriteDDL(stmt string) string
	// SecretKeyDir is the directory that may hold the generated encryption key.
	// An empty string means the engine has no such directory and the operator
	// supplies the key through the environment or does without it.
	SecretKeyDir(cfg Config) string
	// MigrateActivityAttachment brings an existing task_activities table to the
	// schema where task_id is nullable and foreign-keyed, project_id carries a
	// project activity, and no row may hold both. It is a no-op on a table
	// already at that schema, and on a database created today.
	//
	// It takes the backfill rather than running before or after it because the
	// two engines need it at different moments: SQLite rebuilds the table and
	// then cleans it, PostgreSQL has to clean between relaxing task_id and
	// creating the foreign key, which no engine accepts over dirty data. The
	// backfill itself stays engine-independent.
	MigrateActivityAttachment(conn *sqlConn, backfill func(*sqlConn) error) error
	// Engine names the engine itself, for the rare piece of code that must
	// choose a statement by engine rather than by capability: a numbered
	// migration the two cannot express the same way. Everything else asks a
	// question about behaviour instead, which is why this arrived last.
	Engine() Driver
	// LockForMigration serialises the migration run against another process
	// holding the same database, and returns the release. One server instance
	// per database is the invariant, and a rolling deploy still overlaps two of
	// them for a few seconds, which is exactly when both would migrate.
	LockForMigration(conn *sqlConn) (func(), error)
	// RunsLegacyMigrations reports whether the repairs written for databases
	// created by older versions apply. They only ever applied to SQLite files
	// that predate a schema change; a database created today starts complete.
	RunsLegacyMigrations() bool
	// ServesOneProcess reports whether the engine can only be shared by one
	// server process. When it is true, whatever an earlier process left running
	// died with it, so a start may reclaim all unfinished work at once; when it
	// is false, other live instances may own some of it. See docs/adrs/0016 and
	// internal/db/instances.go.
	ServesOneProcess() bool
	// ForUpdate is the clause that locks the rows a SELECT returns until the
	// transaction ends, or nothing on an engine whose writers are already
	// serialised. It is what lets a read-decide-write sequence hold across
	// server processes, where DB.mu stops at the process boundary. See
	// docs/db-concurrency-audit.md.
	ForUpdate() string
	// AcquireProjectWorker waits until no other server process runs a
	// server-side job for the project, and returns the release. It never holds a
	// database connection while waiting. An engine serving one process has
	// nothing to take: the in-process ProjectLimiter already decides.
	AcquireProjectWorker(conn *sqlConn, projectID string) (func(), error)
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

// EngineName is what the running engine is called, for the startup banner and
// for an error that needs to say which store it could not reach.
func (d *DB) EngineName() string {
	if d == nil || d.dialect == nil {
		return string(DriverSQLite)
	}
	return d.dialect.Name()
}
