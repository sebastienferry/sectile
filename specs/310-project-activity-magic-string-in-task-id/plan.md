# #310 — Implementation plan

Behaviour and acceptance criteria are in [`spec.md`](spec.md); this file holds the
technical choices only.

## Stack

- **Go store**, `internal/db/db.go` — schema, migrations, every read and write of
  `task_activities`.
- **Go store, dialect seam**, `internal/db/dialect.go` (+ its SQLite and PostgreSQL
  implementations) — the one place where the two engines are allowed to differ.
- **Go store, satellites** — `internal/db/remoterun.go` (a second
  `INSERT INTO task_activities`), `internal/db/autosync.go`, `internal/db/trackerops.go`.
- **Go models**, `internal/models/models.go` — `TaskActivity` already declares
  `ProjectID string \`json:"projectId,omitempty"\``; no new field is needed.
- **Go handlers**, `internal/handlers/handlers.go:2637` — `HandleActivities` and its
  filters.
- **Web**, `web/src/components/ActivitiesView.tsx`, `SyncView.tsx` — read-only
  consumers, expected untouched.

---

## Target schema

```sql
CREATE TABLE task_activities (
    id         TEXT PRIMARY KEY,
    task_id    TEXT     REFERENCES tasks(id),
    project_id TEXT     REFERENCES projects(id) ON DELETE CASCADE,
    ...                                  -- all other columns unchanged
    CHECK (task_id IS NULL OR project_id IS NULL)
);
CREATE INDEX IF NOT EXISTS idx_activities_task    ON task_activities(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activities_project ON task_activities(project_id, created_at DESC);
```

Three legitimate states: task activity (`task_id` set), project activity
(`project_id` set), global activity (both null). The `CHECK` is what stops the
implicit convention coming back through another door — it is the enforcement the
ticket is really asking for, not a nicety.

`idx_activities_task` is kept as is. The new index mirrors it so that the project
history is served the same way the task history is.

The long comment currently sitting inside the `CREATE TABLE` (the one explaining why
there is deliberately no foreign key) is replaced: it documents a decision this ticket
reverses. The replacement records the three states and why the `CHECK` exists.

### D1 — `task_id` stays a `string` in Go

`TaskActivity.TaskID` remains a non-pointer `string`. Every read does
`COALESCE(a.task_id, '')`, every write passes `NULLIF(?, '')` (or a
`sql.NullString` built from the empty test — see D5). A null column and an empty
string are therefore the same thing to the Go layer and to the JSON contract, which is
exactly what keeps `/api/activities` additive. Making the field a `*string` would ripple
through every call site and every test for no behavioural gain.

`TaskActivity.ProjectID` is treated identically: `COALESCE(a.project_id, '')` on read,
`NULLIF(?, '')` on write.

### D2 — The DDL difference goes through the dialect, not through scattered SQL

The schema is written once, in SQLite's spelling, and `dialect.RewriteDDL` adapts it —
that is the existing contract of the seam (ADR 0016). Two engine differences matter
here and neither may be handled by an `if driver == …` sprinkled through `db.go`:

- **Fresh databases.** Both engines create the table at the target schema directly
  from the `CREATE TABLE` above. Per ADR 0016, a new PostgreSQL database is born at
  the current schema and needs no DDL migration at all.
- **Existing databases.** SQLite can neither relax a `NOT NULL` nor add a foreign key
  nor add a `CHECK` through `ALTER TABLE`; it needs a table rebuild
  (`CREATE … _new` / `INSERT … SELECT` / `DROP` / `RENAME`, then recreate the
  indexes), in the shape `migrateTasksKeyUnique` already uses for `tasks`.
  PostgreSQL does it with `ALTER TABLE` statements.

This is a new kind of difference for the seam: today `RewriteDDL` only maps type
names. Rather than widen `RewriteDDL` into a statement generator, the plan adds a
narrow method to the `dialect` interface:

