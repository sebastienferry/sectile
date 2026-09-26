package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// baselineSnapshot is the schema the baseline builds on SQLite, one line per
// column and per index. It is written once and never regenerated: the baseline
// is frozen, so the only reason for this file to change is a mistake.
const baselineSnapshot = "testdata/baseline_schema.txt"

// openBaselineOnly builds a SQLite database with the baseline and nothing
// after it. openWith cannot, since it runs every numbered migration, so the
// store is assembled here from the same parts and only the baseline is applied.
func openBaselineOnly(t *testing.T) *DB {
	t.Helper()
	cfg := SQLiteConfig(filepath.Join(t.TempDir(), "baseline.db"))
	dialect, err := newDialect(cfg)
	if err != nil {
		t.Fatalf("choosing the dialect: %v", err)
	}
	conn, err := dialect.Open(cfg)
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	d := &DB{conn: newSQLConn(conn, dialect), dialect: dialect, cfg: cfg}
	if err := d.ensureSchemaMigrationsTable(); err != nil {
		t.Fatalf("creating schema_migrations: %v", err)
	}
	if err := d.applyBaseline(); err != nil {
		t.Fatalf("applying the baseline: %v", err)
	}
	return d
}

// baselineSchema describes every table's columns and every named index, sorted,
// in the form the snapshot keeps. schema_migrations belongs to the runner, not
// to the baseline, and sqlite_ objects belong to the engine.
func baselineSchema(t *testing.T, d *DB) []string {
	t.Helper()
	rows, err := d.conn.Query(`SELECT type, name, tbl_name FROM sqlite_master
		WHERE type IN ('table', 'index') AND name NOT LIKE 'sqlite_%' AND tbl_name <> 'schema_migrations'`)
	if err != nil {
		t.Fatalf("listing the schema: %v", err)
	}
	type object struct{ kind, name, table string }
	var objects []object
	for rows.Next() {
		var o object
		if err := rows.Scan(&o.kind, &o.name, &o.table); err != nil {
			t.Fatalf("reading the schema: %v", err)
		}
		objects = append(objects, o)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("listing the schema: %v", err)
	}

	var lines []string
	for _, o := range objects {
		if o.kind == "index" {
			lines = append(lines, fmt.Sprintf("index %s on %s", o.name, o.table))
			continue
		}
		columns, err := d.conn.Query(fmt.Sprintf("PRAGMA table_info(%q)", o.name))
		if err != nil {
			t.Fatalf("reading the columns of %s: %v", o.name, err)
		}
		for columns.Next() {
			var (
				position, notNull, primaryKey int
				name, kind                    string
				fallback                      sql.NullString
			)
			if err := columns.Scan(&position, &name, &kind, &notNull, &fallback, &primaryKey); err != nil {
				t.Fatalf("reading a column of %s: %v", o.name, err)
			}
			line := fmt.Sprintf("%s.%s %s", o.name, name, kind)
			if notNull == 1 {
				line += " NOT NULL"
			}
			if fallback.Valid {
				line += " DEFAULT " + fallback.String
			}
			if primaryKey > 0 {
				line += " PRIMARY KEY"
			}
			lines = append(lines, line)
		}
		if err := columns.Close(); err != nil {
			t.Fatalf("reading the columns of %s: %v", o.name, err)
		}
	}
	slices.Sort(lines)
	return lines
}

// TestTheBaselineSchemaIsFrozen guards ADR 0021 against the mistake of #411: a
// column added to a baseline CREATE TABLE reaches a database created from
// nothing and no other, since a database that already carries a version never
// runs the baseline again. projects.enabled_views shipped that way and broke
// every project read on every upgraded deployment.
//
// Nothing about the baseline may change, so the test compares it whole with a
// snapshot rather than checking any one column, and has no flag to rewrite
// the snapshot: a diff to that file in a pull request is the review signal.
func TestTheBaselineSchemaIsFrozen(t *testing.T) {
	want, err := os.ReadFile(baselineSnapshot)
	if err != nil {
		t.Fatalf("reading the snapshot: %v", err)
	}
	got := baselineSchema(t, openBaselineOnly(t))
	expected := strings.Split(strings.TrimSpace(string(want)), "\n")

	var added, removed []string
	for _, line := range got {
		if !slices.Contains(expected, line) {
			added = append(added, "+ "+line)
		}
	}
	for _, line := range expected {
		if !slices.Contains(got, line) {
			removed = append(removed, "- "+line)
		}
	}
	if len(added) == 0 && len(removed) == 0 {
		return
	}
	t.Fatalf("the baseline schema changed:\n%s\n\n"+
		"The baseline is frozen (docs/adrs/0021-the-schema-carries-its-version.md): a database that "+
		"already carries a version never runs it again, so whatever is added there reaches new "+
		"databases only. Revert the baseline edit and write a numbered migration in "+
		"internal/db/migrations.go instead.",
		strings.Join(append(removed, added...), "\n"))
}

// A numbered migration is outside the baseline, so the guard must not see it:
// the snapshot describes version 1 and nothing later.
func TestTheBaselineGuardIgnoresNumberedMigrations(t *testing.T) {
	d := openBaselineOnly(t)
	if version, err := d.schemaVersion(); err != nil || version != baselineVersion {
		t.Fatalf("version after the baseline alone = %d (%v), want %d", version, err, baselineVersion)
	}
	for _, line := range baselineSchema(t, d) {
		// Migration 5 adds this column: it must not be in the baseline.
		if strings.HasPrefix(line, "projects.enabled_views ") {
			t.Fatalf("the baseline builds %q, which belongs to migration 5", line)
		}
	}
}
