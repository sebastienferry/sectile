# #403: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Schema and seam (FR2, FR3)

- [x] T1.1 Migration 3 `server_instances`: `task_activities.instance_id`, table `server_instances`.
- [x] T1.2 `dialect.ServesOneProcess()`: SQLite `true`, PostgreSQL `false`.

## 2. Instance lifecycle (FR1, FR2, FR5, FR7)

- [x] T2.1 `instances.go`: `instanceID` set in `openWith`, `InstanceID()`, bounds as package variables.
- [x] T2.2 `reclaimDeadInstances(now)` with the three idempotent statements.
- [x] T2.3 `recoverInterruptedRuns` moved to `instances.go`, engine-dependent per FR4.
- [x] T2.4 `StartInstance()`: register, heartbeat (re-register on a missing row), reclaim ticker, `stop`.

## 3. Stamping (FR3)

- [x] T3.1 `insertTaskActivity` writes `instance_id`; all callers pass the store's id.
- [x] T3.2 Job start stamps `instance_id`.

## 4. Server wiring (US4)

- [x] T4.1 `cmd/server/main.go` calls `StartInstance` after `db.Open`; `sectile-migrate` untouched.

## 5. Tests

- [x] T5.1 Existing restart tests unchanged and green (`TestRestartRecoveryRunsOnEveryStart`, `TestServerRestartPreservesRemoteExecutionOwnership`, `TestPostgresRecoversInterruptedRuns`).
- [x] T5.2 SQLite: `reclaimDeadInstances` reclaims work of a stale instance, of an unknown instance and with an empty owner; leaves a live instance's work, agent-owned runs and finished work untouched; deletes only stale rows; second call is a no-op.
- [x] T5.3 SQLite: `AddTaskActivity` and job start stamp the store's instance id.
- [x] T5.4 SQLite: `StartInstance` registers a row; heartbeat refreshes it; a deleted row is re-registered; `stop` removes it; `NewDB` alone registers nothing.
- [x] T5.5 SQLite: reopening the store removes every instance row and reclaims everything (US3).
- [x] T5.6 PostgreSQL (skipped without DSN): a live instance's running job and client run survive another `Open`; after the instance goes stale, `Open` reclaims them.

## 6. Documentation

- [x] T6.1 Update the documentation where it describes restart recovery: `docs/ARCHITECTURE.md` does not describe it; the README section on MCP sessions does, and is updated.
- [x] T6.2 `CHANGELOG.md` `[Unreleased]` `Fixed`: a second server on PostgreSQL, or a rolling deploy, no longer interrupts the running server's work.

## Test plan

`go build ./...`, `go vet ./...`, `go test ./internal/db/... ./cmd/...`, then the full
`go test ./...`; PostgreSQL subset with a throwaway `SECTILE_TEST_POSTGRES_DSN` when
available.