```go
// MigrateActivityAttachment brings an existing task_activities table to the
// schema where task_id is nullable and foreign-keyed, project_id exists, and a
// row may not carry both. It is a no-op on a table already at that schema.
MigrateActivityAttachment(conn *sqlConn) error
```

SQLite implements it as the rebuild; PostgreSQL as the `ALTER TABLE`s. The keys of
the interface stay small and named after *what* is needed, not after the engine.

### D3 — The DDL migration and the backfill are two separate things

This distinction is the one the clarification insists on, and getting it wrong is the
main risk in this ticket.

- The **DDL migration** repairs a table shape that only pre-#310 databases have. It
  runs on both engines, but it is shape-conditional (skipped when `project_id` already
  exists), not engine-conditional.
- The **backfill** rewrites `sync-*` rows. It must run on **both** engines and must
  **not** live inside `applyLegacyMigrations()`, which is gated behind
  `dialect.RunsLegacyMigrations()` and therefore never runs under PostgreSQL. A
  PostgreSQL database created after #304 contains `sync-*` rows: putting the backfill
  in the legacy block would leave them behind, `task_id` values that are not tasks,
  and the restored foreign key would then refuse to be created at all.

Both are invoked from `initSchema` after the `CREATE TABLE` loop and before the
default-workspace seed, in this order: DDL migration, then backfill, then (only once
the data is clean) the constraint-bearing final state. Under the SQLite rebuild the
natural sequencing is: backfill the old table first, then rebuild — a rebuild that
copies rows violating the new foreign key would fail on any engine that enforces it.
The implementation picks one order per engine and documents it at the call site.

### D4 — The backfill, in SQL

Two statements, both idempotent, both driven by the `sync-` prefix:

```sql
-- 1. A suffix that resolves to a project becomes a project activity.
UPDATE task_activities
   SET project_id = SUBSTR(task_id, 6), task_id = NULL
 WHERE task_id LIKE 'sync-%'
   AND SUBSTR(task_id, 6) IN (SELECT id FROM projects);

-- 2. Every remaining sync-* row becomes a global activity.
UPDATE task_activities
   SET task_id = NULL
 WHERE task_id LIKE 'sync-%';
```

Statement 2 catches `sync-all`, `sync-github`, `sync-jira` and any project that has
since been deleted, exactly as decided. No row is deleted. Re-running both is a no-op:
after the first pass no `task_id` starts with `sync-`.

`SUBSTR(x, 6)` is portable to both engines. If PostgreSQL's `SUBSTR` spelling proves
awkward next to SQLite's, the offset extraction moves behind the dialect rather than
being duplicated.

A belt-and-braces third statement nulls any `task_id` that matches no task at all
(rows that are neither `sync-*` nor real, if any exist) — otherwise the foreign key
cannot be created. It is written as a repair, and it logs how many rows it touched.

### D5 — Writes stop fabricating identifiers

`EnqueueSyncAs` (`db.go:4439`) is the only producer of a synthetic id. It loses
`targetTaskID` entirely:

- `proj != nil` → the activity is written with `ProjectID: proj.ID`, `TaskID: ""`.
- otherwise (`sync-all`, or a tracker-wide sync) → both empty.

`SkillJob.TaskID` stops relaying the synthetic value. The job already carries
`ProjectID` separately (line 4526 passes `projectID` alongside), so the consumer side
has what it needs; anything downstream that read `SkillJob.TaskID` expecting
`sync-<x>` is corrected to read `ProjectID`. This is worth a grep rather than an
assumption, and is an explicit step in `tasks.md`.

`addTaskActivityDirect` (`db.go:3080`) and the second insert in
`remoterun.go:263` both gain `project_id` in their column list and write both
attachment columns through `NULLIF(?, '')`.

### D6 — Reads

