# Plan #693 - A PostgreSQL database must be UTF8

## 1. Placement

`internal/db`: the dialect interface, both dialects, `openWith`. Test helpers
in `internal/db`, `internal/handlers`, `cmd/server`. Docs and changelog.

## 2. Check

- New dialect method `CheckEncoding(conn *sql.DB) error`, documented beside
  `LowerASCII`, the other cluster-dependent behaviour.
- SQLite: returns nil.
- PostgreSQL: `SELECT current_setting('server_encoding'), current_database()`,
  then `requireUTF8(encoding, database)`.
- `requireUTF8` is pure: nil for `UTF8` in any case, else
  `fmt.Errorf("the PostgreSQL database %q is encoded in %s, and Sectile needs UTF8: ... ")`
  with the fix. `server_encoding` is the encoding of the connected database,
  not the client's, which pgx always sets to UTF8.
- `openWith`: right after `Ping`, `d.CheckEncoding(conn)`; on error close the
  pool and return it unwrapped, before `migrateSchema`.

## 3. Tests

- `internal/db/dialect_postgres_test.go` (or the nearest existing file):
  `TestRequireUTF8` table test: `UTF8`, `utf8` accepted; `SQL_ASCII`, `LATIN1`
  refused, message names the encoding, the database and `UTF8`.
- `TestPostgresRefusesANonUTF8Database`: from `SECTILE_TEST_POSTGRES_DSN`,
  `CREATE DATABASE sectile_test_sql_ascii_<n> ENCODING 'SQL_ASCII' TEMPLATE
  template0 LC_COLLATE 'C' LC_CTYPE 'C'`, open it through `Open`, expect the
  error, check no `schema_migrations` table was created, drop the database.
  Skips without the DSN.
- Helpers: `openPostgres` and `openPostgresInstance` wrap the failure as
  `SECTILE_TEST_POSTGRES_DSN cannot be used: %v`; the harness does the same
  with a pre-flight `db.Open` + `Close`.

## 4. Docs

- README "PostgreSQL": one paragraph, the database must be UTF8, with the
  `CREATE DATABASE ... ENCODING 'UTF8' TEMPLATE template0` line.
- `docs/TESTING.md`: the test database must be UTF8, and a `initdb --locale=C`
  cluster defaults to SQL_ASCII.
- `remoterun.go`: the comment says characters on both engines "on a UTF8
  PostgreSQL database, which Open requires".

## 5. Changelog

`Changed`: "**A Sectile server refuses a PostgreSQL database that is not
UTF8.** On a SQL_ASCII or other non-UTF8 database, run output was cut at half
its length and could store a broken character; the server now stops at
startup and says how to create a UTF8 database. A UTF8 database, the
PostgreSQL default, is not affected. (#693)"
