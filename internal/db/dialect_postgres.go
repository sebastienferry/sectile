package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
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
	conn, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	// The session is pinned to UTC so CURRENT_TIMESTAMP means what it means on
	// SQLite. Without it the default lands in the server's zone and two
	// deployments disagree about when a row was written.
	if _, err := conn.Exec("SET TIME ZONE 'UTC'"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to pin the session to UTC: %w", err)
	}
	return conn, nil
}

func (postgresDialect) Rebind(query string) string { return rebindNumbered(query) }

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
