# #404: Technical plan

References: [`spec.md`](spec.md), [`docs/clarifications/404.md`](../../docs/clarifications/404.md).

## Schema: migration 6 `auto_sync_state` (renumbered from 4 after main took 3 and 4)

```sql
CREATE TABLE auto_sync_projects (
    project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    last_pass_at DATETIME,
    last_full_sync_at DATETIME
);
CREATE TABLE auto_sync_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    backoff_until DATETIME,
    last_run_at DATETIME,
    last_error TEXT NOT NULL DEFAULT '',
    last_imported INTEGER NOT NULL DEFAULT 0,
    passes INTEGER NOT NULL DEFAULT 0,
    imported INTEGER NOT NULL DEFAULT 0
);
INSERT INTO auto_sync_state (id) VALUES (1);
```

All times are `time.Now().UTC()`. `forgetSchemaVersion` (test helper) drops both tables.

## `internal/db/autosync.go`

- `autoSync` keeps only `mu` and `running` (per-process overlap guard).
- `autoSyncPacing` struct: `lastPass`, `lastFull` as `time.Time` (zero when absent).
- `claimAutoSyncPass(projectID string, interval time.Duration, now time.Time) (autoSyncPacing, bool, error)`:
  1. `INSERT INTO auto_sync_projects (project_id) VALUES (?) ON CONFLICT DO NOTHING`
  2. `SELECT last_pass_at, last_full_sync_at` (scanned as `sql.NullTime`)
  3. `UPDATE auto_sync_projects SET last_pass_at = ? WHERE project_id = ? AND (last_pass_at IS NULL OR last_pass_at < ?)` with `now - interval`
  4. claimed when one row was affected; returns the pacing read in step 2.
- `autoSyncWindowFrom(p autoSyncPacing, now time.Time) int`: today's `autoSyncWindow` rules as a pure function.
- `runAutoSyncPass`: per project, tracker checks first (unchanged), then the claim, then the window, then `EnqueueSyncWith`. At the end: `UPDATE auto_sync_state SET last_run_at = ?, passes = passes + 1` and, on failures, `last_error`.
- `recordAutoSyncPass`: `UPDATE auto_sync_state SET last_imported = ?, imported = imported + ?, last_error = ?`; on success of a full read, upsert `last_full_sync_at`. No longer skipped when the loop has not started in this process.
- `enterAutoSyncBackoff`: `UPDATE auto_sync_state SET backoff_until = ?` (now + 10 min).
- Loop: reads `backoff_until` from the row instead of memory.
- `recordAutoSyncError`: `UPDATE auto_sync_state SET last_error = ?`.
- `AutoSyncStatus`: reads the row; `Running` from memory.
- No `d.mu` in these statements (clarification D6).

## Tests (`internal/db/autosyncwindow_test.go`, new `autosyncreplicas_test.go`)

- Helpers `setAutoSyncLastPass(t, d, projectID, at)` and `autoSyncLastFull(t, d, projectID) (time.Time, bool)` replace direct map writes.
- Two `*DB` on one SQLite file (same process): both run a pass on a due project, one sync queued in total.
- Claim refused within the interval, accepted after it.
- A backoff entered through one store stops the other's loop check and appears in its status.
- A full read dated by one store makes the other's next window incremental.
- `autoSyncWindowFrom` table test.

## Target files

`internal/db/migrations.go`, `internal/db/autosync.go`, `internal/db/migrations_test.go`,
`internal/db/autosyncwindow_test.go`, `internal/db/prdiscovery_test.go`,
`internal/db/autosyncreplicas_test.go` (new), `CHANGELOG.md`.

## Risks

- Two stores on one SQLite file in tests exercise the SQL, not PostgreSQL concurrency;
  the claim statement is one conditional `UPDATE`, which both engines execute atomically.
  A PostgreSQL variant of the claim test runs when a DSN is set.
