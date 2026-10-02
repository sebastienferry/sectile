# Tasks #675 - A finished run deleted from the activities view stays deleted

Ordered checklist. Each group leaves the tree buildable.

## 1. Tests first

- [ ] T1.1 Add the five tests of `plan.md` §5 and confirm the deletion tests
  fail on the current code (the run is recreated).

## 2. Schema

- [ ] T2.1 Migration 46 `deleted_remote_runs` in `internal/db/migrations.go`.
- [ ] T2.2 Update any test that pins the latest schema version.

## 3. Behaviour

- [ ] T3.1 `DeleteActivity` and `ClearCompletedActivities` record finished
  remote runs and purge records older than 30 days (FR1 to FR3, FR6).
- [ ] T3.2 `SyncRemoteRunStatusFor` skips the insert of a recorded id (FR4,
  FR5), with its doc comment updated.

## 4. Changelog

- [ ] T4.1 One `Fixed` line under `## [Unreleased]` (FR8).

## 5. Verification

- [ ] T5.1 `go build ./...`, `go vet ./internal/db/...`.
- [ ] T5.2 `go test ./internal/db/... ./internal/handlers/...` on SQLite.
- [ ] T5.3 The new tests and the migration tests on PostgreSQL
  (`SECTILE_TEST_POSTGRES_DSN`, UTF8 throwaway cluster).
