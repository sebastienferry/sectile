package db

import (
	"fmt"
	"slices"
	"testing"
)

// TestPostgresUpgradeRestoresLateColumns covers the one thing a schema built
// from scratch can never cover: the upgrade.
//
// A PostgreSQL database created by an earlier version keeps the schema it was
// born with, so the test has to put the database back into that state — there
// is no older schema to ask for, only a DROP. Opening the store again must then
// restore every column, because CREATE TABLE IF NOT EXISTS will not, and
// because no ALTER is ever replayed on this engine.
//
// It drives the whole of lateColumns rather than one column, so a column added
// to a CREATE TABLE and forgotten in the list fails here instead of failing in
// production, which is how #327's users.blocked_at reached a live deployment.
func TestPostgresUpgradeRestoresLateColumns(t *testing.T) {
	d := openPostgres(t)

	for _, column := range lateColumns {
		if _, err := d.conn.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", column.table, column.name)); err != nil {
			t.Fatalf("dropping %s.%s to simulate an older database: %v", column.table, column.name, err)
		}
	}
	// The database is shared with every other test in this package, so it is
	// left complete whatever this one concludes.
	t.Cleanup(func() {
		for _, column := range lateColumns {
			_, _ = d.conn.Exec(column.addStatement())
		}
	})

	again, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("opening the upgraded database: %v", err)
	}
	defer again.Close()

	for _, column := range lateColumns {
		columns, err := columnNames(again, column.table)
		if err != nil {
			t.Fatalf("reading the columns of %s: %v", column.table, err)
		}
		if !slices.Contains(columns, column.name) {
			t.Errorf("%s.%s is still missing after the upgrade: the schema declares it, so it also belongs in lateColumns",
				column.table, column.name)
		}
	}

	// And the write path that reported the missing column in the first place:
	// blocking an account is the statement that failed with
	// `column "blocked_at" does not exist`.
	seedProjectAndUser(t, again)
	user, err := again.SetUserBlocked("u1", true)
	if err != nil {
		t.Fatalf("SetUserBlocked on the upgraded database: %v", err)
	}
	if !user.Blocked || user.BlockedAt == nil {
		t.Fatalf("the account did not come back blocked: %+v", user)
	}
}
