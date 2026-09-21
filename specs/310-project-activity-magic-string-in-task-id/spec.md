# #310 — A project activity is a magic string in `task_id`

Ticket: https://github.com/sebastienferry/sectile/issues/310
Type: Technical debt — follow-up of PR #304.
Branch: `feat/310`.
Clarification: [`docs/clarifications/310.md`](../../docs/clarifications/310.md).

## Context

`task_activities.task_id` currently carries two unrelated meanings. For an activity
that belongs to a ticket it is a foreign key into `tasks`. For an activity that
belongs to a project — or to nobody — it is a synthetic identifier built by string
concatenation: `sync-<projectID>` when a project is targeted, `sync-<syncType>`
(`sync-all`, `sync-github`, `sync-jira`) otherwise.

That convention is implicit and unenforced. It cost the project a real incident:
PR #304 had to drop the foreign key on `task_id` altogether, because PostgreSQL
enforced it and rejected every synthetic row, which made a synchronisation run and
leave no trace at all. SQLite never enforced it, so the same rows had always gone in
silently. A second implicit convention grew on top: the project filter of
`GetActivities` and `GetActivityStats` recovers project activities with
`a.task_id LIKE '%projectID%' OR a.prompt LIKE '%projectID%'`, a substring match on
two free-text columns.

This ticket separates the two meanings. `task_id` becomes nullable and regains its
foreign key to `tasks`, a `project_id` column carries project activities, existing
`sync-*` rows are migrated, and the database itself forbids a row from holding both.

Out of scope: any rework of the activity history beyond this split, and any change to
the `/api/activities` contract other than the additive one the new column requires.
The autosync-without-`ActingUser` question is explicitly out of scope and tracked as
its own ticket, **#312**.