| Site | Change |
| --- | --- |
| `getTaskActivitiesUnsafe` (`db.go:3116`) | Unchanged filter `a.task_id = ?`. It is the *task* history and stays exactly that. Its callers that passed `"sync-" + project.ID` move to the new project reader. |
| **new** `getProjectActivitiesUnsafe` / `GetProjectActivities` | Dedicated reader, filter `a.project_id = ?`. Decision 4 of the clarification: a separate method rather than teaching `GetTaskActivities` to accept a project id, which would recreate the same ambiguity in Go that the ticket is removing from SQL. |
| `GetActivities` (`db.go:4747`) | Project predicate becomes `(a.project_id = ? OR t.project_id = ? OR t.project_id = (SELECT slug …) OR t.project_id = (SELECT id FROM projects WHERE slug = ?) OR a.project_id = (SELECT id FROM projects WHERE slug = ?))` — the existing slug/id resolution preserved, both `LIKE` clauses removed. The `SELECT` list gains `COALESCE(a.project_id, '')` and wraps `a.task_id` in `COALESCE`. |
| `GetActivityStats` (`db.go:4967`) | Same predicate, kept textually identical to the one in `GetActivities` so the two cannot drift; extracting it into a shared helper that returns `(clause, args)` is preferred over copying it. |
| `GetActivityByID` (`db.go:4881`) | Already `LEFT JOIN tasks` with `COALESCE`; add the `project_id` column and, if the view needs it, a `LEFT JOIN projects`. |
| `remoterun.go:168,384,394` | `id = ? AND task_id = ?` — always real tasks, unchanged. |
| `DeleteTask` (`db.go:2915,2921`) | Unchanged. The explicit delete stays; the restored foreign key carries no `ON DELETE` for tasks, as noted in the current schema comment. |

All queries keep `?` placeholders and go through `Rebind` (ADR 0016).

### D7 — Handlers and API

`HandleActivities` (`handlers.go:2637`) passes its `projectId` filter through
unchanged; the semantics change underneath it, not its signature. The serialised
`taskId` stays a string, empty when null. `projectId` is already declared on the model
with `omitempty` and simply starts being populated. Nothing is removed from the
payload — the contract is additive, per US6.

### D8 — Tests that encode the convention are rewritten, not preserved

`postgres_sync_test.go`, `jiratracker_test.go` (lines 207, 259, 343, 395, 423, 453)
and `headless_test.go:146` write `TaskID: "sync-" + project.ID`. They are rewritten to
`ProjectID: project.ID`. Keeping them as they are would mean keeping the convention
alive in the one place that is supposed to prove it is gone.

---

## Rejected alternatives

- **Teach `GetTaskActivities` to accept a project identifier.** Rejected: it moves the
  overloading from the column into the function signature.
- **Denormalise `project_id` onto task activities.** Rejected in Round 2: two sources
  of truth for one attachment, and the join on `tasks` already exists.
- **Delete unresolvable `sync-*` rows.** Rejected in Round 2: they are history, and
  they read perfectly well as global activities.
- **Gate the backfill behind `RunsLegacyMigrations()`.** Rejected: PostgreSQL
  databases created since #304 hold the very rows that need it. See D3.
- **A pointer `*string` for `TaskID`.** Rejected: see D1.

---

## Risks

1. **Creating the foreign key before the data is clean** fails the migration outright
   under PostgreSQL. Ordering (D3) is the mitigation, and the smoke path must cover a
   database that still holds `sync-*` rows at startup.
2. **A `SkillJob.TaskID` consumer** still expecting `sync-<x>`. Mitigated by the
   explicit grep step in `tasks.md`, not by assumption.
3. **The two project predicates drifting apart** between `GetActivities` and
   `GetActivityStats`. Mitigated by the shared helper (D6).
4. **The SQLite rebuild dropping a column or an index** added since the table was
   written. Mitigated by the existing schema-comparison test that opens SQLite through
   a dialect skipping the legacy migrations and compares the two schemas (`openWith`
   exists for exactly this).
