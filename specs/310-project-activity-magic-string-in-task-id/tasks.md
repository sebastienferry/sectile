# #310 — Implementation checklist

Ordered so that each layer is verified before the next is built on it: schema and
dialect first, migration and backfill second, writes third, reads fourth, handlers and
tests last. The order inside section 2 is load-bearing — the foreign key cannot be
created while `sync-*` rows are still in the table.

---

## 1. Schema and dialect (`internal/db/db.go`, `internal/db/dialect.go`)

- [x] **T1** In the `CREATE TABLE task_activities` statement (`db.go:454`):
  - Make `task_id` nullable and add `REFERENCES tasks(id)`.
  - Add `project_id TEXT REFERENCES projects(id) ON DELETE CASCADE`.
  - Add `CHECK (task_id IS NULL OR project_id IS NULL)`.
  - Replace the comment explaining the deliberate absence of a foreign key: it
    documents the decision this ticket reverses. Record instead the three legitimate
    states (task, project, global) and why the `CHECK` is there.
- [x] **T2** Add `CREATE INDEX IF NOT EXISTS idx_activities_project ON task_activities(project_id, created_at DESC);` beside the existing activity indexes. Leave `idx_activities_task` untouched.
- [x] **T3** In `internal/db/dialect.go`, add `MigrateActivityAttachment(conn *sqlConn) error` to the `dialect` interface, with a doc comment saying it is a no-op on a table already at the target schema.
- [x] **T4** Implement it for SQLite as a table rebuild (`CREATE task_activities_new` at the target schema / `INSERT … SELECT` / `DROP` / `RENAME`, then recreate the three indexes), following the shape of `migrateTasksKeyUnique`. Detect "already migrated" by the presence of the `project_id` column via `ColumnsQuery()`.
- [x] **T5** Implement it for PostgreSQL as `ALTER TABLE` statements: drop the `NOT NULL` on `task_id`, add `project_id`, add both foreign keys, add the `CHECK`. Same "already migrated" detection.
- [x] **T6** Verify with the existing schema-comparison test (`openWith` with a dialect that skips the legacy migrations) that a freshly created SQLite database and a migrated one end at the same schema. Fix any column or index the rebuild drops.

## 2. Migration and backfill (`internal/db/db.go`)

- [x] **T7** Write `backfillActivityAttachment()`, engine-independent, running the three statements of plan D4 in order: resolve `sync-<projectID>` onto `project_id`, null every remaining `sync-%` `task_id`, then null any `task_id` matching no row of `tasks`. Log the number of rows each statement touched. Every statement is idempotent.
- [x] **T8** In `initSchema`, after the `CREATE TABLE` loop and **before** the default-workspace seed, call the backfill and `dialect.MigrateActivityAttachment` in an order that never leaves `sync-*` rows in a table that already carries the foreign key. Under SQLite the backfill runs on the old table first, then the rebuild copies clean rows. Document the chosen order in a comment at the call site.
- [x] **T9** Confirm neither call sits inside `applyLegacyMigrations()`, which is gated behind `dialect.RunsLegacyMigrations()` and never runs under PostgreSQL. This is the single most important constraint of the migration (plan D3).
- [x] **T10** Test, on both engines: a database seeded with `sync-<existing project>`, `sync-all`, `sync-github` and `sync-<deleted project>` rows, plus a normal task activity. After startup, assert the first became a project activity, the other three became global activities, the task activity is untouched, no row was deleted, and a second startup changes nothing.
- [x] **T11** Test that the foreign key and the `CHECK` are actually enforced after migration: inserting an activity with an unknown `task_id` is rejected, and inserting one carrying both `task_id` and `project_id` is rejected.

## 3. Writes (`internal/db/db.go`, `internal/db/remoterun.go`)

- [x] **T12** In `addTaskActivityDirect` (`db.go:3080`), add `project_id` to the column list and write both attachment columns through `NULLIF(?, '')`.
- [x] **T13** Same for the second `INSERT INTO task_activities` in `remoterun.go:263`.
  Left unchanged on purpose: that insert only ever writes a real task
  (`realTaskID`), so a `project_id` column there could only ever be `NULL`. The
  denormalisation it would have carried is removed instead — see T12.
- [x] **T14** In `EnqueueSyncAs` (`db.go:4439`), delete `targetTaskID` (lines 4462-4465). Write `ProjectID: proj.ID` with an empty `TaskID` when a project is targeted; leave both empty for a global or tracker-wide sync.
- [x] **T15** In the `enqueueJob(SkillJob{…})` call (`db.go:4526`), stop passing the synthetic value as `TaskID`. `ProjectID` is already carried separately.
- [x] **T16** Grep for every consumer of `SkillJob.TaskID` and for any remaining `"sync-"` string construction across the repository. Correct each consumer that expected the synthetic form to read `ProjectID` instead. Do not assume there are none.
- [x] **T17** Verify that the other `TaskID:` literals in `db.go` (lines 2686, 2707, 4138, 4157, 4378, 4401, 4637, 4666, 5177) are all `task.ID` and need no change.

