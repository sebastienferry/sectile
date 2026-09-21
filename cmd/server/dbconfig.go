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
		dsn := strings.TrimSpace(getenv("DATABASE_URL"))
		if dsn == "" {
			return db.Config{}, "", "", fmt.Errorf("DB_DRIVER=%s requires a connection string in DATABASE_URL", driver)
		}
		return db.Config{Driver: db.DriverPostgres, DSN: dsn}, redactDSN(dsn), "DATABASE_URL", nil

	default:
		return db.Config{}, "", "", fmt.Errorf("unknown DB_DRIVER %q: expected %q or %q", driver, db.DriverSQLite, db.DriverPostgres)
	}
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
