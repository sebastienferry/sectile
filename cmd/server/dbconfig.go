package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"tasks/internal/db"
)

// resolveDBConfig reads the database configuration out of the environment.
//
// DB_DRIVER selects the engine and defaults to SQLite, so a deployment that
// predates PostgreSQL support starts unchanged. DATABASE_URL carries the
// PostgreSQL connection string. DB_PATH keeps exactly the meaning it always
// had and is ignored under PostgreSQL.
//
// origin is what the startup log prints beside the target, so an operator can
// see which of the several possible sources won.
func resolveDBConfig(getenv func(string) string) (cfg db.Config, target, origin string, err error) {
	driver := strings.ToLower(strings.TrimSpace(getenv("DB_DRIVER")))
	switch driver {
	case "", string(db.DriverSQLite):
		path, pathOrigin := resolveDBPath(getenv("DB_PATH"))
		return db.SQLiteConfig(path), path, pathOrigin, nil

	case string(db.DriverPostgres), "postgresql", "pg":
		if dsn := strings.TrimSpace(getenv("DATABASE_URL")); dsn != "" {
			return db.Config{Driver: db.DriverPostgres, DSN: dsn}, redactDSN(dsn), "DATABASE_URL", nil
		}
		// No DSN: fall back to the standard libpq variables, which is how a
		// platform that stores a username and a password as two separate
		// secrets can hand them over. Kubernetes cannot interpolate a secret
		// into a string, so a single DATABASE_URL would force the whole
		// connection string, password included, into one more secret alongside
		// the two the database module already creates.
		if target := libpqTarget(getenv); target != "" {
			return db.Config{Driver: db.DriverPostgres, FromEnvironment: true}, target, "PG* environment", nil
		}
		return db.Config{}, "", "", fmt.Errorf(
			"DB_DRIVER=%s needs a connection: set DATABASE_URL, or the standard PGHOST/PGDATABASE/PGUSER/PGPASSWORD variables", driver)

	default:
		return db.Config{}, "", "", fmt.Errorf("unknown DB_DRIVER %q: expected %q or %q", driver, db.DriverSQLite, db.DriverPostgres)
	}
}

// libpqTarget describes the connection the libpq variables point at, and
// returns "" when they say nothing at all.
//
// PGHOST alone is enough to be deliberate: everything else has a usable libpq
// default, but a host does not, and dialing localhost because nobody said
// otherwise is exactly the silent fallback this whole path refuses.
func libpqTarget(getenv func(string) string) string {
	host := strings.TrimSpace(getenv("PGHOST"))
	if host == "" {
		return ""
	}
	if port := strings.TrimSpace(getenv("PGPORT")); port != "" {
		host += ":" + port
	}
	if name := strings.TrimSpace(getenv("PGDATABASE")); name != "" {
		return host + "/" + name
	}
	return host
}

// redactDSN keeps the host and the database name and drops everything that
// authenticates. The startup banner is read over shoulders and copied into
// issue reports, so it must never carry a password.
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		// A key/value DSN, or something unparseable. Naming the database is not
		// worth the risk of echoing a secret, so say nothing about it.
		return "(connection string)"
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return u.Host
	}
	return u.Host + "/" + name
}

// osGetenv is the indirection resolveDBConfig is tested through.
func osGetenv(key string) string { return os.Getenv(key) }
