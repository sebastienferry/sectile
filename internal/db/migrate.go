package db

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"tasks/internal/secrets"
)

// migrationTables lists what a migration copies, parents before children.
//
// The order matters because the destination declares foreign keys: a task
// inserted before its project is rejected. It is written out rather than
// discovered, so that adding a table forces someone to decide where it belongs
// instead of finding out in production.
//
// daily_digests is absent on purpose: it is retired storage that startup drops.
// server_instances too: it describes the processes serving a database, and
// none of the source's processes serves the destination.
var migrationTables = []string{
	"settings",
	"users",
	"user_settings",
	"projects",
	"tasks",
	"task_activities",
	"task_comments",
	"teams",
	"team_members",
	"macros",
	"pinned_tasks",
	"project_skills",
	"user_project_bookmarks",
	"board_views",
	"user_tracker_credentials",
	"device_credentials",
	"pairing_codes",
	"login_flows",
	"web_sessions",
}

// TableCount is how many rows one table contributed to a migration.
type TableCount struct {
	Table string
	Rows  int
}

// Migrate copies a SQLite database into an already-initialised PostgreSQL one.
//
// It is deliberately one-way and deliberately explicit: an automatic import on
// first start would turn a configuration mistake into a data movement, and
// would give nobody the chance to check that the encryption key came across.
//
// The source is only ever read.
func Migrate(src *DB, dst *DB, progress io.Writer) ([]TableCount, error) {
	if err := ensureDestinationEmpty(dst); err != nil {
		return nil, err
	}
	if err := ensureKeyOpensCredentials(src, dst); err != nil {
		return nil, err
	}

	var counts []TableCount
	for _, table := range migrationTables {
		n, err := copyTable(src, dst, table)
		if err != nil {
			// The destination is left as it is rather than half-cleaned: naming
			// the table that failed lets an operator look, and a destination
			// that must be empty to start with is easy to drop and recreate.
			return counts, fmt.Errorf("copying %s: %w", table, err)
		}
		counts = append(counts, TableCount{Table: table, Rows: n})
		if progress != nil {
			fmt.Fprintf(progress, "  %-26s %6d row(s)\n", table, n)
		}
	}
	return counts, nil
}

// ensureDestinationEmpty refuses to write into a database that already holds
// something, so that running the migration twice, or against the wrong target,
// cannot duplicate or destroy anything.
func ensureDestinationEmpty(dst *DB) error {
	for _, table := range migrationTables {
		var n int
		if err := dst.conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			return fmt.Errorf("reading %s in the destination: %w", table, err)
		}
		// settings and projects are seeded on first start, so their presence is
		// not evidence of a database in use; anything else is.
		if n > 0 && table != "settings" && table != "projects" {
			return fmt.Errorf("the destination database is not empty: %s already holds %d row(s)", table, n)
		}
	}
	// The seeded rows still have to go, or the copy collides with them.
	for _, table := range []string{"projects", "settings"} {
		if _, err := dst.conn.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("clearing the seeded %s: %w", table, err)
		}
	}
	return nil
}

// ensureKeyOpensCredentials checks, before a single row moves, that sealed
// credentials will still be readable at the destination.
//
// A sealed token is bound to its owner and its tracker, never to the database,
// so the rows themselves copy perfectly well under the wrong key — and nobody
// finds out until someone's tracker call fails, weeks later. Checking here
// turns that into a refusal now.
func ensureKeyOpensCredentials(src, dst *DB) error {
	var userID, tracker string
	var record []byte
	err := src.conn.QueryRow(
		`SELECT user_id, tracker, record FROM user_tracker_credentials WHERE sealed = 0 LIMIT 1`,
	).Scan(&userID, &tracker, &record)
	if err == sql.ErrNoRows {
		// Nothing is sealed under the server key, so the key is not needed.
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading a credential to check the encryption key: %w", err)
	}
	if dst.serverKeyErr != nil {
		return fmt.Errorf(
			"the destination has no encryption key (%w), but the source holds credentials sealed under one; "+
				"set %s to the same key the source uses, or those tokens become unreadable",
			dst.serverKeyErr, secrets.KeyEnvVar)
	}
	if _, err := secrets.Open(dst.serverKey, secrets.Binding{UserID: userID, Tracker: tracker}, record); err != nil {
		return fmt.Errorf(
			"the destination encryption key does not open the source's credentials (%w); "+
				"set %s to the key the source uses, or those tokens become unreadable",
			err, secrets.KeyEnvVar)
	}
	return nil
}

// copyTable moves one table, column by column as the destination declares them.
//
// The destination schema is the authority: a column the source has and the
// destination does not is storage the current version no longer reads, and
// carrying it would fail the insert.
func copyTable(src, dst *DB, table string) (int, error) {
	dstCols, err := columnNames(dst, table)
	if err != nil {
		return 0, err
	}
	srcCols, err := columnNames(src, table)
	if err != nil {
		return 0, err
	}
	var cols []string
	for _, c := range dstCols {
		for _, s := range srcCols {
			if c == s {
				cols = append(cols, c)
				break
			}
		}
	}
	if len(cols) == 0 {
		return 0, fmt.Errorf("no column in common between the source and the destination")
	}

	quoted := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = `"` + c + `"`
		placeholders[i] = "?"
	}
	selectSQL := fmt.Sprintf("SELECT %s FROM %s", strings.Join(quoted, ", "), table)
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(quoted, ", "), strings.Join(placeholders, ", "))

	rows, err := src.conn.Query(selectSQL)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	tx, err := dst.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	n := 0
	for rows.Next() {
		values := make([]any, len(cols))
		targets := make([]any, len(cols))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return n, err
		}
		if _, err := tx.Exec(insertSQL, values...); err != nil {
			return n, err
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return n, err
	}
	if err := tx.Commit(); err != nil {
		return n, err
	}
	return n, nil
}

// columnNames reads a table's columns from the engine that holds it.
func columnNames(d *DB, table string) ([]string, error) {
	rows, err := d.conn.Query(d.dialect.ColumnsQuery(), table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
