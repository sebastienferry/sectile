# Implementation checklist

Ordered so that each step is reviewable on its own and the suite stays green throughout.
Steps 2 to 6 change no behaviour on SQLite; the PostgreSQL path only becomes reachable at step 7.

- [x] 1. Validate this change with `openspec validate 296-dual-db-support --strict`.
- [x] 2. Write `docs/adrs/0016-postgresql-as-an-alternative-store.md` recording the decision and
      the rejected alternatives (interface above `*DB`, `BOOLEAN` columns, automatic import on
      first start, refusing to boot without a key). The repository keeps ADRs for architectural
      trade-offs and this is one.
- [x] 3. Add `internal/db/dialect.go` with the `dialect` interface and `Config`, plus
      `internal/db/dialect_sqlite.go` reproducing today's behaviour exactly. No call site changes
      yet. `go test ./...` stays green.
- [x] 4. Add `Rebind` to the SQLite dialect as an identity function and to a new
      `internal/db/rebind.go` as the shared literal-aware implementation. Unit-test it:
      placeholders, `?` inside single- and double-quoted literals, no placeholder, many
      placeholders, and a query mixing both.
- [x] 5. Introduce `d.exec`, `d.query`, `d.queryRow` wrappers applying `dialect.Rebind`, and
      convert the 342 call sites in `internal/db` to them. Mechanical rename; review by inspection.
- [x] 6. Bring every `CREATE TABLE IF NOT EXISTS` up to date with the columns its `ALTER TABLE`
      statements add, then gate the legacy migrations behind `dialect.RunsLegacyMigrations()`:
      the ~40 `ALTER TABLE` statements, `migrateTasksKeyUnique`, the `pragma_table_info` scan in
      `timestamprepair.go` and the `sqlite_master` probe in `macros.go`.
      **Verification that this step is correct:** create a fresh SQLite database with the legacy
      migrations disabled and assert its schema is identical to one created with them enabled.
      Without this check a PostgreSQL database silently ships missing columns.
- [x] 7. Add `internal/db/dialect_postgres.go`: `pgx/v5/stdlib`, session pinned to UTC,
      `TIMESTAMPTZ` for the timestamp type, `INTEGER` for the boolean type, `SecretKeyDir`
      returning the empty string, `RunsLegacyMigrations` returning false.
- [x] 8. Change `NewDB(dbPath string)` to `NewDB(cfg Config)` and add the path-shaped helper the
      ~80 existing test call sites keep using unchanged.
- [x] 9. Route the encryption key through `dialect.SecretKeyDir` instead of
      `filepath.Dir(dbPath)`. Under PostgreSQL an absent key logs the existing warning, serves on,
      and **never** generates or writes a key file.
- [x] 10. Read `DB_DRIVER` and `DATABASE_URL` in `cmd/server/main.go`. Skip `resolveDBPath`
      entirely under PostgreSQL, refuse to start on an unknown driver, a missing DSN or an
      unreachable server, and make the startup log report the engine without ever printing the
      DSN password.
- [x] 11. Add the smoke tests, skipped when no test database is configured: schema creation from
      empty, task CRUD round trip, `ON CONFLICT` upsert, timestamp round trip, second start on an
      initialised database.
- [x] 12. Add the `postgres:16` service and the PostgreSQL smoke job to `.gitlab-ci.yml`. Do
      **not** carry `allow_failure: true` from `test:go`: the new job starts green and must gate.
- [x] 13. Add the migration entry point: destination emptiness check, encryption-key precondition
      check before the first row, copy in foreign-key order (`projects`, `users`, `tasks` before
      their dependants), skipping `daily_digests`, leaving the source untouched, and a per-table
      row-count report.
- [x] 14. Add the migration round-trip test: seed a SQLite database including one server-key-sealed
      credential and one passphrase-sealed credential, migrate, assert row counts per table and
      that both credentials still open.
- [x] 15. Document `DB_DRIVER`, `DATABASE_URL` and the `SECTILE_SECRET_KEY` requirement in
      `.env.sample`, the `Dockerfile` and the deployment section of `README.md`. State plainly that
      under PostgreSQL the key must be supplied and is never generated.
- [x] 16. Update `CHANGELOG.md` and `docs/ARCHITECTURE.md` where they describe persistence.
- [x] 17. Run `make test` and fix until green. Run the PostgreSQL smoke subset against a local
      `postgres:16` container.
