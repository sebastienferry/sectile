# #407: Technical plan

References: [`spec.md`](spec.md), [`inventory.md`](inventory.md),
[`docs/clarifications/407.md`](../../docs/clarifications/407.md).

## Stack

Go, `internal/db` behind the `dialect` seam (ADR 0016), numbered migrations
(ADR 0021), pgx v5 stdlib and modernc SQLite. No new dependency (`pgconn` already
comes with pgx).

## Principles (clarification D1-D5, R3-1..R3-3)

- `DB.mu` stays as the in-process serializer; after this change no invariant depends
  on it.
- Conversion order, first that fits: (1) one SQL statement, relying on the implicit
  row lock of `UPDATE` under `READ COMMITTED`; (2) conditional `UPDATE ... WHERE
  <expected state>` with a `RowsAffected` check; (3) a transaction that locks the row
  with `SELECT ... FOR UPDATE` and does the check inside. A unique constraint or
  `INSERT ... ON CONFLICT` counts as safe.
- No row lock and no open transaction across a tracker or forge HTTP call.
- Jobs (`enqueueJob`, `pushTrackerOpJob`) are pushed **after** `Commit`, never inside
  the transaction: a rolled-back transaction must not leave a job behind.
- Isolation stays the default (`READ COMMITTED`). `SERIALIZABLE` is rejected (R3-2):
  retry loops everywhere, and the SQLite drivers refuse non-default levels.

## Seam additions (`internal/db/dialect.go`, `dialect_sqlite.go`, `dialect_postgres.go`)

- `ForUpdate() string`: `" FOR UPDATE"` on PostgreSQL, `""` on SQLite (its single
  writer plus `DB.mu` already serialize). Wrapped by `d.forUpdate()` like
  `d.lowerASCII`.
- `AcquireProjectWorker(conn *sqlConn, projectID string) (release func(), err error)`:
  - SQLite: returns a no-op release immediately.
  - PostgreSQL: loop { reserve a connection with `conn.db.Conn(ctx)`; `SELECT
    pg_try_advisory_lock($1, $2)` with `$1 = projectWorkerLockClass` (int32
    constant `0x5EC7`) and `$2 = int32(fnv32a(projectID))`; if acquired, return a
    release that runs `pg_advisory_unlock($1, $2)` then closes the connection; else
    close the connection and sleep `projectWorkerRetry` (500 ms, ±20 % jitter,
    package variable for tests) }.
  - The two-int4 key space does not overlap the bigint `migrationLockKey`
    (PostgreSQL documents the two spaces as distinct). A hash collision between two
    projects only serializes them: accepted.
  - If the held connection dies, PostgreSQL releases the lock; the release logs the
    unlock error and closes. Accepted (R3-3).
- `IsUniqueViolation(err error, index string) bool` (free function in a new
  `internal/db/uniqueviolation.go`): PostgreSQL `errors.As(err, *pgconn.PgError)`,
  `Code == "23505"` and `ConstraintName == index`; SQLite, the error text contains
  `UNIQUE constraint failed: task_activities.task_id` for the run index (map the
  index name to its column list). It must not match a primary-key collision.

## Transaction helper (`internal/db/conn.go`)

`func (d *DB) withTx(fn func(tx *sqlTx) error) error`: `Begin`, `fn`, `Commit`, or
`Rollback` on error or panic (re-panics). The seven hand-written `Begin` sites stay
as they are unless touched.

## Schema: migration 9 `one_active_run`

Next free number on `main` is 9 (8 is `agent_presence`). Renumber at merge time if
another migration lands first. Never edit the baseline `CREATE TABLE`.

```sql
ALTER TABLE task_activities ADD COLUMN concurrent INTEGER NOT NULL DEFAULT 0;

UPDATE task_activities SET concurrent = 1
WHERE <P> AND id <> (
  SELECT k.id FROM task_activities k
  WHERE k.task_id = task_activities.task_id AND <P over k>
  ORDER BY (k.skill_id = 'remote_run') DESC, k.started_at IS NULL, k.started_at, k.created_at, k.id
  LIMIT 1);

CREATE UNIQUE INDEX idx_activities_one_active_run ON task_activities (task_id) WHERE <P>;
```

with the predicate `<P>` written as literals only (immutable, valid on both engines):

```sql
task_id IS NOT NULL
AND concurrent = 0
AND status IN ('queued', 'pending', 'running')
AND skill_id IN ('remote_run', 'clarify', 'specify', 'implement', 'adjust', 'handoff',
                 'create_pr', 'pickup', 'rewrite_story', 'refine_macro', 'pickup_issues',
                 'review', 'pick')
```

- The migration runs before the restart sweep (`migrateSchema` precedes
  `recoverInterruptedRuns`), so it sees the previous process's stale rows; marking
  them `concurrent = 1` instead of canceling them (Round 4) leaves the sweep's
  outcome unchanged.
- `agent_launch`, `tracker_op`, `tracker_update`, `sync_*`, `convert`, `postback`,
  `install_spec_framework` are not runs and stay outside `<P>`.
