# Plan #411 - A guard on the frozen baseline schema

## Stack

Go, `internal/db`, SQLite (`modernc.org/sqlite` through the package's own
dialect), standard `testing`.

## What exists

- `migrateSchema` runs `applyBaseline` on a database with no version, then
  `applyMigrations(migrations, current)` (`internal/db/migrations.go`).
- `openWith(cfg, dialect)` builds the `DB` and calls `migrateSchema`.
- `columnNames` reads column names. SQLite exposes the rest through
  `PRAGMA table_info`.

## Decisions

### 1. Building the baseline alone

The test assembles a `DB` from the same parts `openWith` uses (dialect,
connection, `newSQLConn`), then runs `ensureSchemaMigrationsTable` and
`applyBaseline` only. No production code changes: `openWith` keeps running
every migration.

Rejected: a `DB.migrations` field read by `migrateSchema`, which was the first
plan. The test does not need it, since `applyBaseline` relies on no other field
of `DB`. Also rejected: swapping the package-level `migrations` variable in
the test. It is shared by every test in the package and would race with any
parallel one.

### 2. The snapshot

`internal/db/testdata/baseline_schema.txt`, one line per column, sorted:
`table.column TYPE [NOT NULL] [DEFAULT x] [PRIMARY KEY]`, plus one
`index <name> on <table>` line per named index (an index added to the baseline
has the same problem as a column). `sqlite_%` tables and `schema_migrations`
are excluded: the first are internal, the second is written by the runner,
not by the baseline.

### 3. The failure message

It lists the lines missing from and added to the snapshot, then says: the
baseline is frozen (ADR 0021). Write a numbered migration in
`internal/db/migrations.go` and revert the baseline edit. There is no `-update`
flag on purpose: the snapshot only changes when the baseline does, which must
not happen.

## Target files

| File | Change |
| --- | --- |
| `internal/db/baseline_guard_test.go` | new: helper, snapshot reader, test |
| `internal/db/testdata/baseline_schema.txt` | new: the snapshot |

## Risks

- The snapshot depends on how SQLite reports declared types and defaults. It is
  generated from the same engine the test runs, so it is stable on one driver
  version. A driver upgrade that changed `PRAGMA table_info` output would fail
  the guard spuriously. It would then be regenerated deliberately, in a
  reviewed diff.
