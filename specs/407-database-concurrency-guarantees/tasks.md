# #407: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md), [`inventory.md`](inventory.md).

Before starting: `git fetch origin`, rebase `feat/407` onto `origin/main`, check the
next free migration number and that no ticket of #397 landed the same conversions.

## 1. Seam and helpers

- [ ] T1.1 `dialect.ForUpdate()` (SQLite `""`, PostgreSQL `" FOR UPDATE"`) and `d.forUpdate()`.
- [ ] T1.2 `d.withTx(fn)` in `conn.go`: commit, rollback on error or panic.
- [ ] T1.3 `IsUniqueViolation(err, index)` for pgx (`23505` + constraint name) and modernc SQLite (message), never matching a primary-key collision.
- [ ] T1.4 `dialect.AcquireProjectWorker(conn, projectID)`: SQLite no-op; PostgreSQL `pg_try_advisory_lock(0x5EC7, fnv32a(projectID))` on a reserved connection, released between retries, `projectWorkerRetry` package variable (FR8).

## 2. One ordinary run per task (US3, FR4-FR7)

- [ ] T2.1 Migration 9 `one_active_run`: `concurrent` column, surplus active runs marked `concurrent = 1` (keeper order in plan), partial unique index `idx_activities_one_active_run`.
- [ ] T2.2 `models.TaskActivity.Concurrent`; `insertTaskActivity` writes it.
- [ ] T2.3 `activeRunSkillIDs` / `activeRunStatuses` constants; `ActiveRunOnTask` counts queued, pending, running, all `concurrent` values, drops the legacy action clause, reports a running run before a queued one.
- [ ] T2.4 `ErrTaskBusy` / `TaskBusyError`.
- [ ] T2.5 `enqueueSkillOnTask`: stop ignoring the insert error; violation returns `*TaskBusyError` before `enqueueJob` (covers full chain, overrides, retry, chain step).
- [ ] T2.6 `startRemoteRun`: `concurrent = 1` without a launcher run id; `StartAgentRun` takes the flag.
- [ ] T2.7 `processSkillJob`: check the `agent_launch` rename error; on violation from `StartAgentRun`, fail the launch row with a readable reason.
- [ ] T2.8 Handler run-skill: `concurrent = 1` with `force`; insert the `remote_run` before the `agent_launch` row; violation answers the 409 body.
- [ ] T2.9 Handlers full chain, enqueue with overrides and retry map `ErrTaskBusy` to 409 `{error, activeRunId}`.
- [ ] T2.10 `describeActiveRun`: queued variant.
- [ ] T2.11 `SyncRemoteRunStatusFor`: COND on non-terminal status (FR9); insert branch `ON CONFLICT (id) DO NOTHING`, `concurrent = 1`.

## 3. Project worker across replicas (US4, FR8)

- [ ] T3.1 `startQueueWorker`: after `d.limiter.Acquire`, `AcquireProjectWorker`; log and continue on error; release after `runJobGuarded`. `tracker_op` unchanged.

## 4. Task row conversions (US1, US2, FR3)

- [ ] T4.1 `TransitionTaskStageBy`: lock the task row in its transaction; running-stage guard, labels, links and branch from the locked row.
- [ ] T4.2 `updateTaskBy`, `MoveTask`, `MoveTaskToTrackerStatus`, `PostBackTask`: LOCK, jobs pushed after commit.
- [ ] T4.3 `applyDiscoveredPullRequests`, `applyPullRequestStates`, `adjustmentPrerequisite`: LOCK, `pr_links` re-read inside; no forge call inside the transaction.
- [ ] T4.4 `SetTaskPinnedBy`, `ToggleTaskPinnedBy`: LOCK, flip decided inside.

## 5. Activity and run conversions (US1)

- [ ] T5.1 `AppendRemoteRunOutput`: single `CASE` statement, character limit (FR12).
- [ ] T5.2 `finishRemoteRun`: summary joined in SQL.
- [ ] T5.3 `appendActivityStep`, `finishTrackerOp`: LOCK the activity row.

## 6. Projects, settings and the rest (US1, US5)