This file states behaviour and acceptance criteria only. Implementation choices are in
[`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

All six open questions of the clarification were answered by the ticket owner in
Round 2, each in line with the recommendation. They are restated here as the
decisions this specification implements.

1. **Three legitimate attachments, never two at once.** An activity is attached to a
   task, or to a project, or to nothing (a global activity). The database rejects a
   row that carries both.
2. **`task_id` regains its foreign key** to `tasks(id)` and becomes nullable.
3. **`project_id` is a real column** with a foreign key to `projects(id)`,
   `ON DELETE CASCADE`.
4. **The substring heuristics disappear.** Both `a.task_id LIKE '%projectID%'` and
   `a.prompt LIKE '%projectID%'` are removed from the project filter, which becomes
   `a.project_id = ? OR t.project_id = ?`, keeping the existing slug/id resolution.
5. **No denormalisation.** A task activity does not also carry its task's
   `project_id`; that attachment stays carried by the join on `tasks`.
6. **No row is deleted by the migration.** A `sync-<x>` row whose suffix resolves to
   an existing project becomes a project activity; one whose suffix resolves to no
   project becomes a global activity.
7. **The JSON contract of `/api/activities` does not break.** A null `task_id` is
   rendered as an empty string; `projectId` is populated from the new column.

---

## User stories

### US1 — A synchronisation appears in its project's history, on both engines (P1)

**As** a user watching a project's activity history,
**I want** a synchronisation launched for that project to appear in it,
**So that** I can see that the sync ran, and what it did, whichever engine the server
stores its data in.

- **Given** a server backed by SQLite and a project `P`
- **When** a synchronisation is launched for `P`
- **Then** the activity is stored with `project_id = P.id` and `task_id` null,
  and it appears in the activity history filtered on `P`.

- **Given** a server backed by PostgreSQL and a project `P`
- **When** a synchronisation is launched for `P`
- **Then** the insert is accepted (no foreign key violation), the activity is stored
  with `project_id = P.id` and `task_id` null, and it appears in the activity history
  filtered on `P`.

- **Given** a synchronisation launched for all projects (`sync-all`) or for a tracker
  rather than a project (`sync_github`, `sync_jira`)
- **When** the activity is recorded
- **Then** both `task_id` and `project_id` are null, and the activity appears in the
  "all projects" history and in no single-project history.

### US2 — No activity row carries a `task_id` that is not a task (P1)

**As** a maintainer of the store,
**I want** `task_id` to mean exactly one thing,
**So that** the column can be trusted, joined and constrained without a decoder ring.

- **Given** a database at the target schema
- **When** any row of `task_activities` is read
- **Then** its `task_id` is either null or the id of an existing row of `tasks`.

- **Given** any code path that records an activity
- **When** that activity belongs to a project rather than to a task
- **Then** it is written through `project_id`, and no identifier is built by string
  concatenation anywhere in the store.

- **Given** an attempt to insert a row carrying both a `task_id` and a `project_id`
- **When** the insert reaches the database
- **Then** the database rejects it, on both engines.

### US3 — The foreign key is restored for task activities (P1)

**As** a maintainer of the store,
**I want** the referential integrity PR #304 had to sacrifice,
**So that** an activity cannot point at a task that does not exist.

- **Given** a database at the target schema
- **When** the schema of `task_activities` is inspected
- **Then** `task_id` declares `REFERENCES tasks(id)`, and `project_id` declares
  `REFERENCES projects(id) ON DELETE CASCADE`.

- **Given** a project that is deleted
- **When** the deletion completes
- **Then** the activities attached to that project are removed with it.

- **Given** a task that is deleted
- **When** the deletion completes
- **Then** its activities are removed, as they already are today by the explicit
  delete in `DeleteTask`.

### US4 — Existing `sync-*` rows are migrated, none are lost (P1)

**As** a user of a server that has been running since PR #304,
**I want** the synchronisation history I already have to survive the change,
**So that** the upgrade does not silently erase part of my activity history.

- **Given** a database holding a row with `task_id = 'sync-<id of an existing project>'`
- **When** the migration runs
- **Then** that row ends up with `project_id = <that project's id>` and `task_id` null,
  and it is still visible in that project's history.

- **Given** a database holding a row with `task_id = 'sync-all'`, `'sync-github'`,
  `'sync-jira'`, or `sync-<id of a project that no longer exists>`
- **When** the migration runs
- **Then** that row ends up with `task_id` null and `project_id` null, and it is still
  visible in the "all projects" history.

- **Given** a database holding such rows on **either** engine — PostgreSQL databases
  created since #304 contain them too
- **When** the server starts
- **Then** the backfill runs on both engines; it is not gated behind the SQLite-only
  legacy migrations.

- **Given** a migration that has already run
- **When** the server starts again
- **Then** the migration is a no-op and no row changes.

### US5 — The project filter stops guessing (P2)

**As** a user filtering the activity history by project,
**I want** the filter to match on the recorded attachment,
**So that** I see the project's activities and only those.

- **Given** activities attached to project `P` and activities attached to tasks of `P`
- **When** the history is filtered on `P` (by id or by slug)
- **Then** both sets are returned, matched on `a.project_id` and on `t.project_id`
  respectively, with no substring match on `a.task_id` or `a.prompt`.

- **Given** an activity whose `prompt` happens to contain the identifier of an
  unrelated project `Q`
- **When** the history is filtered on `Q`
- **Then** that activity is not returned.

- **Given** the same filter applied to the activity statistics
- **When** the statistics are computed
- **Then** they count exactly the same set of activities as the list does.

### US6 — The API and the interface keep working unchanged (P2)

**As** a consumer of `/api/activities`,
**I want** the response shape to stay compatible,
**So that** the existing views keep rendering without modification.

- **Given** an activity with a null `task_id`
- **When** it is serialised by `/api/activities`
- **Then** `taskId` is the empty string, not `null`, and no field is removed.

- **Given** an activity attached to a project
- **When** it is serialised
- **Then** `projectId` carries that project's id.

- **Given** the existing `ActivitiesView` and `SyncView`
- **When** they query `/api/activities?projectId=…`
- **Then** they render the project's activities without any change to their code being
  required by this ticket.

---

## Acceptance criteria

The ticket's own criterion, unchanged by the clarification:

1. A synchronisation appears in its project's activity history, on **both** engines.
2. No row of `task_activities` carries a `task_id` that is not a task.
3. The foreign key is restored for task activities.

Plus, from the decisions above:

4. The exclusivity constraint is enforced by the database, on both engines.
5. No activity row is deleted by the migration; unresolvable `sync-*` rows survive as
   global activities.
6. The `LIKE` heuristics are gone from both `GetActivities` and `GetActivityStats`.
7. The `/api/activities` JSON contract is additive only.

---

## Open requirements

None. Round 2 of the clarification closed all six open questions, and the ticket owner
accepted every recommendation. Nothing in this specification is left to the
implementer's discretion at the behavioural level.

One item is deferred rather than open: autosync running without an `ActingUser` is
tracked as **#312** and must not be addressed here.

## Assumed, and revisable during implementation

- The interface does not need to change for the acceptance criteria to be met. A
  display adjustment remains possible if implementation shows one is necessary; it
  would be a small addition, not a contract change.
