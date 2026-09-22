package db

import "database/sql"

// sqlConn wraps the pool so every query passes through the dialect's Rebind on
// its way out.
//
// The alternative was to rename all 335 call sites onto d.exec / d.query
// helpers. That is a mechanical edit across 27 files where one mistyped
// argument list still compiles and still runs, just against the wrong column.
// Wrapping the handle instead leaves every call site exactly as it was written
// and puts the whole engine difference in one place a reviewer can read.
//
// The method set is the part of *sql.DB this package uses, no more.
type sqlConn struct {
	db      *sql.DB
	dialect dialect
}

func newSQLConn(db *sql.DB, d dialect) *sqlConn { return &sqlConn{db: db, dialect: d} }

func (c *sqlConn) Exec(query string, args ...any) (sql.Result, error) {
	// RewriteDDL is a no-op on anything that is not a schema statement, so the
	// ordinary write path pays one prefix check.
	return c.db.Exec(c.dialect.Rebind(c.dialect.RewriteDDL(query)), args...)
}

func (c *sqlConn) Query(query string, args ...any) (*sql.Rows, error) {
	return c.db.Query(c.dialect.Rebind(query), args...)
}

func (c *sqlConn) QueryRow(query string, args ...any) *sql.Row {
	return c.db.QueryRow(c.dialect.Rebind(query), args...)
}

func (c *sqlConn) Begin() (*sqlTx, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx: tx, dialect: c.dialect}, nil
}

func (c *sqlConn) Close() error       { return c.db.Close() }
func (c *sqlConn) Stats() sql.DBStats { return c.db.Stats() }
func (c *sqlConn) Ping() error        { return c.db.Ping() }

// sqlTx is the same wrapping for a transaction in progress.
type sqlTx struct {
	tx      *sql.Tx
	dialect dialect
}

func (t *sqlTx) Exec(query string, args ...any) (sql.Result, error) {
	// RewriteDDL as on the connection: a numbered migration runs its schema
	// statements inside a transaction, and its DATETIME has to reach PostgreSQL
	// as TIMESTAMPTZ exactly as it does everywhere else.
	return t.tx.Exec(t.dialect.Rebind(t.dialect.RewriteDDL(query)), args...)
}

func (t *sqlTx) Query(query string, args ...any) (*sql.Rows, error) {
	return t.tx.Query(t.dialect.Rebind(query), args...)
}

func (t *sqlTx) QueryRow(query string, args ...any) *sql.Row {
	return t.tx.QueryRow(t.dialect.Rebind(query), args...)
}

func (t *sqlTx) Commit() error   { return t.tx.Commit() }
func (t *sqlTx) Rollback() error { return t.tx.Rollback() }

// Prepare rebinds once, at prepare time, which is the whole point of preparing:
// the statement is then executed many times with only its arguments changing.
func (t *sqlTx) Prepare(query string) (*sql.Stmt, error) {
	return t.tx.Prepare(t.dialect.Rebind(query))
}