- [x] 18. Review the diff, commit, push and update pull request #297.

## Deviations from the plan, and why

Recorded here rather than silently: the acceptance criteria are unchanged, the route to them
is not.

- **Task 5 — the 342 call sites were not renamed.** The plan said convert `d.conn.Exec` to
  `d.exec` across 27 files. Instead `d.conn` became a wrapper type holding the pool and the
  dialect, so every call site stayed exactly as written. A 335-site mechanical edit still
  compiles when an argument list is mistyped, and then runs against the wrong column; wrapping
  the handle removes that class of mistake entirely and puts the whole engine difference in one
  readable file.
- **Task 8 — `NewDB(dbPath string)` kept its signature.** The plan had it become
  `NewDB(cfg Config)` plus a path-shaped helper, which would have renamed ~80 test call sites
  after all. `NewDB` *is* the path-shaped helper; `Open(cfg Config)` is the general constructor.
  Zero test churn.
- **`ColumnType` was replaced by `RewriteDDL`.** The schema is literal SQL, so nothing ever
  called a type-mapping function. The whole schema uses four types, two of which PostgreSQL does
  not have (`DATETIME`, `BLOB`), so a guarded DDL rewrite is what the job actually needed.
- **Five tables were created lazily, not by the schema.** `task_comments`, `teams`,
  `team_members`, `macros` and `project_skills` were created on first use. That works while a
  server runs but leaves a fresh database incomplete, which the migration discovers the hard
  way. They are now created at schema time, from their own definitions.

## What the verification actually caught

Neither of these would have been found by review, and both were found by the tests the plan
asked for.

1. **Task 6 found 41 missing columns and one missing table** on the first run: `projects` (19),
   `settings` (11), `task_activities` (7), `tasks` (4), plus `pinned_tasks`. Every SQLite test
   was green throughout, because the `ALTER` path put them back. PostgreSQL would have shipped
   a schema quietly missing all of them.
2. **The smoke tests found the session zone was pinned on one connection only.** `SET TIME ZONE`
   after `sql.Open` reaches exactly one pooled connection; every later one kept the server's
   zone, so behaviour depended on which connection served the write. It is now a connection
   parameter, so it is part of every connection the pool ever makes.

## Evidence

- `make test`: exit 0. 16 Go packages ok, web tests ok, `tsc --noEmit` clean, `oxlint` reports
  only warnings that predate this change in files it does not touch.
- PostgreSQL 18.4, real server: 8 integration tests pass — schema from empty, second start,
  CRUD round trip, `ON CONFLICT` upsert, timestamp round trip, migration round trip, refusal of
  a non-empty destination, refusal of a key that does not open the credentials.
- `sectile-migrate` end to end: 18 tables copied, and both refusals fire on a second run.
- Baseline before the change was green locally, so nothing here is masking a pre-existing
  failure. The four failures `.gitlab-ci.yml` documents for `test:go` do not reproduce on this
  machine.

## Follow-up after the deployment review (2026-09-21)

The ArgoCD deployment surfaced a mismatch the specification had not foreseen, and the fix landed
on this branch rather than in a later ticket, because it changes an acceptance criterion.

**What the deployment actually gets.** The Terraform module provisioning the database
(`iac-data`, `live/databases/sp/europe-west4/*`) creates the credentials in Secret Manager as
`<env>_application_sectile_all_db_user-name` and `_password` — two separate secrets. Kubernetes
cannot interpolate a secret into a string environment variable, so a single `DATABASE_URL` would
force the whole connection string, password included, into a third secret maintained by hand
beside the two the module already creates.

**The change.** `DATABASE_URL` stays the primary source. When it is absent, the standard libpq
variables supply the connection, which `pgx` reads natively. `Config.FromEnvironment` says so
explicitly rather than being inferred from an empty DSN — pgx falls back to those variables on its
own, so an unconfigured server would otherwise dial localhost instead of refusing to start.
`PGHOST` is what makes the intent deliberate.

- [x] 19. Accept the standard PostgreSQL connection variables when `DATABASE_URL` is absent, keep
      the refusal when neither source says anything, and keep the password out of the startup log.
- [x] 20. Update the behaviour spec (three scenarios replace the single "no connection string"
      one), the design record, `.env.sample`, the `Dockerfile` and the README.
- [x] 21. Cover it: two unit tests on the resolution and its precedence, one integration test that
      actually connects over the environment-provided variables, one that refuses an empty
      configuration.
