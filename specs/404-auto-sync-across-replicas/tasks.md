# #404: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Schema (FR1, FR4, FR5)

- [x] T1.1 Migration 7 `auto_sync_state` (renumbered from 4, then 6): both tables and the seeded row.
- [x] T1.2 `forgetSchemaVersion` drops them.

## 2. Loop (FR2 to FR6)

- [x] T2.1 `claimAutoSyncPass` and `autoSyncWindowFrom`.
- [x] T2.2 `runAutoSyncPass` claims before queueing; records last run, passes, errors in the row.
- [x] T2.3 `recordAutoSyncPass`, `enterAutoSyncBackoff`, `recordAutoSyncError` persist.
- [x] T2.4 The loop reads the backoff from the row; `AutoSyncStatus` reads the row.
- [x] T2.5 `autoSync` reduced to the per-process `running` guard.

## 3. Tests

- [x] T3.1 Existing auto-sync tests rewritten to set rows instead of maps; all green.
- [x] T3.2 Two stores, one database: one sync queued per due project.
- [x] T3.3 Claim refused within the interval, accepted after.
- [x] T3.4 Backoff shared between stores, visible in both statuses.
- [x] T3.5 Full read dated by one store narrows the other's next window.
- [x] T3.6 `autoSyncWindowFrom` table test.
- [x] T3.7 PostgreSQL claim test (skipped without DSN).

## 4. Documentation

- [x] T4.1 `CHANGELOG.md` `[Unreleased]` `Fixed`: with several servers, each project is synchronised once per interval and a rate limit pauses all of them.

## Test plan

`go build ./...`, `go vet ./...`, `go test ./...`, PostgreSQL subset with a throwaway DSN.
