package db

import (
	"database/sql"
	"fmt"

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

func (postgresDialect) RunsLegacyMigrations() bool { return false }
