# ADR 0016: PostgreSQL is an alternative store, behind a dialect rather than a repository

Status: Accepted

## Context

Sectile has always kept everything in one embedded SQLite file, opened by
`db.NewDB(dbPath)` through `modernc.org/sqlite`. That is the right default: it
needs no server, it ships inside the desktop application, and a developer
clones the repository and runs it.

It is not the right only option. A deployment that already runs PostgreSQL
cannot use it, and inherits instead the operational profile of a file: no
managed backup, no point-in-time recovery, no external tooling, and a volume
that has to follow the container. Issue #296 asks for PostgreSQL as an
alternative, selected by environment variable, behind a thin code-level
abstraction.

"Thin abstraction" is where this record earns its place, because the obvious
reading of it is wrong.

## Decision

**The abstraction goes below `*DB`, not above it.**

The measurement that settles it: all 342 SQL call sites outside tests live
inside `internal/db`, and zero live anywhere else. `internal/handlers`,
`internal/agent` and `internal/taskmcp` never write SQL. The isolation a
storage abstraction is supposed to buy already exists, at the package boundary.

Meanwhile `*db.DB` exposes 210 exported methods, consumed as a concrete type. A
`Store` interface above it would need all 210, and a second implementation of
the roughly 20 000 lines behind them. Two copies of every query, kept in step
by hand, forever.

So `internal/db` keeps one `*DB`, one set of queries, and gains an unexported
`dialect` carrying only what actually differs between the two engines:
connection opening, placeholder rebinding, DDL type names, catalogue
introspection, the encryption-key directory, and whether the historical
migrations run. Seven methods against 210.

**Queries keep their `?` placeholders.** A `Rebind` at the boundary converts to
`$n` for PostgreSQL. Rewriting 342 queries by hand would be a change where a
transposed argument goes unnoticed; converting `d.conn.Exec` to `d.exec` is a
rename a reviewer can check by inspection.

**Booleans stay `INTEGER` on both engines.** 24 columns are `INTEGER NOT NULL
DEFAULT 0/1`, scanned into Go `bool` by the SQLite driver's implicit
conversion. PostgreSQL will not scan an `integer` into a `*bool`, but it stores
and returns `INTEGER` happily. Mapping the logical boolean to `INTEGER`
everywhere keeps every `Scan` site byte-identical.

**Timestamps are `TIMESTAMPTZ` with the session pinned to UTC.** Columns are
declared `DATETIME`, which PostgreSQL does not have, and defaulted with
`CURRENT_TIMESTAMP`, which returns UTC in SQLite and the session zone in
PostgreSQL. Pinning the session to UTC keeps `CURRENT_TIMESTAMP` meaning the
same thing on both.

**A new PostgreSQL database is created at the current schema, with no
historical replay.** The ~40 additive `ALTER TABLE ADD COLUMN` statements,
`migrateTasksKeyUnique`, the `pragma_table_info` scan and the `sqlite_master`
probe all exist to repair databases written by older versions. PostgreSQL has
no such history.

