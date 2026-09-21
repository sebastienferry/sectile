# Support PostgreSQL as an alternative to SQLite

## Why
Issue #296 asks for PostgreSQL as an alternative to the embedded SQLite database, selected by
environment variable, behind a thin code-level abstraction. Today `db.NewDB(dbPath)` opens one
file with `modernc.org/sqlite` and every deployment inherits the operational profile of an
embedded file: no managed backup, no point-in-time recovery, no external tooling, and a volume
that has to follow the container. An operator who already runs PostgreSQL cannot use it.

The clarification measured the surface before proposing anything. All 342 SQL call sites outside
tests live inside `internal/db`; not one lives anywhere else. The seam therefore already exists
at the package boundary. But `*db.DB` exposes 210 exported methods consumed as a concrete type,
so an interface *above* it would mean a second implementation of roughly 20 000 lines. The thin
interface the ticket asks for belongs *below* `*DB`, where the two engines actually differ.

## What Changes
- Add `internal/db/dialect.go`: an unexported `dialect` interface carrying the six things that
  differ between the engines — connection opening, placeholder rebinding, DDL type names, upsert
  syntax, catalogue introspection, and encryption-key directory resolution.
- Add `internal/db/dialect_sqlite.go` and `internal/db/dialect_postgres.go`.
- Change `NewDB(dbPath string)` to `NewDB(cfg Config)`, keeping a path-shaped helper so the ~80
  existing test call sites are untouched.
- Route the 342 SQL call sites through `d.exec` / `d.query` / `d.queryRow` wrappers that apply
  the dialect's rebinding. This is a mechanical rename, not a query rewrite: the queries keep `?`.
- Gate the SQLite-only migrations behind the dialect: the ~40 additive `ALTER TABLE` statements,
  `migrateTasksKeyUnique` (which reads `sqlite_master` and rebuilds a table), the
  `pragma_table_info` scan in `timestamprepair.go` and the `sqlite_master` probe in `macros.go`.
- Resolve the server encryption key through the dialect instead of `filepath.Dir(dbPath)`.
- Add `cmd/sectile-migrate` (or an equivalent entry point): a one-way SQLite to PostgreSQL copy.
- Add a `postgres:16` service and a smoke job to `.gitlab-ci.yml`.
- Document `DB_DRIVER`, `DATABASE_URL` and the `SECTILE_SECRET_KEY` requirement in `.env.sample`,
  the `Dockerfile` and `README.md`.

## Capabilities
### New Capabilities
- `database-engine-selection`: the backing database engine is chosen by configuration, and the
  application behaves identically whichever engine serves it.
- `sqlite-to-postgres-migration`: an existing SQLite database can be moved to PostgreSQL once,
  with its sealed credentials still readable afterwards.

### Modified Capabilities
None. No existing capability changes its observable behaviour.

## Impact
Confined to `internal/db`, `internal/secrets`, `cmd/server` and one new command. No handler, no
API route, no web asset and no tracker integration changes. The data model is untouched: the same
18 tables, the same columns, the same JSON-in-TEXT encoding. SQLite remains the default and the
only engine the desktop application ships with.

## Out of Scope
Changing the data model, introducing an ORM, sharding or replication, running several
`sectile-server` instances against one database, migrating in the PostgreSQL to SQLite direction,
and running the full test suite against both engines (a noted follow-up).

## Decision Source
`docs/clarifications/296.md`, rounds 1 to 4. Four product questions answered by the owner, zero
left open.
