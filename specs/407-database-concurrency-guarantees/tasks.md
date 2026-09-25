# #407: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md), [`inventory.md`](inventory.md).

Before starting: `git fetch origin`, rebase `feat/407` onto `origin/main`, check the
next free migration number and that no ticket of #397 landed the same conversions.

## 1. Seam and helpers

- [x] T1.1 `dialect.ForUpdate()` (SQLite `""`, PostgreSQL `" FOR UPDATE"`) and `d.forUpdate()`.
- [x] T1.2 `d.withTx(fn)` in `conn.go`: commit, rollback on error or panic.
- [x] T1.3 `IsUniqueViolation(err, index)` for pgx (`23505` + constraint name) and modernc SQLite (message), never matching a primary-key collision.
- [x] T1.4 `dialect.AcquireProjectWorker(conn, projectID)`: SQLite no-op; PostgreSQL `pg_try_advisory_lock(0x5EC7, fnv32a(projectID))` on a reserved connection, released between retries, `projectWorkerRetry` package variable (FR8).

## 2. One ordinary run per task (US3, FR4-FR7)

- [x] T2.1 Migration 9 `one_active_run`: `concurrent` column, surplus active runs marked `concurrent = 1` (keeper order in plan), partial unique index `idx_activities_one_active_run`.
- [x] T2.2 `models.TaskActivity.Concurrent`; `insertTaskActivity` writes it.
- [x] T2.3 `activeRunSkillIDs` / `activeRunStatuses` constants; `ActiveRunOnTask` counts queued, pending, running, all `concurrent` values, drops the legacy action clause, reports a running run before a queued one.
- [x] T2.4 `ErrTaskBusy` / `TaskBusyError`.
- [x] T2.5 `enqueueSkillOnTask`: stop ignoring the insert error; violation returns `*TaskBusyError` before `enqueueJob` (covers full chain, overrides, retry, chain step).
- [x] T2.6 `startRemoteRun`: `concurrent = 1` without a launcher run id; `StartAgentRun` takes the flag.
- [x] T2.7 `processSkillJob`: check the `agent_launch` rename error; on violation from `StartAgentRun`, fail the launch row with a readable reason.
- [x] T2.8 Handler run-skill: `concurrent = 1` with `force`; insert the `remote_run` before the `agent_launch` row; violation answers the 409 body.
- [x] T2.9 Handlers full chain, enqueue with overrides and retry map `ErrTaskBusy` to 409 `{error, activeRunId}`.
- [x] T2.10 `describeActiveRun`: queued variant.
- [x] T2.11 `SyncRemoteRunStatusFor`: COND on non-terminal status (FR9); insert branch `ON CONFLICT (id) DO NOTHING`, `concurrent = 1`.

## 3. Project worker across replicas (US4, FR8)

- [x] T3.1 `startQueueWorker`: after `d.limiter.Acquire`, `AcquireProjectWorker`; log and continue on error; release after `runJobGuarded`. `tracker_op` unchanged.

## 4. Task row conversions (US1, US2, FR3)

- [x] T4.1 `TransitionTaskStageBy`: lock the task row in its transaction; running-stage guard, labels, links and branch from the locked row.
- [x] T4.2 `updateTaskBy`, `MoveTask`, `MoveTaskToTrackerStatus`, `PostBackTask`: LOCK, jobs pushed after commit.
- [x] T4.3 `applyDiscoveredPullRequests`, `applyPullRequestStates`, `adjustmentPrerequisite`: LOCK, `pr_links` re-read inside; no forge call inside the transaction.
- [x] T4.4 `SetTaskPinnedBy`, `ToggleTaskPinnedBy`: LOCK, flip decided inside.

## 5. Activity and run conversions (US1)

- [x] T5.1 `AppendRemoteRunOutput`: single `CASE` statement, character limit (FR12).
- [x] T5.2 `finishRemoteRun`: summary joined in SQL.
- [x] T5.3 `appendActivityStep`, `finishTrackerOp`: LOCK the activity row.

## 6. Projects, settings and the rest (US1, US5)

- [x] T6.1 `CreateTaskAs`: `CreateIssue` before any lock; key and position under a project row lock (FR11).
- [x] T6.2 `ConvertTaskToRemote`: `converting` claim, final conditional update, reset on failure; readers treat `converting` as local (FR10).
- [x] T6.3 `CreateProjectAs`, `UpdateProjectAs`, `DeleteProject`: settings sentinel lock and one transaction; `UpdateSettings`: LOCK settings row.
- [x] T6.4 `UpdateUserSettings`, `UpdateBoardView`, `saveMacroMetaFull`, `writeTaskParentLocally`, `MigrateMacro`: single statements per inventory.
- [x] T6.5 `ToggleProjectBookmark`: COND; `storeTeamMembers`: LOCK plus one transaction; `CreateBoardView`: map the unique violation to `ErrBoardViewNameTaken`.

## 7. Tests

PostgreSQL, `postgres_concurrency_test.go`, `openPostgresPair`, skipped without DSN:

- [x] T7.1 Run output: 2 handles × 20 goroutines append distinct chunks; all present; with an overflowing total, one marker and a length of limit plus marker.
- [x] T7.2 Stage transitions: concurrent transitions carrying different PR URLs from both handles; every link present, labels consistent with the last applied stage.
- [x] T7.3 Running-stage rule: a managed run active on the stage; concurrent transitions on both handles are all refused.
- [x] T7.4 PR links: `applyDiscoveredPullRequests` on one handle racing a transition on the other; no link lost.
- [x] T7.5 One ordinary run: 2 handles × 10 `enqueueSkillOnTask` on one task; exactly one succeeds, the rest `ErrTaskBusy`; `concurrent` inserts all succeed.
- [x] T7.6 Project worker: `AcquireProjectWorker` held on handle A; B's attempt waits until A releases; two different projects do not wait on each other.
- [x] T7.7 Terminal runs: a run finished on A is not revived by a `running` report on B.
- [x] T7.8 Convert: concurrent `ConvertTaskToRemote` on both handles with a fake tracker; one `CreateIssue`, one refusal.
- [x] T7.9 Local tasks: concurrent `CreateTaskAs` on both handles; distinct keys, no error.
- [x] T7.10 Default project: concurrent `CreateProjectAs(isDefault)`; exactly one default. Concurrent `UpdateProjectAs` of different fields; both kept.

Default engine (SQLite), no DSN needed:

- [x] T7.11 Migration 9 on a database with two active ordinary runs on a task: upgrade succeeds, the keeper stays ordinary (`remote_run` preferred), the other becomes `concurrent`.
- [x] T7.12 Index: a second ordinary active run is refused; queued counts; `agent_launch`, `tracker_op`, terminal rows and `concurrent` rows do not; NULL `task_id` never collides; `IsUniqueViolation` does not match a PK collision.
- [x] T7.13 Catalog guard: every `skills.StageSkills` id is in `activeRunSkillIDs`.
- [x] T7.14 Handlers: busy run-skill, full chain, overrides and retry answer 409 with `{error, activeRunId}` and record nothing; the queued wording; `force` by the owner starts a second run; MCP `start_run` without run id is never refused.
- [x] T7.15 `AppendRemoteRunOutput` cuts multibyte output on a character boundary.
- [x] T7.18 Both acceptance races (T7.1, T7.2) fail on `main` before the change: chunks and 11 of 12 links lost, three runs out of three.
- [x] T7.16 Rewrite existing tests that insert two active ordinary runs on one task (for example `agentlaunch_test.go`) to mark the extra one `concurrent`.
- [x] T7.17 Existing suites green, including the restart tests and #405's guards.

## 8. Documentation

- [x] T8.1 `docs/db-concurrency-audit.md` from `inventory.md`, final line numbers and outcomes (FR2).
- [x] T8.2 `CHANGELOG.md` `[Unreleased]`:
  - `Changed`: a task with a queued run is now busy, and a second launch is refused until that run starts, ends or is canceled.
  - `Fixed`: a run that ended is no longer revived by a late report from the agent (FR9). The multibyte output line was left out: the failure was not reproduced before the fix.
- [x] T8.3 README or `docs/` sections describing the busy rule or "Launch anyway", if any, mention that queued runs count.

## Deviations from the plan

- `UpdateBoardView`, `saveMacroMetaFull` and `MigrateMacro` use a row lock rather
  than a single statement: same guarantee, far less SQL rewritten.
- `applyDiscoveredPullRequests` does not re-read `pr_links_detached` inside its
  transaction; the flag is read before discovery, as on one process. Noted in the
  audit.
- `ToggleTaskPinnedBy` now writes the tracker label as the acting user, as its
  comment always said; it used to go through the actor-less setter.
- `TestSyncRemoteRunStatus` asserted that a failed run is revived by a later
  "running" report. FR9 reverses that, so the test now asserts the opposite.
- `CreateProjectAs` used to clear the previous default before a validation that
  could refuse the project; the clear now happens in the insert's transaction.

## Adjustments after review

- The conversion claim survived no concurrent edit: full-row task writers wrote
  `source` back as `local`. They now keep `converting`; the claim expires after
  five minutes; the final write locks the task and fails loudly if the claim was
  lost (`TestAnEditDuringAConversionKeepsBoth`, `TestAStaleConversionClaimIsTakenOver`).
- FR5 on the queued path: `enqueueSkillOnTask` checks `ActiveRunOnTask` before
  its insert, so a concurrent run makes the task busy there too
  (`TestEnqueueNextToAConcurrentRunIsRefused`).
- "Launch anyway" with nothing active records an ordinary run.
- `UpdateProjectAs` locks the row when called with a slug as well.
- At most eight worker-lock connections are reserved per process.

## Test plan

`go build ./...`, `go vet ./...`, `go test ./internal/db/... ./internal/handlers/... ./internal/taskmcp/...`,
then the full `go test ./...`; the PostgreSQL subset with a throwaway
`SECTILE_TEST_POSTGRES_DSN` (never the dev database: the suite truncates it), and in CI
through `test:postgres`. Run the race subset with `-race -count=3`.