**A shape change both engines need goes through the seam, as a method named
after what it achieves.** `MigrateActivityAttachment` (ticket #310) is the first
of these: SQLite can neither relax a `NOT NULL` nor add a foreign key nor add a
`CHECK` through `ALTER TABLE`, so it rebuilds the table, while PostgreSQL walks
it with `ALTER TABLE`. It also takes the data backfill as an argument, because
the two engines need it at different points of their sequence — PostgreSQL
validates a foreign key against the rows already there — while the backfill
itself stays engine-independent, as it must: a PostgreSQL database created since
PR #304 holds the very rows it repairs, and the legacy block never runs there.
The alternative, widening `RewriteDDL` into a statement generator, was rejected:
the keys of the seam stay named after what is needed, not after the engine.

**PostgreSQL backs one server instance, not several.** The job queue, the sync
loop and the per-project limiter stay in-process.

## Consequences

- The PostgreSQL schema is only correct if every `CREATE TABLE` carries the
  columns its `ALTER TABLE` statements add. This is the single most likely way
  to ship a broken PostgreSQL schema, and it is invisible to the SQLite tests,
  which keep going through the `ALTER` path. It is checked explicitly: a
  database built with the legacy migrations disabled must have the same schema
  as one built with them enabled.

  The first version of this record only counted the statements in `initSchema`,
  and the guard only covered those. It missed the tables created by their own
  `ensure…Table` helper, each carrying its own `ALTER`: seven further columns
  reached PostgreSQL missing, `users.role` among them, which is read on every
  authorisation check. Those helpers are now gated the same way and the guard
  covers them. The rule to carry forward: **any** `ALTER TABLE ... ADD COLUMN`
  anywhere in the package is a column its `CREATE TABLE` must also declare.

- `CREATE TABLE IF NOT EXISTS` does not retrofit a column onto a table that
  already exists, so a PostgreSQL database keeps the schema it was created
  with. This record first concluded that such a database had to be recreated or
  patched by hand, on the grounds that an automatic repair would mean replaying
  the historical SQLite migrations. That holds for the history, and does not
  hold from the moment PostgreSQL shipped: a running deployment cannot be asked
  to be recreated because a column was added, and #327 proved it — `blocked_at`
  reached the CREATE TABLE and the legacy `ALTER`, and an upgraded deployment
  answered every attempt to block an account with `column "blocked_at" does not
  exist`.

  So the repair exists, and it is bounded rather than a replay: `lateColumns`
  in `internal/db/db.go` lists the columns declared *after* PostgreSQL support,
  each as an idempotent `ADD COLUMN IF NOT EXISTS`, applied on every start by
  the engines that skip the legacy migrations. The ~90 columns before that point
  are not in it, and must not be: they only ever went missing from a SQLite file.
  The rule to carry forward, alongside the CREATE TABLE one above: **a column
  added from now on goes in the `CREATE TABLE`, in the legacy `ALTER`, and in
  `lateColumns`.** The third is checked — `TestPostgresUpgradeRestoresLateColumns`
  drops every listed column, opens the store again and requires them all back —
  but the check can only cover what the list names, so the list is still the
  thing a reviewer has to read.

  A database created in the hours between PostgreSQL support (#296) and the
  first correction of the schema's completeness (#302) is missing `users.role`
  and six others, which predate this list. That one really does have to be
  patched by hand; no deployment is known to be in that state.
- `pgx` rejects multi-statement `Exec` under its default extended protocol, and
  `migrateTasksKeyUnique` sends a 20-statement script. Another reason it stays
  SQLite-only, and a trap for anyone who later tries to make it portable.
- The encryption key can no longer be derived from the database file's
  directory. Under PostgreSQL it comes from `SECTILE_SECRET_KEY` and is never
  generated: a container without persistent storage would otherwise invent a
  new key on every restart and silently orphan every stored token. An absent
  key degrades rather than refusing to boot, which is the convention `NewDB`
  already states.
- Two instances against one database would run every queued skill job twice.
  Multi-instance is therefore not merely unsupported, it is unsafe, and that is
  a property of the queue rather than of the store.
- Running the whole suite against both engines needs per-test database
  isolation for ~80 `NewDB` call sites across 38 files. Not done here: CI runs
  a smoke subset chosen for what breaks silently — schema creation, a CRUD
  round trip, an upsert, a timestamp round trip, a migration round trip.

## Alternatives rejected

- **A `Store` interface above `*DB`.** The literal reading of "thin interface".
  210 methods, a second implementation of ~20 000 lines, two copies of every
  query. Rejected on cost, and unnecessary given the measured containment.
- **`BOOLEAN` columns on PostgreSQL.** More idiomatic, but it forces an audit
  of every read and write of 24 columns for a gain invisible to the product.
  Reversible later behind the same seam.
- **Deriving the engine from the shape of `DB_PATH`** (a path versus a URL).
  Implicit, and a typo silently selects the wrong engine.
- **An automatic import on first start against an empty PostgreSQL database.**
  It turns a configuration mistake into a data movement, and gives the operator
  no moment to verify that the encryption key came across.
- **Refusing to boot without an encryption key under PostgreSQL.** It would
  make a deployment that stores no personal credential impossible to run, and
  contradicts the convention stated at `NewDB`.
- **An ORM.** It would rewrite all 342 queries to solve a problem that is six
  syntactic differences wide.

## Amendment (#410, 2026-09-26): several instances are supported

The consequence "Multi-instance is therefore not merely unsupported, it is
unsafe" no longer holds. Macro #397 made every piece of state that lived in one
process either shared through PostgreSQL or reached through the instance that
holds it, and the queue no longer runs a job twice. Several server instances on
one PostgreSQL database are the supported way to scale and to deploy without
downtime; SQLite remains single-instance. See
[ADR 0030](0030-several-server-replicas-share-one-postgresql.md).
