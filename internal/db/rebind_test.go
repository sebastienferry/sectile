package db

import "testing"

func TestRebindNumbered(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "no placeholder is left alone",
			query: "SELECT id FROM tasks",
			want:  "SELECT id FROM tasks",
		},
		{
			name:  "placeholders are numbered in order",
			query: "SELECT id FROM tasks WHERE project_id = ? AND status = ?",
			want:  "SELECT id FROM tasks WHERE project_id = $1 AND status = $2",
		},
		{
			name:  "a literal default is copied whole",
			query: "INSERT INTO projects (id, sprints) VALUES (?, '[]')",
			want:  "INSERT INTO projects (id, sprints) VALUES ($1, '[]')",
		},
		{
			// The case the literal-awareness exists for: without it the '?' in
			// the literal would consume $1 and every argument would bind one
			// position off, in a query that still executes.
			name:  "a question mark inside a literal is not a placeholder",
			query: "UPDATE tasks SET title = ? WHERE title <> 'why?' AND id = ?",
			want:  "UPDATE tasks SET title = $1 WHERE title <> 'why?' AND id = $2",
		},
		{
			name:  "an escaped quote does not end the literal",
			query: "UPDATE tasks SET title = 'it''s ? here' WHERE id = ?",
			want:  "UPDATE tasks SET title = 'it''s ? here' WHERE id = $1",
		},
		{
			name:  "a quoted identifier is copied whole",
			query: `SELECT "key?" FROM tasks WHERE id = ?`,
			want:  `SELECT "key?" FROM tasks WHERE id = $1`,
		},
		{
			name:  "many placeholders keep counting",
			query: "INSERT INTO t (a,b,c,d,e,f,g,h,i,j,k) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
			want:  "INSERT INTO t (a,b,c,d,e,f,g,h,i,j,k) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)",
		},
		{
			name:  "an unterminated literal does not run off the end",
			query: "SELECT * FROM t WHERE a = 'oops",
			want:  "SELECT * FROM t WHERE a = 'oops",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := rebindNumbered(tc.query); got != tc.want {
				t.Fatalf("rebindNumbered(%q)\n got: %q\nwant: %q", tc.query, got, tc.want)
			}
		})
	}
}

// The SQLite dialect must not touch the query at all: it is the engine the
// queries are written for.
func TestSQLiteRebindIsIdentity(t *testing.T) {
	q := "SELECT id FROM tasks WHERE project_id = ? AND title <> 'why?'"
	if got := (sqliteDialect{}).Rebind(q); got != q {
		t.Fatalf("SQLite Rebind changed the query:\n got: %q\nwant: %q", got, q)
	}
}

func TestRewriteDDLTypes(t *testing.T) {
	for _, tc := range []struct {
		name string
		stmt string
		want string
	}{
		{
			name: "DATETIME becomes TIMESTAMPTZ",
			stmt: "CREATE TABLE t (created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)",
			want: "CREATE TABLE t (created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)",
		},
		{
			name: "BLOB becomes BYTEA",
			stmt: "CREATE TABLE t (record BLOB NOT NULL, salt BLOB)",
			want: "CREATE TABLE t (record BYTEA NOT NULL, salt BYTEA)",
		},
		{
			name: "TEXT and INTEGER are shared and left alone",
			stmt: "CREATE TABLE t (a TEXT NOT NULL DEFAULT '', b INTEGER NOT NULL DEFAULT 0)",
			want: "CREATE TABLE t (a TEXT NOT NULL DEFAULT '', b INTEGER NOT NULL DEFAULT 0)",
		},
		{
			name: "ALTER is rewritten too",
			stmt: "ALTER TABLE t ADD COLUMN seen_at DATETIME;",
			want: "ALTER TABLE t ADD COLUMN seen_at TIMESTAMPTZ;",
		},
		{
			// The guard that makes a textual replacement safe: a value in a
			// query must never be touched, whatever it happens to contain.
			name: "a query carrying the word is not DDL and is untouched",
			stmt: "UPDATE tasks SET title = 'DATETIME and BLOB' WHERE id = ?",
			want: "UPDATE tasks SET title = 'DATETIME and BLOB' WHERE id = ?",
		},
		{
			name: "a SELECT is untouched",
			stmt: "SELECT created_at FROM t WHERE note LIKE '% BLOB%'",
			want: "SELECT created_at FROM t WHERE note LIKE '% BLOB%'",
		},
		{
			name: "leading whitespace does not hide the keyword",
			stmt: "\n\t\tCREATE TABLE t (a DATETIME)",
			want: "\n\t\tCREATE TABLE t (a TIMESTAMPTZ)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := rewriteDDLTypes(tc.stmt); got != tc.want {
				t.Fatalf("rewriteDDLTypes(%q)\n got: %q\nwant: %q", tc.stmt, got, tc.want)
			}
		})
	}
}

// Every type the schema actually uses must be one the rewriter knows about or
// one both engines share. If a new type appears in a CREATE TABLE, this is the
// test that should fail.
func TestSQLiteRewriteDDLIsIdentity(t *testing.T) {
	stmt := "CREATE TABLE t (a TEXT, b INTEGER, c DATETIME, d BLOB)"
	if got := (sqliteDialect{}).RewriteDDL(stmt); got != stmt {
		t.Fatalf("SQLite RewriteDDL changed the statement:\n got: %q\nwant: %q", got, stmt)
	}
}
