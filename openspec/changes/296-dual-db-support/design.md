# Design

## Decision: the abstraction goes below `*DB`, not above it
`*db.DB` exposes 210 exported methods, consumed as a concrete type by `internal/handlers`,
`internal/agent` and `internal/taskmcp`. A `Store` interface above it would need all 210 methods
and a second implementation of the ~20 000 lines behind them.

The measured reason this is unnecessary: **all 342 SQL call sites outside tests are inside
`internal/db`, and zero are outside it.** The isolation the ticket asks for already exists at the
package boundary; only the engine-specific fragments need abstracting. So `internal/db` keeps one
`*DB`, one set of queries, and gains an unexported `dialect`:

```go
type dialect interface {
    Open(cfg Config) (*sql.DB, error)
    Rebind(query string) string
    ColumnType(logical string) string   // "text" | "bool" | "timestamp" | "int"
    HasTable(conn *sql.DB, name string) (bool, error)
    HasColumn(conn *sql.DB, table, column string) (bool, error)
    SecretKeyDir(cfg Config) string     // "" when the operator must supply the key
    RunsLegacyMigrations() bool
}
```

Seven methods against 210. That is the thin interface the ticket asks for.

## Decision: `pgx/v5/stdlib` as the driver
`Dockerfile` l.34 and the release jobs build with `CGO_ENABLED=0`, so the driver must be pure Go.
`pgx` is, it is the maintained reference driver, and its `stdlib` wrapper plugs into
`database/sql` so `d.conn *sql.DB` and every existing call keep their types.

One trap to carry into implementation: pgx's default extended protocol rejects multi-statement
`Exec`. `migrateTasksKeyUnique` sends a 20-statement script in one call. This is a second reason
that migration stays SQLite-only (see below), not a reason to change protocol.

## Decision: rebind `?` at the boundary, do not rewrite 342 queries
Queries keep their `?` placeholders. Three wrappers — `d.exec`, `d.query`, `d.queryRow` — apply
`dialect.Rebind` before delegating. Converting the call sites is a mechanical rename
(`d.conn.Exec` to `d.exec`), reviewable by inspection, not a query rewrite where a transposed
argument would go unnoticed.

`Rebind` must be literal-aware: a `?` inside a quoted string is not a placeholder. It gets its
own table-driven unit test, including quoted literals, `??`, and queries with no placeholder.

## Decision: booleans stay `INTEGER` on both engines
24 columns are `INTEGER NOT NULL DEFAULT 0/1` and scanned into Go `bool` through the SQLite
driver's implicit conversion. PostgreSQL will not scan an `integer` into a `*bool`, but it will
happily store and return `INTEGER`. Mapping the logical boolean type to `INTEGER` on both engines
keeps all 24 columns and every `Scan` site byte-identical.

Rejected alternative: `BOOLEAN` on PostgreSQL. More idiomatic, but it forces an audit of every
read and write of those 24 columns for a gain that is invisible to the product. Reversible later
behind the same `ColumnType` seam.

## Decision: timestamps are `TIMESTAMPTZ`, in UTC, everywhere
Columns are declared `DATETIME` — not a PostgreSQL type — and defaulted with `CURRENT_TIMESTAMP`,
which returns UTC in SQLite and the session zone in PostgreSQL. The dialect maps the logical
timestamp type to `TIMESTAMPTZ`, and the PostgreSQL DSN pins the session to UTC so
`CURRENT_TIMESTAMP` keeps SQLite's meaning.

This is the highest-risk area of the change: #294 exists because timestamps round-tripped
incorrectly, and `_time_format=sqlite` is the workaround. That workaround is SQLite-only. The
timestamp round trip is an explicit smoke test on both engines, not something left to reviewers.

## Decision: a new PostgreSQL database is created at the current schema
The ~40 additive `ALTER TABLE ... ADD COLUMN` statements (`db.go` l.425-465, `usercredentials.go`
l.109) exist to patch databases created by older versions. `migrateTasksKeyUnique` (l.636-698)
exists to rebuild a `tasks` table created before `UNIQUE(project_id, key)`. The
`pragma_table_info` scan in `timestamprepair.go` l.72 and the `sqlite_master` probe in `macros.go`
l.92 are SQLite catalogue reads.

