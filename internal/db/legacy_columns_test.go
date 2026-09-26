package db

import (
	"sort"
	"strings"
	"testing"
)

// setLegacyColumns writes columns no request writes any more (#305), the way
// a database written before the upgrade holds them: the seed and the server
// paths still read them.
func setLegacyColumns(t *testing.T, d *DB, table, where string, id any, columns map[string]any) {
	t.Helper()
	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names)
	assignments := make([]string, 0, len(names))
	args := make([]any, 0, len(names)+1)
	for _, name := range names {
		assignments = append(assignments, name+" = ?")
		args = append(args, columns[name])
	}
	args = append(args, id)
	if _, err := d.conn.Exec("UPDATE "+table+" SET "+strings.Join(assignments, ", ")+" WHERE "+where+" = ?", args...); err != nil {
		t.Fatal(err)
	}
}

func setLegacyProject(t *testing.T, d *DB, id string, columns map[string]any) {
	t.Helper()
	setLegacyColumns(t, d, "projects", "id", id, columns)
}

func setLegacySettings(t *testing.T, d *DB, columns map[string]any) {
	t.Helper()
	if _, err := d.GetSettings(); err != nil {
		t.Fatal(err)
	}
	setLegacyColumns(t, d, "settings", "id", 1, columns)
}
