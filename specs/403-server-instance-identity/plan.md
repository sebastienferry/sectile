# #403: Technical plan

References: [`spec.md`](spec.md), [`docs/clarifications/403.md`](../../docs/clarifications/403.md).

## Stack

Go, `internal/db` (both engines behind the `dialect` seam, ADR 0016), numbered
migrations (ADR 0021). No new dependency.

## Schema: migration 3 `server_instances`

```sql
ALTER TABLE task_activities ADD COLUMN instance_id TEXT NOT NULL DEFAULT '';
CREATE TABLE server_instances (
    id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL DEFAULT '',
    pid INTEGER NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL,
    last_seen DATETIME NOT NULL
);
```

`DATETIME` is rewritten to `TIMESTAMPTZ` by `RewriteDDL`. All times written and
compared are `time.Now().UTC()`.

## Dialect

`ServesOneProcess() bool`: `true` for SQLite, `false` for PostgreSQL. A behaviour
question rather than `Engine()`, as the seam asks.

## `internal/db/instances.go` (new)

- `DB.instanceID string`, set in `openWith` with `uuid.NewString()`.
- `InstanceID() string`.
- Constants `instanceHeartbeatEvery = 10s`, `instanceDeadAfter = 45s`,
  `instanceReclaimEvery = 15s`, package variables so tests can shorten them.
- `recoverInterruptedRuns()` (existing name kept, called from `openWith` as today):
  - `ServesOneProcess()`: the two current global `UPDATE`s, then `DELETE FROM server_instances`.
  - otherwise: `reclaimDeadInstances(time.Now().UTC())`.
- `reclaimDeadInstances(now) (int64, error)`: with `cutoff = now - instanceDeadAfter`,
  and `orphan = (instance_id = '' OR instance_id NOT IN (SELECT id FROM server_instances WHERE last_seen >= ?))`:
  1. `UPDATE task_activities SET status='failed', error='Interrupted by server restart'
     WHERE status IN (...) AND skill_id != 'remote_run' AND <orphan>`
  2. `UPDATE ... SET status='canceled', summary=?, completed_at=?, waiting_since=NULL
     WHERE status IN (...) AND skill_id='remote_run' AND action != RunActionAgent AND <orphan>`
  3. `DELETE FROM server_instances WHERE last_seen < ?`
  Each statement is idempotent: two survivors running it together converge.
- `StartInstance() (stop func(), err error)`: inserts the row (hostname from
  `os.Hostname`, `os.Getpid`), then one goroutine with two tickers:
  - heartbeat: `UPDATE server_instances SET last_seen=? WHERE id=?`; zero rows
    affected re-inserts the row and logs that the instance had been declared dead;
  - reclaim: `reclaimDeadInstances`; a non-zero count is logged.
  `stop` ends the goroutine and deletes the row (used by tests; the server never
  calls it, see D8 of the clarification).

## Stamping

- `insertTaskActivity(conn, act)` gains the owner column. It is a free function used
  with a transaction in places, so it takes the id: `insertTaskActivity(conn, instanceID, act)`,
  callers pass `d.instanceID`.
- Job start (`db.go`, "Mark activity as running"): `SET status='running', started_at=?, instance_id=?`.
- `SyncRemoteRunStatusFor` insert branch creates agent-owned runs only: not stamped,
  as they are never reclaimed.

## Server

`cmd/server/main.go`: right after `db.Open`, `if _, err := database.StartInstance(); err != nil { log.Fatalf(...) }`
and log the instance id. `sectile-migrate` does not call it.

## Target files

- `internal/db/migrations.go` (migration 3)
- `internal/db/dialect.go`, `dialect_sqlite.go`, `dialect_postgres.go`
- `internal/db/instances.go` (new), `internal/db/instances_test.go` (new)
- `internal/db/db.go` (`openWith`, `recoverInterruptedRuns` moved, `insertTaskActivity`, job start)
- `internal/db/postgres_instances_test.go` (new, skipped without DSN)
- `cmd/server/main.go`
- `docs/ARCHITECTURE.md` if it describes the restart sweep; `CHANGELOG.md` `[Unreleased]`

## Risks

- A process whose heartbeat stalls over 45 s (database outage) is declared dead
  while alive; its jobs are marked failed and may be overwritten by its own final
  update. It re-registers on the next heartbeat. Accepted and logged; the status guard
  comes with #405.
- A rolling upgrade from the previous version cancels the old pod's work once (empty
  owner). Accepted in the clarification (D5).
- Migration number 3 may collide with another branch; renumber on rebase.