A PostgreSQL database has no such history: `CREATE TABLE` produces the complete, current
definition. All of these are gated behind `dialect.RunsLegacyMigrations()` and are no-ops on
PostgreSQL.

Consequence to respect during implementation: the `CREATE TABLE` statements must be brought up to
date with the columns the `ALTER TABLE` statements add, or a PostgreSQL database will be missing
them. This is the single most likely way to ship a broken PostgreSQL schema, and it is what task 6
checks.

## Decision: the encryption key is a dialect concern
`NewDB` currently calls `secrets.ServerKey(filepath.Dir(dbPath))` — it derives the key file's
directory from the database file path. Under PostgreSQL there is no file and no directory.

`dialect.SecretKeyDir` returns the database's directory on SQLite and the empty string on
PostgreSQL. `secrets.ServerKey` already refuses an empty directory when the environment variable
is unset, and `NewDB` already logs a warning and serves on without a key (l.162-167), which is the
convention this change follows:

> It is not required to serve: a deployment on a read-only volume, or one that never stores a
> personal credential, must still start. Only the operations that need the key refuse, and they
> say why.

The one hardening added: under PostgreSQL the absent key must never be followed by key generation.
No file is written, and the warning names `SECTILE_SECRET_KEY` as the only source.

Rejected alternative: refusing to boot without a key under PostgreSQL. It would make a perfectly
valid deployment — one that stores no personal credential at all — impossible to run, and it
contradicts a convention the code states explicitly.

## Decision: configuration variables
`DB_DRIVER` (`sqlite` default, `postgres`) selects the engine. `DATABASE_URL` carries the
PostgreSQL DSN. `DB_PATH` keeps its exact current meaning and is ignored under PostgreSQL.

`DB_PATH` is documented in `.env.sample` l.31 and baked into the `Dockerfile` l.51, so it must not
change meaning. `DATABASE_URL` is the conventional name and avoids inventing six `PG_*` variables.
Rejected alternative: deriving the engine from the shape of `DB_PATH` (a path versus a URL). It is
implicit, it makes a typo silently select the wrong engine, and the ticket asks for an explicit
variable.

## Decision: a single server instance
The owner settled that PostgreSQL backs one `sectile-server`, not several. `startQueueWorker`,
`StartAutoSync` and `ProjectLimiter` therefore keep their in-process design, and no advisory lock
or leader election enters this change. `SetMaxOpenConns(25)` is kept as-is; it is reasonable for
PostgreSQL too.

This is a deliberate constraint, not an oversight: two instances sharing one database would run
every queued skill job twice. If multi-instance is ever wanted, that is its own change with its
own design for the queue.

## Decision: the migration is a separate entry point, run deliberately
An automatic import on first start against an empty PostgreSQL database was considered and
rejected: it turns a configuration mistake into a data movement, and it gives the operator no
moment to verify the key precondition. The migration is an explicit command, run once, that
refuses a non-empty destination.

Tables are copied in foreign-key order. The declared references are `tasks` (2) and `users` (3),
so `projects`, `users` and `tasks` precede their dependants. `daily_digests` is retired storage
(dropped at startup) and is not copied. `tasks_new` is a transient migration artefact and is not a
table to copy.

## Test strategy
The owner settled on a `postgres:16` CI service plus a smoke subset, not the full suite on both
engines. Running all ~80 `NewDB` call sites across 38 files against PostgreSQL needs per-test
database isolation, which is a larger piece of work than this feature; it is a noted follow-up.

The smoke subset is chosen to cover what silently breaks rather than what is easy to test: schema
creation from empty, a task CRUD round trip, an `ON CONFLICT` upsert, a timestamp round trip (the
#294 regression, re-run against PostgreSQL), and a migration round trip from a seeded SQLite file.

`test:go` currently carries `allow_failure: true` because main has four pre-existing failures. The
new PostgreSQL job must **not** inherit that flag: it is new, it starts green, and it should gate.
