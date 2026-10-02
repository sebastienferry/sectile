package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestRequireUTF8 is the decision of #693 on its own: only UTF8 is accepted,
// whatever its case, and a refusal names the database, what it holds and what
// it needs.
func TestRequireUTF8(t *testing.T) {
	for _, encoding := range []string{"UTF8", "utf8"} {
		if err := requireUTF8(encoding, "sectile"); err != nil {
			t.Errorf("%s refused: %v", encoding, err)
		}
	}
	for _, encoding := range []string{"SQL_ASCII", "LATIN1", ""} {
		err := requireUTF8(encoding, "sectile")
		if err == nil {
			t.Errorf("%q accepted", encoding)
			continue
		}
		for _, want := range []string{`"sectile"`, encoding, "UTF8", "TEMPLATE template0"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%q: the refusal %q does not name %q", encoding, err, want)
			}
		}
	}
}

// TestPostgresRefusesANonUTF8Database opens a SQL_ASCII database, where
// LENGTH and SUBSTR count bytes (#693), and expects the opening to fail before
// the schema is written. The database is made beside the test one and dropped.
func TestPostgresRefusesANonUTF8Database(t *testing.T) {
	dsn := postgresDSN(t)
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	// Closed by the cleanup below, after the drop: a deferred Close would run first.
	t.Cleanup(func() { admin.Close() })
	name := fmt.Sprintf("sectile_test_sql_ascii_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name + " ENCODING 'SQL_ASCII' TEMPLATE template0 LC_COLLATE 'C' LC_CTYPE 'C'"); err != nil {
		t.Skipf("cannot create a SQL_ASCII database with this role: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)"); err != nil {
			t.Errorf("dropping %s: %v", name, err)
		}
	})
	target, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	target.Path = "/" + name

	d, err := Open(Config{Driver: DriverPostgres, DSN: target.String()})
	if err == nil {
		d.Close()
		t.Fatal("a SQL_ASCII database was opened")
	}
	if !strings.Contains(err.Error(), "SQL_ASCII") || !strings.Contains(err.Error(), "UTF8") {
		t.Errorf("the refusal %q does not name the encoding found and the one needed", err)
	}

	check, err := sql.Open("pgx", target.String())
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var tables int
	if err := check.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Errorf("the refused database holds %d tables, want none", tables)
	}
}