- The skill list is frozen in the migration. A new catalog skill id needs a later
  migration that recreates the index; a test fails when `skills.StageSkills` holds
  an id missing from the Go constant below.
- In Go, `activeRunSkillIDs` and `activeRunStatuses` constants build the predicate
  used by `ActiveRunOnTask` (without the `concurrent = 0` term, FR5) and by the
  tests.

## Q4: one ordinary run per task

- `models.TaskActivity` gains `Concurrent bool`; `insertTaskActivity` writes it.
- `var ErrTaskBusy` plus `type TaskBusyError struct{ Active *models.TaskActivity }`
  (`Is(ErrTaskBusy)`), built after a violation by re-reading `ActiveRunOnTask`.
- `ActiveRunOnTask`: predicate `status IN ('queued','pending','running') AND skill_id
  IN (...)`; drops the legacy `action` clause; order `started_at IS NULL, started_at,
  created_at` (a running run is reported before a queued one).
- Insert paths:
  | Path | `concurrent` | On violation |
  |---|---|---|
  | `enqueueSkillOnTask` (`db.go:~4971`), used by `EnqueueFullChainRun`, `EnqueueSkillOnTaskWithOverrides`, `RetryActivity`, `enqueueChainStep` | 0 | return `*TaskBusyError` **before** `enqueueJob`; stop ignoring the insert error |
  | Handler run-skill: `StartAgentRun` (`handlers.go:~2100`) | 0, or 1 with `force` | 409 body; insert the `remote_run` **before** the `agent_launch` row so a refusal records nothing |
  | `processSkillJob` → `StartAgentRun` (`db.go:~4071`), after the queued row is renamed `agent_launch` | 0 | mark the job's `agent_launch` row failed with a readable reason; check the rename's error instead of discarding it |
  | `startRemoteRun` from MCP `start_run` without `runId` | 1 | cannot happen |
  | `startRemoteRun` with a launcher's `runId` | no insert | n/a |
  | `SyncRemoteRunStatusFor` insert branch (`remoterun.go:~298`) | 1 | cannot happen |
- Handlers `EnqueueFullChainRun` (`handlers.go:~2474`), skill enqueue with overrides
  (`~2489`) and retry (`~2995`) map `ErrTaskBusy` to the 409 body; other errors keep
  their current status.
- `describeActiveRun`: when the active run's status is `queued` or `pending`, answer
  "A run of %s is queued on this task." (English, like the existing message).
- The busy pre-check in the handler stays: it gives the 409 without a failed insert
  in the common case, and the index covers the race.

## Q3: project worker (`internal/db/db.go` `startQueueWorker`)

Around `runJobGuarded` for non-`tracker_op` jobs: keep `d.limiter.Acquire/Release`
(it keeps one replica from try-locking against itself), then
`release, err := d.dialect.AcquireProjectWorker(d.conn, projID)`; on error, log and
run the job anyway (degraded to today's per-replica behaviour, never a stuck
queue); `defer release()`. The job stays `queued` while waiting: `processSkillJob`
sets `running` only after this point.

## Other conversions

The per-section decision is in [`inventory.md`](inventory.md). Groups:

- **Task row lock** (`SELECT ... FROM tasks WHERE id = ?` + `ForUpdate`, inside
  `withTx`, all derived values recomputed from the locked row):
  `TransitionTaskStageBy` (existing transaction; `managedStageRunningUnsafe` moves
  inside it; the `stage.go:30` snapshot is re-read locked), `updateTaskBy`,
  `MoveTask`, `MoveTaskToTrackerStatus`, `PostBackTask`, `applyDiscoveredPullRequests`
  (re-read `pr_links`, `branch_name`, `pr_links_detached`), `applyPullRequestStates`,
  `adjustmentPrerequisite`, `SetTaskPinnedBy` / `ToggleTaskPinnedBy` (flip decided
  inside the transaction), `appendActivityStep` and `finishTrackerOp` (lock the
  activity row; steps stay a Go-appended JSON array).
  JSON appends in SQL were rejected for `pr_links` and `steps`: dedup by URL,
  normalization and the detached flag cannot be expressed portably, and the two
  engines' JSON functions differ.
- **Single statement**:
  - `AppendRemoteRunOutput`:
    `SET output = CASE WHEN output LIKE '%' || :marker THEN output
     WHEN LENGTH(output) + LENGTH(CAST(? AS TEXT)) > :limit
       THEN SUBSTR(output || CAST(? AS TEXT), 1, :limit) || :marker
     ELSE output || CAST(? AS TEXT) END
     WHERE id = ? AND task_id = ? AND skill_id = 'remote_run'`;
    zero rows means not found. `LENGTH`/`SUBSTR` count characters on both engines,
    which fixes the byte cut that can split a UTF-8 character (rejected by
    PostgreSQL). No negative `SUBSTR`.
  - `finishRemoteRun`: summary joined in SQL with `CASE WHEN summary LIKE <silence
    prefix pattern> THEN summary || <existing separator> || ? ELSE ? END`, keeping the existing
    `status = 'running'` condition.
  - `UpdateUserSettings`, `UpdateBoardView`, `saveMacroMetaFull`,
    `writeTaskParentLocally`, `MigrateMacro` (copy `INSERT ... SELECT` + `DELETE` in
    one transaction): write only the fields the request carries, as shown in the
    inventory.
