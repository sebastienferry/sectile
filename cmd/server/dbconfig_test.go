package main

import "testing"

func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

func TestResolveDBConfigDefaultsToSQLite(t *testing.T) {
	cfg, _, _, err := resolveDBConfig(env(map[string]string{"DB_PATH": "/tmp/x.db"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Driver != "sqlite" && cfg.Driver != "" {
		t.Fatalf("driver = %q, want SQLite", cfg.Driver)
	}
	if cfg.Path != "/tmp/x.db" {
		t.Fatalf("path = %q, want the DB_PATH value", cfg.Path)
	}
}

func TestResolveDBConfigPostgres(t *testing.T) {
	cfg, target, origin, err := resolveDBConfig(env(map[string]string{
		"DB_DRIVER":    "postgres",
		"DATABASE_URL": "postgres://sectile:hunter2@db.internal:5432/sectile?sslmode=require",
		// Set, and must be ignored.
		"DB_PATH": "/tmp/should-be-ignored.db",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Driver != "postgres" {
		t.Fatalf("driver = %q, want postgres", cfg.Driver)
	}
	if cfg.Path != "" {
		t.Fatalf("path = %q, want DB_PATH to be ignored under PostgreSQL", cfg.Path)
	}
	if origin != "DATABASE_URL" {
		t.Fatalf("origin = %q, want DATABASE_URL", origin)
	}
	if target != "db.internal:5432/sectile" {
		t.Fatalf("target = %q, want host and database name", target)
	}
}

// The banner is read over shoulders and pasted into issue reports.
func TestResolveDBConfigNeverEchoesThePassword(t *testing.T) {
	for _, dsn := range []string{
		"postgres://sectile:hunter2@db.internal:5432/sectile",
		"postgresql://u:p@h/d?sslmode=disable",
		"host=db.internal user=sectile password=hunter2 dbname=sectile",
	} {
		_, target, _, err := resolveDBConfig(env(map[string]string{"DB_DRIVER": "postgres", "DATABASE_URL": dsn}))
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", dsn, err)
		}
		for _, secret := range []string{"hunter2", "password=", ":p@"} {
			if contains(target, secret) {
				t.Fatalf("the startup target %q leaks %q from %q", target, secret, dsn)
			}
		}
	}
}

// The platform stores a username and a password as two separate secrets, and
// Kubernetes cannot interpolate a secret into a string, so the connection has
// to be expressible without a single DATABASE_URL.
func TestResolveDBConfigAcceptsLibpqVariables(t *testing.T) {
	cfg, target, origin, err := resolveDBConfig(env(map[string]string{
		"DB_DRIVER":  "postgres",
		"PGHOST":     "pgsql-central.internal",
		"PGPORT":     "5432",
		"PGDATABASE": "sectile",
		"PGUSER":     "sectile",
		"PGPASSWORD": "hunter2",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.FromEnvironment {
		t.Fatal("the connection should be taken from the environment")
	}
	if cfg.DSN != "" {
		t.Fatalf("DSN = %q, want it left to pgx and the PG* variables", cfg.DSN)
	}
	if origin != "PG* environment" {
		t.Fatalf("origin = %q", origin)
	}
	if target != "pgsql-central.internal:5432/sectile" {
		t.Fatalf("target = %q, want host, port and database name", target)
	}
	if contains(target, "hunter2") {
		t.Fatalf("the startup target %q leaks the password", target)
	}
}

// DATABASE_URL stays the primary source when both are present, so an existing
// deployment is not changed by variables that happen to be in its environment.
func TestResolveDBConfigPrefersTheDSN(t *testing.T) {
	cfg, _, origin, err := resolveDBConfig(env(map[string]string{
		"DB_DRIVER":    "postgres",
		"DATABASE_URL": "postgres://u@explicit.host/explicit",
		"PGHOST":       "ignored.host",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.FromEnvironment || cfg.DSN == "" {
		t.Fatal("DATABASE_URL must win over the PG* variables")
	}
	if origin != "DATABASE_URL" {
		t.Fatalf("origin = %q", origin)
	}
}

func TestResolveDBConfigRejectsIncompleteConfiguration(t *testing.T) {
	if _, _, _, err := resolveDBConfig(env(map[string]string{"DB_DRIVER": "postgres"})); err == nil {
		t.Fatal("PostgreSQL with neither DATABASE_URL nor PGHOST must be refused, not silently served from SQLite")
	}
	// PGUSER alone is not a connection: pgx would default the host to localhost
	// and dial something nobody asked for.
	if _, _, _, err := resolveDBConfig(env(map[string]string{"DB_DRIVER": "postgres", "PGUSER": "sectile"})); err == nil {
		t.Fatal("PG* variables without PGHOST must be refused rather than defaulting to localhost")
	}
	if _, _, _, err := resolveDBConfig(env(map[string]string{"DB_DRIVER": "mysql"})); err == nil {
		t.Fatal("an unknown driver must be refused")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && stringIndex(haystack, needle) >= 0
}

func stringIndex(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
