# Database concurrency audit

`DB.mu` is the store's process mutex. Since #407 no invariant depends on it:
several server processes may share one PostgreSQL database, and each
read-decide-write the mutex used to protect is made correct by the database
itself. The mutex stays as the in-process serialiser (SQLite has one writer, and
it guards process memory). SQLite remains single-process (#403).

This document lists every section the mutex protects, its class, and what holds
it across processes. It was taken on `main` at `6dac2fc` (137 `d.mu.Lock()` /
`d.mu.RLock()` sites in non-test files under `internal/db`); line numbers are
those of that commit and only locate the section, the function name is the
reference. Keep it current: a new read-decide-write on the store gets a row here
and one of the mechanisms below.

## Mechanisms

In order of preference:

1. **One statement** (**SQL-1**). An `UPDATE` takes an implicit row lock; under
   PostgreSQL's default `READ COMMITTED`, a concurrent `UPDATE` of the row waits,
   then re-evaluates its `WHERE` on the committed version. Appends and merges
   written in SQL need nothing more.
2. **Conditional update** (**COND**): `UPDATE ... WHERE <expected state>` with a
   `RowsAffected` check.
3. **Row lock** (**LOCK**): a transaction reads the row with
   `SELECT ... FOR UPDATE` (`dialect.ForUpdate`, empty on SQLite), decides, then
   writes through the same transaction. Helpers: `sqlConn.WithTx`,
   `DB.lockTaskUnsafe`, `DB.lockActivityStepsUnsafe`, `DB.lockSettingsUnsafe`.
   Queue pushes happen after the commit; no lock is held across a tracker or forge
   HTTP call; inside the transaction, writes go through it only. SQLite opens
   every transaction `IMMEDIATE` (`_txlock=immediate`): a deferred one that reads
   then writes fails at once when another connection wrote in between.
4. **Constraint** (**Q4**): the partial unique index
   `idx_activities_one_active_run` (migration 9) allows one ordinary active run
   per task. Forced launches, client-declared runs and agent-reported runs carry
   `task_activities.concurrent = 1` and stay out of it.
5. **Advisory lock** (**Q3**): `dialect.AcquireProjectWorker` takes
   `pg_try_advisory_lock(0x5EC7, fnv32a(project))` on a reserved connection, so one
   server-side job per project runs across all instances.

`SERIALIZABLE` was rejected: every transaction would need a retry loop on
`40001`, and the SQLite drivers refuse non-default isolation levels. See
[the specification](../specs/407-database-concurrency-guarantees/plan.md).

Classes: READ 54, SINGLE 47, RMW 18, RCW 15, MEM 3.

- **READ**: reads only.
- **SINGLE**: one statement, or statements whose correctness does not depend on the mutex.
- **RMW**: read, modify in Go, write back (lost update).
- **RCW**: read, decide in Go, write (double apply).
- **MEM**: also guards process memory.

## Sections converted (RMW, RCW, Q4)

| file:line | function | class | read-then-written | outcome |
|---|---|---|---|---|
| stage.go:132 | TransitionTaskStageBy | RCW | running guard outside the tx; labels, status, pr_links, branch from the unlocked snapshot at :30; UPDATE without a stage condition | LOCK tasks row in the existing tx; guard and derived values recomputed from the locked row |
| db.go:2672 | updateTaskBy | RMW | whole task row merged in Go (labels, pr_links, status); `projects.repo_paths` | LOCK tasks row (plus projects row for repo_paths); jobs pushed after commit |
| db.go:3087 | MoveTask | RMW | labels recomputed from the row read | LOCK tasks row; the `position + 1` shift stays one statement |
| board.go:275 | MoveTaskToTrackerStatus | RMW | labels and status from a task read without the lock | LOCK tasks row |
| postback.go:85 | PostBackTask | RCW | running-stage guard, then labels, status, pr_links merged | LOCK tasks row, guard inside the tx |
| prdiscovery.go:179 | applyDiscoveredPullRequests | RMW | pr_links and branch from a snapshot taken before the forge calls; detached flag checked earlier | LOCK tasks row; pr_links and branch_name re-read inside. The detached flag is still read before discovery: a detach made during a discovery is overridden by it, as it was on one process |
| prstates.go:70 | applyPullRequestStates | RMW | pr_links states changed in Go, whole set written | LOCK tasks row |
| adjustment.go:58 | adjustmentPrerequisite | RMW | pr_links appended from the caller's snapshot | LOCK tasks row; re-read pr_links inside |
| pins.go:193 | SetTaskPinnedBy | RMW | labels plus or minus the pinned label | LOCK tasks row |
| pins.go:281 | ToggleTaskPinnedBy | RCW | reads the pin state, then sets | LOCK tasks row; flip decided inside the transaction of `setTaskPinned`, which the toggle now runs as the acting user |
| remoterun.go:416 | AppendRemoteRunOutput | RMW | `output = current + chunk`, truncated in Go by bytes | SQL-1 `CASE` update, character limit |
| macros.go:647 | appendActivityStep | RMW | steps JSON appended in Go | LOCK activity row |
| trackerops.go:826 | finishTrackerOp | RMW | steps JSON appended in Go (status guard from #405) | LOCK activity row |
| remoterun.go:189 | finishRemoteRun | RMW | summary joined in Go from an unlocked read | SQL-1 `CASE` on the summary, `status = 'running'` kept |
| remoterun.go:255 | SyncRemoteRunStatusFor | RCW | exists check, then UPDATE with no status guard or INSERT | COND `status NOT IN (terminal)`; else `INSERT ... ON CONFLICT (id) DO NOTHING`, `concurrent = 1` |
| db.go:4971 | enqueueSkillOnTask | RCW | queued skill inserted with no active-run check; insert error ignored | Q4 (`concurrent = 0`), `ErrTaskBusy` before `enqueueJob` |
| skillresult.go:28 | ActiveRunOnTask | READ | | Q4 predicate (queued counts, legacy action clause dropped) |
| db.go:3336 | AddTaskActivity | SINGLE | | Q4 via `insertTaskActivity` (startRemoteRun, StartAgentRun) |
| db.go:2362 | CreateTaskAs | RCW | key and position as MAX+1; tracker `CreateIssue` over HTTP under the lock | CreateIssue before any lock; LOCK projects row around MAX+1 plus INSERT; duplicate position accepted |
| db.go:5502 | ConvertTaskToRemote | RCW | issue created without a claim, then key, source, labels from the snapshot | COND claim `source 'local' -> 'converting'`, final UPDATE `WHERE source = 'converting'`, reset on failure |
| db.go:6044 | CreateProjectAs | RCW | clear default then INSERT, no tx | LOCK settings row id=1 as sentinel, both in one tx (the default is no longer cleared before a validation that may refuse the project) |
| db.go:6197 | UpdateProjectAs | RMW | whole project row merged; default-project rule | LOCK projects row (plus sentinel when isDefault changes) |
| db.go:6426 | DeleteProject | RCW | checks is_default, reassigns and deletes in several statements | LOCK settings row then projects row; recheck and delete in one tx (a failed statement now aborts the deletion) |
| db.go:3630 | UpdateSettings | RMW | whole settings row merged | LOCK settings row id=1 |
| usersettings.go:121 | UpdateUserSettings | RMW | `current` read outside the lock, merged, upserted | SQL-1 upsert keeping each column the request leaves empty (`CASE WHEN ? = '' THEN user_settings.col ...`, ui_scale on 0) |
| boardviews.go:165 | UpdateBoardView | RMW | view read, merged, whole row written | LOCK board_views row, merge inside; a name clash maps to `ErrBoardViewNameTaken` |
| macros.go:250 | saveMacroMetaFull | RMW | macro row merged in Go (todos JSON) | LOCK macro row, created first if missing, merge inside |
| macros.go:477 | writeTaskParentLocally | RCW | macro title copied into tasks.parent_title | SQL-1 `parent_title = COALESCE(NULLIF((SELECT title FROM macros ...), ''), ?)` |
| macros.go:763 | MigrateMacro | RMW | source row read across GitHub calls, copied, source deleted | LOCK source row re-read after the milestone calls; copy and delete in one tx |
| bookmarks.go:149 | ToggleProjectBookmark | RCW | COUNT then DELETE or INSERT | COND: DELETE; if 0 rows, `INSERT ... ON CONFLICT DO NOTHING` |
| teams.go:91 | storeTeamMembers | RMW | DELETE then INSERTs, no tx | LOCK teams row; one tx |

## Accepted or covered

| file:line | function | class | outcome |
|---|---|---|---|
| db.go:948 | ImportOrUpdateTasks | RCW | accepted: PK and `UNIQUE(project_id, key)` turn a race into a reported error; syncs serialized per project by Q3 |
| db.go:4854 | EnqueueSyncWith | RCW | covered by #404 (claim) and Q3 (project worker); project-level row, outside Q4 |
| boardviews.go:131 | CreateBoardView | RCW | safe: unique index `idx_board_views_user_name`; map the violation to `ErrBoardViewNameTaken` |
| bookmarks.go:17 | EnsureDefaultBookmark | RCW | accepted: idempotent, worst case one extra default bookmark |
| usercredentials.go:174 | SetUserTrackerCredential | MEM | accepted: `unlocked` map is per process, #409 |
| usercredentials.go:207 | ClearUserTrackerCredential | MEM | accepted: #409 |
| orphancredentials.go:148 | DiscardOrphanedTrackerCredential | MEM | accepted: #409; delete already conditional |
| db.go:3967 | runJobGuarded | SINGLE | covered by #405 |
| db.go:4023 | processSkillJob (start) | SINGLE | covered by #405 |
| db.go:4087 | processSkillJob (final) | SINGLE | covered by #405 |
| db.go:4370 | processSyncJob | SINGLE | covered by #405 |
| db.go:4517 | processTrackerUpdateJob | SINGLE | covered by #405 |
| db.go:4625 | processTrackerUpdateJob | SINGLE | covered by #405 |
| db.go:5374 | CancelActivity | SINGLE | covered by #405 (`status IN (...)` guard) |

## Safe as is

READ: usercredentials.go:228, 265, 342; specframework.go:18; db.go:1332, 1586, 1841,
2554, 3430, 3437, 3448, 3934, 3994, 4037, 4511, 4655, 4746, 4887, 4920, 4950, 5088,
5223, 5310, 5428, 5452, 5921, 5953, 6542; usersettings.go:51; boardviews.go:63, 95;
prdiscovery.go:249 (rechecked inside :179); pins.go:120, 293; teams.go:242, 342,
384, 494, 602; orphancredentials.go:80; chain.go:32; macros.go:138, 441, 498, 721,
788, 970; projectskills.go:52, 125; sddslicing.go:558; comments.go:97;
bookmarks.go:63; stage.go:34 (fast pre-check; authoritative check inside :132).

SINGLE: specframework.go:178; db.go:902, 2100, 3139, 4063, 5399, 5407; boardviews.go:200;
remoterun.go:113, 331, 376; trackerops.go:108; agentlaunch.go:7; teams.go:63, 201,
238, 338, 380, 624, 704; instances.go:89, 159, 168, 182; chain.go:52; macros.go:111,
118, 231, 597, 714, 838, 945; projectskills.go:107, 286, 320; comments.go:93, 161;
bookmarks.go:110, 134.

## Process-local state outside `DB.mu`

| State | Decision |
|---|---|
| `ProjectLimiter` (`db.go:87`) | Q3: kept per process, backed by a per-project advisory lock on PostgreSQL |
| `cancelMap` | #405 |
| `autoSync.mu` | #404 |
| `unlocked` key map | #409 |
| `postBackListeners` | #405 |

## Network call under `DB.mu`

Only `CreateTaskAs` held it across `TrackerForProject` and `CreateIssue`. It now
reads what it needs, calls the tracker with no lock held, and writes the task in
a transaction of its own.
