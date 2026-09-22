package db

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// sqliteTimeFormat is the layout the driver writes time.Time values with, as
// requested by the `_time_format=sqlite` DSN parameter, and the first layout it
// tries when reading them back. Unlike the driver's historical default it is a
// round trip whatever the zone of the value being stored.
const sqliteTimeFormat = "2006-01-02 15:04:05.999999999-07:00"

// numericZoneTimestamp matches the timestamps written before that parameter was
// set, and only those that no longer read back.
//
// The driver used to store a time.Time as time.Time.String(), whose last field
// is the zone abbreviation. A date coming from a tracker carries an offset that
// rarely matches the server's own zone, and Go then attaches a location with no
// name to it: String() prints the numeric offset a second time where the
// abbreviation belongs, and none of the driver's read layouts accepts that. The
// value comes back as a string, every Scan into a time.Time fails, and a single
// such row is enough to make the whole task list answer 500.
//
// A server running in the same zone as the tracker never produced one, which is
// why this only ever showed on the deployment, which runs in UTC, and never on
// a workstation in the tracker's own zone.
// The trailing ` m=...` is the monotonic reading String() appends to a value
// that came from time.Now(); it carries nothing about the instant.
var numericZoneTimestamp = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?) ([+-]\d{4}) [+-]\d{4}(?: m=[+-][\d.]+)?$`)

// numericZoneLayout parses what numericZoneTimestamp captures: the wall clock
// and the offset, the two fields that carry the instant. The trailing pseudo
// abbreviation is dropped, as it holds nothing the offset does not already say.
const numericZoneLayout = "2006-01-02 15:04:05.999999999 -0700"

type datetimeColumn struct {
	table  string
	column string
}

// repairNumericZoneTimestamps rewrites the stored timestamps that the driver can
// no longer read, in every declared date column of the schema. Setting the write
// format stops new ones from appearing; this is what makes the rows already on
// disk readable again, and it is the half that unbreaks an existing deployment.
//
// It is idempotent: a repaired value no longer matches, and a database that
// never held one does nothing but a few empty scans.
func (d *DB) repairNumericZoneTimestamps() {
	for _, col := range d.datetimeColumns() {
		n, err := d.repairDatetimeColumn(col)
		if err != nil {
			log.Printf("[repairNumericZoneTimestamps] %s.%s: %v", col.table, col.column, err)
			continue
		}
		if n > 0 {
			log.Printf("[repairNumericZoneTimestamps] %s.%s: %d timestamp(s) rewritten", col.table, col.column, n)
		}
	}
}

// datetimeColumns lists every column the schema declares as a date, which is
// exactly the set the driver converts on read and therefore the set that can
// carry an unreadable value.
func (d *DB) datetimeColumns() []datetimeColumn {
	rows, err := d.conn.Query(`
		SELECT m.name, p.name
		FROM sqlite_master m JOIN pragma_table_info(m.name) p
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'
		  AND upper(p.type) IN ('DATETIME', 'TIMESTAMP', 'DATE')
	`)
	if err != nil {
		log.Printf("[repairNumericZoneTimestamps] schema read failed: %v", err)
		return nil
	}
	defer rows.Close()

	var cols []datetimeColumn
	for rows.Next() {
		var c datetimeColumn
		if err := rows.Scan(&c.table, &c.column); err != nil {
			log.Printf("[repairNumericZoneTimestamps] schema read failed: %v", err)
			return cols
		}
		cols = append(cols, c)
	}
	return cols
}

func (d *DB) repairDatetimeColumn(col datetimeColumn) (int, error) {
	table := quoteIdentifier(col.table)
	column := quoteIdentifier(col.column)

	// Narrow the sweep to the values written in the String() shape, the only one
	// that can carry a numeric pseudo-abbreviation: they are the ones holding a
	// space before the offset sign. That leaves out both formats that read back
	// fine — the driver's own, whose offset follows the seconds directly, and
	// the bare CURRENT_TIMESTAMP, which has no offset at all. The regexp below
	// then rejects the String() values that are merely well-formed.
	rows, err := d.conn.Query(fmt.Sprintf(`
		SELECT rowid, CAST(%s AS TEXT) FROM %s
		WHERE typeof(%s) = 'text' AND (%s LIKE '%% +%%' OR %s LIKE '%% -%%')
	`, column, table, column, column, column))
	if err != nil {
		return 0, err
	}

	type repair struct {
		rowID int64
		value string
	}
	var repairs []repair
	for rows.Next() {
		var rowID int64
		var raw string
		if err := rows.Scan(&rowID, &raw); err != nil {
			rows.Close()
			return 0, err
		}
		match := numericZoneTimestamp.FindStringSubmatch(raw)
		if match == nil {
			continue
		}
		parsed, err := time.Parse(numericZoneLayout, match[1]+" "+match[2])
		if err != nil {
			// Unreadable and unrepairable: leave it rather than guess an
			// instant, and let the caller see it in the logs.
			log.Printf("[repairNumericZoneTimestamps] %s.%s rowid %d: %q left as is (%v)", col.table, col.column, rowID, raw, err)
			continue
		}
		repairs = append(repairs, repair{rowID: rowID, value: parsed.Format(sqliteTimeFormat)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	if len(repairs) == 0 {
		return 0, nil
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(fmt.Sprintf(`UPDATE %s SET %s = ? WHERE rowid = ?`, table, column))
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, r := range repairs {
		if _, err := stmt.Exec(r.value, r.rowID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(repairs), nil
}

// quoteIdentifier makes a name read from the schema safe to interpolate into a
// statement, which is the only way SQLite accepts a table or column name.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
