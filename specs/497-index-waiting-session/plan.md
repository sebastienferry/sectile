# Plan #497 - Index task_activities.waiting_session

## Stack

Go, `internal/db`, SQLite and PostgreSQL through the numbered migrations.

## Design

- Migration 44 (next free number; 39 to 43 went to #635, #654 and #663), `task_activities.waiting_session_index`:
  `CREATE INDEX IF NOT EXISTS idx_task_activities_waiting_session ON
  task_activities (waiting_session) WHERE waiting_session <> '';` Both engines
  accept the same statement.
- `ResumeWaits` select gains `AND waiting_session <> ''`. The query text is
  kept in a package constant so the plan test reads the exact statement.
- Plan test: SQLite `EXPLAIN QUERY PLAN <query>` must mention the index;
  PostgreSQL, inside a transaction, `SET LOCAL enable_seqscan = off` then
  `EXPLAIN <query>` must mention it (a small table would otherwise be scanned
  whatever the index).
- Rewind helpers: an index needs no column drop; check that the rewind tests
  that replay migrations still pass and adjust any that compare the list of
  indexes.

## Target files

- `internal/db/migrations.go`
- `internal/db/remoterun.go`
- `internal/db/waiting_session_index_test.go` (new)
