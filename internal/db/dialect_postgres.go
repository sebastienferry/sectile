package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// postgresDialect backs the store with an external PostgreSQL server.
//
// Two things it deliberately does not do. It does not run the legacy
// migrations: those repair SQLite files written by older versions, and a
// PostgreSQL database is created complete. And it offers no directory for the
// encryption key: under PostgreSQL the operator supplies the key, because a
// container with no persistent storage would otherwise generate a fresh one on
// every restart and silently orphan every stored token.
type postgresDialect struct{}

func (postgresDialect) Name() string { return "PostgreSQL" }

func (postgresDialect) Open(cfg Config) (*sql.DB, error) {
	// An empty string is not a missing value here: pgx reads the standard libpq
	// variables for every field a DSN does not set, which is how a deployment
	// receives a username and a password as two separate secrets. Config.Validate
	// has already refused the case where neither source says anything.
	connConfig, err := pgx.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("unusable connection string: %w", err)
	}
	// The zone is set as a connection parameter rather than with a SET on the
	// open handle. A SET reaches exactly one pooled connection; every other one
	// the pool opens later would keep the server's zone, so CURRENT_TIMESTAMP
	// would land in UTC or not depending on which connection served the write.
	// As a parameter it is part of every connection this pool ever makes.
	if connConfig.RuntimeParams == nil {
		connConfig.RuntimeParams = map[string]string{}
	}
	connConfig.RuntimeParams["timezone"] = "UTC"

	conn := stdlib.OpenDB(*connConfig)
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	return conn, nil
}

func (postgresDialect) Rebind(query string) string { return rebindNumbered(query) }

func (postgresDialect) ColumnsQuery() string {
	return "SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? ORDER BY ordinal_position"
}

// RewriteDDL maps the two type names PostgreSQL does not share.
//
// The whole schema uses four types: TEXT and INTEGER, which both engines
// spell the same, plus DATETIME and BLOB, which PostgreSQL calls TIMESTAMPTZ
// and BYTEA. INTEGER is left alone on purpose, booleans included: PostgreSQL
// stores and returns it happily, and keeping the type identical leaves all 24
// boolean columns and every Scan site untouched (docs/adrs/0016).
func (postgresDialect) RewriteDDL(stmt string) string { return rewriteDDLTypes(stmt) }

// SecretKeyDir is empty: there is no database file to sit beside, and a
// generated key would differ on every restart of a stateless container.
func (postgresDialect) SecretKeyDir(Config) string { return "" }

func (postgresDialect) Engine() Driver { return DriverPostgres }

// migrationLockKey identifies the migration lock. Any constant does, as long as
// every version of Sectile uses the same one.
const migrationLockKey int64 = 0x5EC71

// LockForMigration takes a session advisory lock, and holds it on a connection
// reserved for that alone.
//
// The reservation is the point: an advisory lock belongs to the connection that
// took it, and a pool hands out whichever connection is free, so a lock taken
// through the pool would be released on a different connection than it was
// taken on, or not at all. The migrations themselves keep running on the pool.
func (postgresDialect) LockForMigration(conn *sqlConn) (func(), error) {
	ctx := context.Background()
	held, err := conn.db.Conn(ctx)
	if err != nil {
		return func() {}, err
	}
	if _, err := held.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		held.Close()
		return func() {}, err
	}
	return func() {
		if _, err := held.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey); err != nil {
			log.Printf("[schema] releasing the migration lock: %v", err)
		}
		held.Close()
	}, nil
}

func (postgresDialect) RunsLegacyMigrations() bool { return false }

// MigrateActivityAttachment walks an existing table to the current schema with
// ALTER TABLE, cleaning the data in the middle: PostgreSQL validates a foreign
// key against the rows already there, and every "sync-<x>" identifier would
// fail it, so the constraint can only be created once the backfill has run.
//
// The guard is the last constraint the migration creates, and every statement
// tolerates having run before. An attempt that stops halfway — the backfill
// failing, say — is therefore retried in full on the next start-up, instead of
// leaving a table that has project_id but never regained its foreign keys and
// that no later start would ever look at again.
func (postgresDialect) MigrateActivityAttachment(conn *sqlConn, backfill func(*sqlConn) error) error {
	migrated, err := hasConstraint(conn, "task_activities", "task_activities_project_id_fkey")
	if err != nil || migrated {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE task_activities ALTER COLUMN task_id DROP NOT NULL`,
		`ALTER TABLE task_activities ADD COLUMN IF NOT EXISTS project_id TEXT`,
		`ALTER TABLE task_activities DROP CONSTRAINT IF EXISTS task_activities_attachment_check`,
		`ALTER TABLE task_activities ADD CONSTRAINT task_activities_attachment_check CHECK (task_id IS NULL OR project_id IS NULL)`,
	} {
		if _, err := conn.Exec(stmt); err != nil {
			return fmt.Errorf("preparing task_activities: %w", err)
		}
	}
	if err := backfill(conn); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE task_activities DROP CONSTRAINT IF EXISTS task_activities_task_id_fkey`,
		`ALTER TABLE task_activities ADD CONSTRAINT task_activities_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id)`,
		`ALTER TABLE task_activities ADD CONSTRAINT task_activities_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE`,
	} {
		if _, err := conn.Exec(stmt); err != nil {
			return fmt.Errorf("restoring the foreign keys of task_activities: %w", err)
		}
	}
	return nil
}

// hasConstraint reports whether a table already carries a named constraint.
// PostgreSQL names the ones declared inline exactly as this migration names the
// ones it adds, so a database born at the current schema reads as migrated.
func hasConstraint(conn *sqlConn, table, name string) (bool, error) {
	var found int
	err := conn.QueryRow(
		`SELECT COUNT(*) FROM pg_constraint c JOIN pg_class t ON t.oid = c.conrelid
		  WHERE t.relname = ? AND c.conname = ?`, table, name).Scan(&found)
	if err != nil {
		return false, err
	}
	return found > 0, nil
}
