# Implementation checklist

Ordered so that each step is reviewable on its own and the suite stays green throughout.
Steps 2 to 6 change no behaviour on SQLite; the PostgreSQL path only becomes reachable at step 7.

- [ ] 1. Validate this change with `openspec validate 296-dual-db-support --strict`.
- [ ] 2. Write `docs/adrs/0016-postgresql-as-an-alternative-store.md` recording the decision and
      the rejected alternatives (interface above `*DB`, `BOOLEAN` columns, automatic import on
      first start, refusing to boot without a key). The repository keeps ADRs for architectural
      trade-offs and this is one.
- [ ] 3. Add `internal/db/dialect.go` with the `dialect` interface and `Config`, plus
      `internal/db/dialect_sqlite.go` reproducing today's behaviour exactly. No call site changes
      yet. `go test ./...` stays green.
- [ ] 4. Add `Rebind` to the SQLite dialect as an identity function and to a new
      `internal/db/rebind.go` as the shared literal-aware implementation. Unit-test it:
      placeholders, `?` inside single- and double-quoted literals, no placeholder, many
      placeholders, and a query mixing both.
- [ ] 5. Introduce `d.exec`, `d.query`, `d.queryRow` wrappers applying `dialect.Rebind`, and
      convert the 342 call sites in `internal/db` to them. Mechanical rename; review by inspection.
- [ ] 6. Bring every `CREATE TABLE IF NOT EXISTS` up to date with the columns its `ALTER TABLE`
      statements add, then gate the legacy migrations behind `dialect.RunsLegacyMigrations()`:
      the ~40 `ALTER TABLE` statements, `migrateTasksKeyUnique`, the `pragma_table_info` scan in
      `timestamprepair.go` and the `sqlite_master` probe in `macros.go`.
      **Verification that this step is correct:** create a fresh SQLite database with the legacy
      migrations disabled and assert its schema is identical to one created with them enabled.
      Without this check a PostgreSQL database silently ships missing columns.
- [ ] 7. Add `internal/db/dialect_postgres.go`: `pgx/v5/stdlib`, session pinned to UTC,
      `TIMESTAMPTZ` for the timestamp type, `INTEGER` for the boolean type, `SecretKeyDir`
      returning the empty string, `RunsLegacyMigrations` returning false.
- [ ] 8. Change `NewDB(dbPath string)` to `NewDB(cfg Config)` and add the path-shaped helper the
      ~80 existing test call sites keep using unchanged.
- [ ] 9. Route the encryption key through `dialect.SecretKeyDir` instead of
      `filepath.Dir(dbPath)`. Under PostgreSQL an absent key logs the existing warning, serves on,
      and **never** generates or writes a key file.
- [ ] 10. Read `DB_DRIVER` and `DATABASE_URL` in `cmd/server/main.go`. Skip `resolveDBPath`
      entirely under PostgreSQL, refuse to start on an unknown driver, a missing DSN or an
      unreachable server, and make the startup log report the engine without ever printing the
      DSN password.
- [ ] 11. Add the smoke tests, skipped when no test database is configured: schema creation from
      empty, task CRUD round trip, `ON CONFLICT` upsert, timestamp round trip, second start on an
      initialised database.
- [ ] 12. Add the `postgres:16` service and the PostgreSQL smoke job to `.gitlab-ci.yml`. Do
      **not** carry `allow_failure: true` from `test:go`: the new job starts green and must gate.
- [ ] 13. Add the migration entry point: destination emptiness check, encryption-key precondition
      check before the first row, copy in foreign-key order (`projects`, `users`, `tasks` before
      their dependants), skipping `daily_digests`, leaving the source untouched, and a per-table
      row-count report.
- [ ] 14. Add the migration round-trip test: seed a SQLite database including one server-key-sealed
      credential and one passphrase-sealed credential, migrate, assert row counts per table and
      that both credentials still open.
- [ ] 15. Document `DB_DRIVER`, `DATABASE_URL` and the `SECTILE_SECRET_KEY` requirement in
      `.env.sample`, the `Dockerfile` and the deployment section of `README.md`. State plainly that
      under PostgreSQL the key must be supplied and is never generated.
- [ ] 16. Update `CHANGELOG.md` and `docs/ARCHITECTURE.md` where they describe persistence.
- [ ] 17. Run `make test` and fix until green. Run the PostgreSQL smoke subset against a local
      `postgres:16` container.
- [ ] 18. Review the diff, commit, push and update pull request #297.