- [ ] T6.1 `CreateTaskAs`: `CreateIssue` before any lock; key and position under a project row lock (FR11).
- [ ] T6.2 `ConvertTaskToRemote`: `converting` claim, final conditional update, reset on failure; readers treat `converting` as local (FR10).
- [ ] T6.3 `CreateProjectAs`, `UpdateProjectAs`, `DeleteProject`: settings sentinel lock and one transaction; `UpdateSettings`: LOCK settings row.
- [ ] T6.4 `UpdateUserSettings`, `UpdateBoardView`, `saveMacroMetaFull`, `writeTaskParentLocally`, `MigrateMacro`: single statements per inventory.
- [ ] T6.5 `ToggleProjectBookmark`: COND; `storeTeamMembers`: LOCK plus one transaction; `CreateBoardView`: map the unique violation to `ErrBoardViewNameTaken`.

## 7. Tests

PostgreSQL, `postgres_concurrency_test.go`, `openPostgresPair`, skipped without DSN:

- [ ] T7.1 Run output: 2 handles × 20 goroutines append distinct chunks; all present; with an overflowing total, one marker and a length of limit plus marker.
- [ ] T7.2 Stage transitions: concurrent transitions carrying different PR URLs from both handles; every link present, labels consistent with the last applied stage.
- [ ] T7.3 Running-stage rule: a managed run active on the stage; concurrent transitions on both handles are all refused.
- [ ] T7.4 PR links: `applyDiscoveredPullRequests` on one handle racing a transition on the other; no link lost.
- [ ] T7.5 One ordinary run: 2 handles × 10 `enqueueSkillOnTask` on one task; exactly one succeeds, the rest `ErrTaskBusy`; `concurrent` inserts all succeed.
- [ ] T7.6 Project worker: `AcquireProjectWorker` held on handle A; B's attempt waits until A releases; two different projects do not wait on each other.
- [ ] T7.7 Terminal runs: a run finished on A is not revived by a `running` report on B.
- [ ] T7.8 Convert: concurrent `ConvertTaskToRemote` on both handles with a fake tracker; one `CreateIssue`, one refusal.
- [ ] T7.9 Local tasks: concurrent `CreateTaskAs` on both handles; distinct keys, no error.
- [ ] T7.10 Default project: concurrent `CreateProjectAs(isDefault)`; exactly one default. Concurrent `UpdateProjectAs` of different fields; both kept.

Default engine (SQLite), no DSN needed:

- [ ] T7.11 Migration 9 on a database with two active ordinary runs on a task: upgrade succeeds, the keeper stays ordinary (`remote_run` preferred), the other becomes `concurrent`.
- [ ] T7.12 Index: a second ordinary active run is refused; queued counts; `agent_launch`, `tracker_op`, terminal rows and `concurrent` rows do not; NULL `task_id` never collides; `IsUniqueViolation` does not match a PK collision.
- [ ] T7.13 Catalog guard: every `skills.StageSkills` id is in `activeRunSkillIDs`.
- [ ] T7.14 Handlers: busy run-skill, full chain, overrides and retry answer 409 with `{error, activeRunId}` and record nothing; the queued wording; `force` by the owner starts a second run; MCP `start_run` without run id is never refused.
- [ ] T7.15 `AppendRemoteRunOutput` cuts multibyte output on a character boundary.
- [ ] T7.16 Rewrite existing tests that insert two active ordinary runs on one task (for example `agentlaunch_test.go`) to mark the extra one `concurrent`.
- [ ] T7.17 Existing suites green, including the restart tests and #405's guards.

## 8. Documentation

- [ ] T8.1 `docs/db-concurrency-audit.md` from `inventory.md`, final line numbers and outcomes (FR2).
- [ ] T8.2 `CHANGELOG.md` `[Unreleased]`:
  - `Changed`: a task with a queued run is now busy, and a second launch is refused until that run starts, ends or is canceled.
  - `Fixed`: long run output with non-ASCII characters is cut cleanly instead of failing to save on PostgreSQL (only if T7.15 reproduces the failure before the fix).
- [ ] T8.3 README or `docs/` sections describing the busy rule or "Launch anyway", if any, mention that queued runs count.

## Test plan

`go build ./...`, `go vet ./...`, `go test ./internal/db/... ./internal/handlers/... ./internal/taskmcp/...`,
then the full `go test ./...`; the PostgreSQL subset with a throwaway
`SECTILE_TEST_POSTGRES_DSN` (never the dev database: the suite truncates it), and in CI
through `test:postgres`. Run the race subset with `-race -count=3`.
