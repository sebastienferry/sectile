package db

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand"
	"time"

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

func (p postgresDialect) Open(cfg Config) (*sql.DB, error) {
	connConfig, err := p.connConfig(cfg)
	if err != nil {
		return nil, err
	}
	conn := stdlib.OpenDB(*connConfig)
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	return conn, nil
}

// connConfig is the connection every PostgreSQL connection of this process is
// made from: the pool's, and the one the event bus listens on, so both reach
// the same server as the same user.
func (postgresDialect) connConfig(cfg Config) (*pgx.ConnConfig, error) {
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
	return connConfig, nil
}

func (postgresDialect) Rebind(query string) string { return rebindNumbered(query) }

// LowerASCII lowers under the C collation rather than the database's own.
//
// LOWER follows the collation of its argument, and that collation is a property
// of the cluster, not of PostgreSQL: one created with `initdb --locale=C` leaves
// `É` alone, one using an ICU or builtin C.UTF-8 locale folds it to `é`. A
// predicate comparing the column against a value folded in Go therefore matched
// on one server and not on the next. `COLLATE "C"` is built in, exists in every
// database whatever its encoding, and folds ASCII letters only — which is
// exactly what SQLite's LOWER does, so both engines mean the same thing.
func (postgresDialect) LowerASCII(expr string) string { return `LOWER(` + expr + ` COLLATE "C")` }

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

// ServesOneProcess is false: several server instances may share one PostgreSQL
// database, so a start only reclaims the work of instances that are gone.
func (postgresDialect) ServesOneProcess() bool { return false }

// ForUpdate locks the selected rows until the transaction ends. Under the
// default READ COMMITTED isolation, a second transaction selecting the same row
// FOR UPDATE waits, then reads the version the first one committed.
func (postgresDialect) ForUpdate() string { return " FOR UPDATE" }

// projectWorkerLockClass is the first half of the two-key advisory lock that
// stands for "a server-side job of this project is running". PostgreSQL keeps
// the two-int4 key space apart from the single-bigint one migrationLockKey
// lives in, so the two can never collide.
const projectWorkerLockClass int32 = 0x5EC7

// projectWorkerRetry is how long a job waits before asking again for its
// project's advisory lock. A variable so the tests can shorten it.
var projectWorkerRetry = 500 * time.Millisecond

// projectWorkerSlots caps the connections this process reserves for project
// worker locks. Each running job holds one for its whole duration, and its own
// queries need others from the same pool of 25: left uncapped, 25 projects
// served at once would hold the whole pool and wait on it forever.
var projectWorkerSlots = make(chan struct{}, 8)

// projectWorkerKey hashes a project id onto the second half of the lock key.
// Two projects sharing a hash only run their jobs one after the other, which
// is harmless.
func projectWorkerKey(projectID string) int32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(projectID))
	return int32(h.Sum32())
}

// AcquireProjectWorker takes the project's session advisory lock on a
// connection reserved for it, as LockForMigration does and for the same reason:
// the lock belongs to the connection that took it.
//
// It asks with pg_try_advisory_lock and gives the connection back between two
// attempts, so a job waiting for its turn never pins one of the pool's
// connections. If the held connection dies mid-job PostgreSQL releases the lock
// on its own; the release then only logs.
func (postgresDialect) AcquireProjectWorker(conn *sqlConn, projectID string) (func(), error) {
	ctx := context.Background()
	key := projectWorkerKey(projectID)
	projectWorkerSlots <- struct{}{}
	for {
		held, err := conn.db.Conn(ctx)
		if err != nil {
			<-projectWorkerSlots
			return func() {}, err
		}
		var acquired bool
		if err := held.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1, $2)", projectWorkerLockClass, key).Scan(&acquired); err != nil {
			held.Close()
			<-projectWorkerSlots
			return func() {}, err
		}
		if acquired {
			return func() {
				if _, err := held.ExecContext(ctx, "SELECT pg_advisory_unlock($1, $2)", projectWorkerLockClass, key); err != nil {
					log.Printf("[skill] releasing the worker lock of project %s: %v", projectID, err)
				}
				held.Close()
				<-projectWorkerSlots
			}, nil
		}
		held.Close()
		// Up to a fifth either way, so replicas waiting on one project do not
		// ask in step.
		jitter := time.Duration(rand.Int63n(int64(projectWorkerRetry)/5*2+1)) - projectWorkerRetry/5
		time.Sleep(projectWorkerRetry + jitter)
	}
}

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
