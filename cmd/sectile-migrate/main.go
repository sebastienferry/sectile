// Command sectile-migrate copies an existing SQLite database into PostgreSQL.
//
// It runs once, by hand, and it is one-way. The reverse direction is not
// offered, and neither is an automatic import at server start: that would turn
// a configuration mistake into a data movement, with nobody watching.
//
// Usage:
//
//	SECTILE_SECRET_KEY=... sectile-migrate -from ./tasks.db -to postgres://user:pass@host/sectile
//
// The key matters. A stored tracker token is sealed to its owner and its
// tracker, not to the database, so the rows copy perfectly well under the wrong
// key and nobody notices until a tracker call fails. The migration checks the
// key opens one before it moves a single row, and refuses otherwise.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"tasks/internal/db"
)

func main() {
	from := flag.String("from", "", "path to the SQLite database to read")
	to := flag.String("to", os.Getenv("DATABASE_URL"), "PostgreSQL connection string to write (defaults to DATABASE_URL)")
	flag.Parse()

	if strings.TrimSpace(*from) == "" || strings.TrimSpace(*to) == "" {
		flag.Usage()
		log.Fatal("both -from and -to are required")
	}
	if _, err := os.Stat(*from); err != nil {
		log.Fatalf("source database: %v", err)
	}

	src, err := db.Open(db.SQLiteConfig(*from))
	if err != nil {
		log.Fatalf("opening the source: %v", err)
	}
	defer src.Close()

	dst, err := db.Open(db.Config{Driver: db.DriverPostgres, DSN: *to})
	if err != nil {
		log.Fatalf("opening the destination: %v", err)
	}
	defer dst.Close()

	fmt.Printf("Migrating %s into PostgreSQL.\n", *from)
	counts, err := db.Migrate(src, dst, os.Stdout)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	total := 0
	for _, c := range counts {
		total += c.Rows
	}
	fmt.Printf("\nDone: %d row(s) across %d table(s).\n", total, len(counts))
	fmt.Printf("Start the server with DB_DRIVER=postgres and the same %s.\n", "SECTILE_SECRET_KEY")
}