## 4. Reads (`internal/db/db.go`)

- [x] **T18** Add `GetProjectActivities` / `getProjectActivitiesUnsafe`, filtering on `a.project_id = ?`, mirroring `getTaskActivitiesUnsafe`'s shape and ordering.
- [x] **T19** Leave `getTaskActivitiesUnsafe` (`db.go:3116`) and its `a.task_id = ?` filter unchanged. Move its callers that passed `"sync-" + project.ID` to `GetProjectActivities`.
- [x] **T20** Extract the project predicate into a shared helper returning `(clause, args)`, so `GetActivities` and `GetActivityStats` cannot drift.
- [x] **T21** In `GetActivities` (`db.go:4747`), replace the project condition with that helper: `a.project_id = ? OR t.project_id = ?` plus the existing slug/id resolution, with both `a.task_id LIKE ?` and `a.prompt LIKE ?` removed. Add `COALESCE(a.project_id, '')` to the `SELECT` list and wrap `a.task_id` in `COALESCE`, scanning into `ProjectID` and `TaskID`.
- [x] **T22** In `GetActivityStats` (`db.go:4967`), use the same helper.
- [x] **T23** In `GetActivityByID` (`db.go:4881`), select `project_id` and populate `ProjectID`.
- [x] **T24** Confirm `remoterun.go:168,384,394` and `DeleteTask` (`db.go:2915,2921`) need no change.
- [x] **T25** Test the project filter: an activity attached to `P`, an activity on a task of `P`, and an activity whose `prompt` merely contains another project's id. Filtering on `P` returns the first two and not the third; filtering by slug behaves as filtering by id; the statistics count exactly the set the list returns.

## 5. Handlers and API (`internal/handlers/handlers.go`)

- [x] **T26** Check `HandleActivities` (`handlers.go:2637`) needs no signature change and that its `projectId` filter now flows into the new predicate.
- [x] **T27** Test the serialised payload: a null `task_id` renders as `""` and not `null`, `projectId` carries the project id on a project activity and is absent on a task activity, and no field present before is missing.

## 6. Tests carrying the old convention

- [x] **T28** Rewrite `TaskID: "sync-" + project.ID` to `ProjectID: project.ID` in `postgres_sync_test.go`.
- [x] **T29** Same in `jiratracker_test.go` (lines 207, 259, 343, 395, 423, 453).
- [x] **T30** Same in `headless_test.go:146`.
- [x] **T31** Add the end-to-end acceptance test, run on both engines: launch a synchronisation for a project, assert the activity is stored with `project_id` set and `task_id` null, and assert it appears in that project's history. This is the ticket's own acceptance criterion.

## 7. Interface and documentation

- [x] **T32** Verify `web/src/components/ActivitiesView.tsx` and `SyncView.tsx` still render project activities with no change. If a display adjustment turns out to be necessary, keep it minimal and note it in the pull request.
- [x] **T33** If `docs/adrs/0016-postgresql-as-an-alternative-store.md` needs a note on the new dialect method, add it. Do not open a new ADR: this ticket reverses a tactical decision of #304, it does not take an architectural one.
- [x] **T34** Update `CHANGELOG.md` if the repository keeps one at the time of implementation.

## 8. Gates

- [x] **T35** `make build` (or the repository's equivalent) passes.
- [x] **T36** The Go test suite passes on SQLite.
- [x] **T37** The PostgreSQL suite passes, including the migration and backfill tests of T10 and T11.
- [x] **T38** Lint and formatting gates pass.

---

## Out of scope

- Autosync running without an `ActingUser` — tracked as **#312**, must not be touched
  here.
- Any rework of the activity history beyond this attachment split.


---

## Implementation notes

Three deviations from the plan, all documented at their call sites:

- **T8 — the backfill is passed to the dialect, not called beside it.** The two
  engines need it at different points: SQLite rebuilds the table and then cleans
  it, PostgreSQL has to clean between relaxing `task_id` and creating the foreign
  key, which no engine accepts over dirty data. `MigrateActivityAttachment` takes
  the backfill as an argument; the backfill itself stays engine-independent, in
  `activityattachment.go`, and runs on both engines outside the legacy block.
- **`idx_activities_project` is created after the migration**, not with the other
  indexes: on a database that still holds the old table the column does not exist
  yet, and the statement would fail the whole schema initialisation.
- **Two more magic strings were found by T16** and removed with the first:
  `spec-framework-<project>` (`specframework.go`) and `tracker-op-<project>`
  (`trackerops.go`). Both are project-level work that was filed under a made-up
  `task_id`; both are backfilled onto `project_id` like `sync-<project>`.

Two behaviours the spec implies and the engines do not give for free:

- `DeleteProject` now deletes the project's activities explicitly, as
  `DeleteTask` already did. `ON DELETE CASCADE` only fires under PostgreSQL —
  this package never turns SQLite's foreign keys on.
- `StartAgentRun` no longer writes its task's `project_id` onto the activity
  (decision 5, no denormalisation). The `CHECK` refuses it outright.