- **Conditional update**:
  - `SyncRemoteRunStatusFor`: running/queued `UPDATE ... WHERE id = ? AND status NOT
    IN ('completed','failed','canceled')`; zero rows and the row absent →
    `INSERT ... ON CONFLICT (id) DO NOTHING` with `concurrent = 1`; zero rows and the
    row terminal → ignored (FR9). Terminal → terminal stays allowed.
  - `ConvertTaskToRemote`: claim `UPDATE tasks SET source = 'converting' WHERE id = ?
    AND source = 'local'`; zero rows → refuse ("already being converted or not
    local"); then `CreateIssue` with no lock; then the final `UPDATE ... WHERE id = ?
    AND source = 'converting'`; on tracker failure, reset `source = 'local'` with the
    same condition. Readers treat `converting` as local.
  - `ToggleProjectBookmark`: `DELETE`; if zero rows, `INSERT ... ON CONFLICT DO
    NOTHING`.
- **Sentinel lock for the default project**: `CreateProjectAs`, `UpdateProjectAs`
  (when `isDefault` changes, plus the projects row for the field merge),
  `DeleteProject`: lock `settings` row `id = 1` in the transaction, then clear and set
  `is_default` (and for delete, recheck and delete) inside it. The settings row is
  guaranteed by the baseline seed; the transaction inserts it with `ON CONFLICT DO
  NOTHING` first if absent.
- **`UpdateSettings`**: lock `settings` row `id = 1`, merge inside.
- **`CreateTaskAs`**: `CreateIssue` moves before any lock (D4, FR11); the local key
  and position are computed inside a transaction that locks the project row; a
  duplicate position is cosmetic and accepted.
- **`storeTeamMembers`**: lock the team row; `DELETE` + `INSERT`s in one transaction.
- **Accepted as is, documented**: `ImportOrUpdateTasks` (unique keys turn a race into
  a reported error; project syncs are serialized by Q3), `EnsureDefaultBookmark`,
  `CreateBoardView` (unique index; map the violation to `ErrBoardViewNameTaken`),
  the three MEM sections (#409).

## Audit document

`docs/db-concurrency-audit.md`: generated from [`inventory.md`](inventory.md) and
updated with the final line numbers and outcomes. Linked from
`docs/ARCHITECTURE.md` if it covers the store, and referenced later by #410's ADR.

## Tests

- Helper `openPostgresPair(t) (a, b *DB)` in a `postgres_concurrency_test.go`: first
  handle through `openPostgres` (truncates), second through `Open` (like the
  autosync, bus and instances tests). Skips without `SECTILE_TEST_POSTGRES_DSN`.
- Races use N goroutines split across both handles, started by a closed channel.
- SQLite tests cover the index, the migration keeper, busy mapping and the handler
  bodies without PostgreSQL.

## Target files

- `internal/db/dialect.go`, `dialect_sqlite.go`, `dialect_postgres.go`, `conn.go`
- `internal/db/uniqueviolation.go` (new)
- `internal/db/migrations.go` (migration 9)
- `internal/db/db.go`, `stage.go`, `board.go`, `postback.go`, `prdiscovery.go`,
  `prstates.go`, `adjustment.go`, `pins.go`, `bookmarks.go`, `remoterun.go`,
  `skillresult.go`, `trackerops.go`, `macros.go`, `usersettings.go`, `boardviews.go`,
  `teams.go`, `chain.go`
- `internal/models/models.go` (`Concurrent`)
- `internal/handlers/handlers.go` (409 mapping, `force`, launch order,
  `describeActiveRun`)
- `internal/taskmcp/server.go` if `start_run` needs to pass `concurrent`
- tests: `internal/db/postgres_concurrency_test.go` (new),
  `internal/db/activerun_test.go` (new), `internal/db/agentlaunch_test.go` and other
  tests inserting two active runs, handler tests for the 409 bodies
- `docs/db-concurrency-audit.md` (new), `CHANGELOG.md`

## Risks

- **Rolling deploy on PostgreSQL:** an older replica still running inserts a second
  ordinary run and gets a raw unique-violation error until it is replaced. Accepted:
  short window, the error is visible and nothing is corrupted.
- **Catalog drift:** a new skill id not in the index predicate escapes the rule
  until a migration adds it; guarded by the catalog test.
- **Queued rows now block launches (Q6):** a job stuck in `queued` blocks the task
  until it is canceled or reclaimed by the restart sweeps; the 409 names it and the
  cancel action already exists.
- **Advisory lock connections:** each running project job pins one of the 25 pool
  connections of its replica. Accepted: the per-replica limiter caps it at one per
  project.
- **Stacked migration numbers:** renumber 9 if `main` takes it first.
