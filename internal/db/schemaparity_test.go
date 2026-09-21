package db

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// columnsPerTable reads the live schema: table name to its sorted column names.
func columnsPerTable(t *testing.T, d *DB) map[string][]string {
	t.Helper()
	rows, err := d.conn.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("reading table list: %v", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning table name: %v", err)
		}
		tables = append(tables, name)
	}
	rows.Close()

	out := make(map[string][]string, len(tables))
	for _, table := range tables {
		cols, err := d.conn.Query(fmt.Sprintf("SELECT name FROM pragma_table_info(%q)", table))
		if err != nil {
			t.Fatalf("reading columns of %s: %v", table, err)
		}
		var names []string
		for cols.Next() {
			var name string
			if err := cols.Scan(&name); err != nil {
				t.Fatalf("scanning column of %s: %v", table, err)
			}
			names = append(names, name)
		}
		cols.Close()
		sort.Strings(names)
		out[table] = names
	}
	return out
}

// TestSchemaIsCompleteWithoutLegacyMigrations is the guard on the riskiest part
// of dual-engine support.
//
// PostgreSQL never runs the historical ALTER TABLE statements, so every column
// they add must also appear in the CREATE TABLE that declares its table.
// Forgetting one is invisible on SQLite — the ALTER path puts it back — and
// ships a PostgreSQL database silently missing a column. This test creates the
// schema both ways and requires them to match.
func TestSchemaIsCompleteWithoutLegacyMigrations(t *testing.T) {
	withLegacy, err := NewDB(filepath.Join(t.TempDir(), "with.db"))
	if err != nil {
		t.Fatalf("opening the reference database: %v", err)
	}
	defer withLegacy.Close()

	withoutLegacy, err := openWith(SQLiteConfig(filepath.Join(t.TempDir(), "without.db")), sqliteNoLegacy{})
	if err != nil {
		t.Fatalf("opening the database without legacy migrations: %v", err)
	}
	defer withoutLegacy.Close()

	want := columnsPerTable(t, withLegacy)
	got := columnsPerTable(t, withoutLegacy)

	for table, wantCols := range want {
		gotCols, ok := got[table]
		if !ok {
			t.Errorf("table %q is missing when the legacy migrations do not run: declare it in a CREATE TABLE", table)
			continue
		}
		missing := difference(wantCols, gotCols)
		if len(missing) > 0 {
			t.Errorf("table %q is missing %d column(s) without the legacy migrations: %s\n"+
				"add them to the CREATE TABLE for %q; an ALTER TABLE alone never reaches PostgreSQL",
				table, len(missing), strings.Join(missing, ", "), table)
		}
		if extra := difference(gotCols, wantCols); len(extra) > 0 {
			t.Errorf("table %q has %d unexpected column(s) without the legacy migrations: %s",
				table, len(extra), strings.Join(extra, ", "))
		}
	}
	for table := range got {
		if _, ok := want[table]; !ok {
			t.Errorf("table %q exists only without the legacy migrations", table)
		}
	}
}

func difference(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	var out []string
	for _, v := range a {
		if _, ok := set[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}
