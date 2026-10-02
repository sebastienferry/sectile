# Tasks #693 - A PostgreSQL database must be UTF8

Ordered checklist. Each group leaves the tree buildable.

## 1. Tests first

- [ ] T1.1 `TestRequireUTF8` and `TestPostgresRefusesANonUTF8Database`; the
  integration test fails on the current code (the database opens).

## 2. Check

- [ ] T2.1 `CheckEncoding` on the dialect interface, SQLite no-op,
  PostgreSQL query and `requireUTF8` (FR1, FR2, FR4).
- [ ] T2.2 `openWith` calls it after `Ping`, before the migration (FR1, FR3).

## 3. Test helpers

- [ ] T3.1 `openPostgres`, `openPostgresInstance`: message naming
  `SECTILE_TEST_POSTGRES_DSN` (FR5).
- [ ] T3.2 Multi-replica harness pre-flight (FR6).

## 4. Docs and changelog

- [ ] T4.1 README, `docs/TESTING.md`, `remoterun.go` comment (FR7).
- [ ] T4.2 One `Changed` line under `## [Unreleased]` (FR8).

## 5. Verification

- [ ] T5.1 `go build ./...`, `go vet ./internal/db/`.
- [ ] T5.2 SQLite suite of `internal/db`, `internal/handlers`, `cmd/server`.
- [ ] T5.3 PostgreSQL suite on a UTF8 cluster, `-p 1`, including
  `TestPostgresConcurrentOutputAppendsTruncateOnce`.
- [ ] T5.4 The helpers on a SQL_ASCII `SECTILE_TEST_POSTGRES_DSN`: explicit
  failure.
