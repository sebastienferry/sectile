# Plan #522 - Batch membership and member states

Implements `specs/522-when-running-batch/spec.md`. Behaviour lives in the spec;
this file says where and how.

## Stack and constraints

- Server: Go, `internal/db` (SQLite and PostgreSQL), `internal/handlers`,
  `internal/taskmcp`, `internal/skills`.
- Web: React + TypeScript, `web/src`. Unit tests `web/tests/*.test.mjs`
  (`node --test`), browser tests `web/tests/*.browser.mjs` (Playwright, see
  README "Browser tests").
- The desktop app is not touched.
- A new table goes in `migrations.go` only, never in the frozen baseline, and
  the forget-and-replay fixtures must drop it (`forgetSchemaVersion` in
  `internal/db/migrations_test.go`, and the `DROP TABLE` lists in
  `activerun_test.go` and `specartifacts_test.go`).

## What the code does today

- `confirmBatchPickup` (`web/src/context/AppContext.tsx`) calls
  `runSkill(taskIds[0], 'pickup_issues', buildBatchPickupPrompt(...))`, a
  `POST /api/tasks/{id}/run-skill` with `{skillId, prompt, ...}`. The other
  ticket IDs exist only in the prompt.
- The web launch path is the local agent path only
  (`internal/handlers/handlers.go`, `run-skill` sub-action): it checks
  `ActiveRunOnTask`, then records an agent-owned `remote_run` with
  `StartAgentRun`, then dispatches. Without an agent it answers 409.
- `startRemoteRun` (`internal/db/remoterun.go`) reuses a run only when
  `activity.TaskID == task.ID`; otherwise "remote run does not match an active
  execution on this task". A client run started without a run ID is always
  concurrent and never refused (#308), so FR11 already holds for that call.
- Cards and rows read the global activity list (`GET /api/activities`, limited
  to the 100 newest by default) and filter it by `taskId`. Tasks come from
  `GET /api/tasks`. The web refetches both on the SSE `task_updated` event,
  which `notifyPostBackListeners` triggers.

## Design

### Data: table `batch_members` (migration 30)

```sql
CREATE TABLE batch_members (
    run_id   TEXT NOT NULL,     -- the batch run: task_activities.id
    task_id  TEXT NOT NULL,     -- the member: tasks.id
    position INTEGER NOT NULL,  -- 1..N, launch order; the lead is 1
    state    TEXT NOT NULL DEFAULT 'waiting', -- waiting | processing | done
    PRIMARY KEY (run_id, task_id)
);
CREATE INDEX idx_batch_members_task ON batch_members (task_id);
```

- Plain columns, no foreign key, like `task_activities.task_id`: a deleted task
  leaves a harmless row that no join returns.
- Rows are never deleted when the batch ends: "running batch" is always
  computed from the batch run's status, so an ended batch shows nothing (FR US4)
  without a cleanup step that could be missed.
- Same SQL for both engines; no engine-specific branch.

### DB API (`internal/db/batch.go`, new)

- `RecordBatch(runID string, taskIDs []string) error`: inserts the rows in one
  transaction, position 1 `processing`, the others `waiting` (FR4).
- `MarkBatchMemberProcessing(runID, taskID string) (bool, error)`: in one
  transaction, sets the current `processing` row of the run to `done` and the
  given one to `processing` (FR5); no-op when it is already `processing`.
  Returns false when the task is not a member of the run.
- `ActiveBatchOf(taskID string) (*models.TaskBatch, error)`: the membership of
  the task in a running batch, joined with the batch run
  (`task_activities.status IN activeRunStatuses`) and the lead task's key.
  `nil` when none.
- `ActiveBatchesByTask(taskIDs []string) (map[string]models.TaskBatch, error)`:
  the same for a task list in one query, for `GetTasks`.

### Models

```go
// TaskBatch is a task's place in a running batch.
type TaskBatch struct {
    RunID      string `json:"runId"`
    LeadTaskID string `json:"leadTaskId"`
    LeadKey    string `json:"leadKey"`
    Position   int    `json:"position"`
    Size       int    `json:"size"`
    State      string `json:"state"` // waiting | processing | done
}
```

- `Task` gains `Batch *TaskBatch \`json:"batch,omitempty"\``, filled by
  `GetTasks` and `GetTaskByID`. Only a running batch fills it.
- `RunSkillRequest` gains `BatchTaskIDs []string \`json:"batchTaskIds,omitempty"\``.
- `TaskBusyError` gains `Batch *models.TaskBatch`, set when the busy cause is a
  batch rather than the task's own run.

Membership is exposed on the task rather than on the batch run's activity. The
activity list the web reads is capped at the 100 newest rows, and a long batch
writes many tracker activities: its run can fall out of the list while it is
still running. Tasks are always loaded whole.

### Busy rule

- `ActiveRunOnTask(taskID)`: unchanged first query. When it finds nothing, it
  looks up `ActiveBatchOf(taskID)` and, if found, returns the batch run's
  activity. A new `ActiveBusyCause(taskID) (*TaskActivity, *TaskBatch, error)`
  wraps both so the handler can name the batch; `ActiveRunOnTask` keeps its
  signature for its other callers (`enqueueSkillOnTask`, the macro path).
- The lead ticket is busy through its own run as today; `ActiveBusyCause`
  still reports its batch so the message is the batch one.
- `describeActiveRun` gains a batch branch:
  `"%s is part of the batch led by %s, which is still running."` (FR8), in
  English like the neighbouring messages. The 409 body keeps `error` and
  `activeRunId`, and adds `batchLeadKey`.
- "Launch anyway" (FR9): the existing `req.Force` branch applies unchanged, with
  `active.UserID` being the batch run's owner.
- Residual race: a single launch on a member and the batch launch that adds it
  can cross between the check and the insert, because membership is not covered
  by `idx_activities_one_active_run`. The window is the few milliseconds
  between the checks and `RecordBatch`; it is accepted and documented in the
  ADR rather than closed with a cross-table lock.

### Batch launch (`run-skill` handler)

When `req.BatchTaskIDs` is not empty:

1. Validate FR3: `SkillID == "pickup_issues"`, at least two IDs, no duplicate,
   first ID equal to the path task, every task found and in the task's project.
   Otherwise 400 with a message, nothing recorded.
2. For every member, `ActiveBusyCause`; the first busy one answers 409 naming
   it (FR10). "Launch anyway" does not apply to a batch launch: `Force` with
   `BatchTaskIDs` is 400.
3. `StartAgentRun` on the lead as today (its insert still settles the lead's
   race), then `RecordBatch(remoteRun.ID, ids)`. If `RecordBatch` fails, the
   run is finished as failed with the error and the launch answers 500, the
   same way a failed dispatch closes its run.
4. Notify listeners for every member (a `task_updated` per member) so every
   viewer's board picks the batch up (FR17).

The prompt built by `buildBatchPickupPrompt` is unchanged; the server does not
parse it.

### Member report (`start_run` with a batch run ID)

In `startRemoteRun`, the reuse branch accepts
`activity.TaskID != task.ID` when the run is a running `remote_run` and
`MarkBatchMemberProcessing(runID, task.ID)` returns true; it returns the batch
run activity (FR6). The lead ticket with its own run ID also calls
`MarkBatchMemberProcessing`, so reporting the lead again works (FR5). A task
that is not a member keeps today's error. Listeners are notified for the member
that changed and the one that became `done`.

The MCP `start_run` handler needs no change beyond its description: it already
passes the run ID through, and a reused run is not adopted by the session.
`finish_run` is unchanged: it is called once, on the lead, with the batch run.

### Agent contract

- `internal/skills/fragments/contracts/transition.md`: replace "A batch tracks
  each task separately." with, for `pickup_issues` only: "For a batch, reuse the
  launch runId for every ticket: call start_run with that ticket's full key and
  the runId when you begin working on it, and call finish_run once, on the first
  ticket, when the whole batch ends." (FR19)
- `internal/taskmcp/server.go`: append to the `start_run` description "For a
  batch, call it with each ticket's key and the batch runId when work on that
  ticket begins."
- Regenerate the goldens (`UPDATE_GOLDEN=1 go test ./internal/skills/`) and
  review the diff: only the batch lines may change.

### Web

- `web/src/types/index.ts`: `TaskBatch` type, `Task.batch?: TaskBatch`.
- `web/src/context/AppContext.tsx`: `runSkill` accepts an optional
  `batchTaskIds` and sends it; `confirmBatchPickup` passes `taskIds`. The error
  toast already shows the server's message (FR8, no client change).
- `web/src/lib/batchMembership.ts` (new, pure): `batchIndicator(task)` returns
  `null` or `{ leadKey, position, size, state, tone: 'amber'|'indigo'|null,
  labelKey }`, used by the three surfaces so they cannot disagree.
- `web/src/components/BatchBadge.tsx` (new): renders the badge, the tooltip and
  the state label from `batchIndicator` and the locale.
- `TaskCard.tsx`: render `BatchBadge` next to `RemoteRunBadge`; when
  `batchIndicator(task)` is not null its tone replaces the `isRunning` /
  `isQueued` border (FR14). `RemoteRunBadge` untouched (FR15, FR16).
- `ListView.tsx`: render `BatchBadge` next to `RemoteRunBadge` in the row.
- `TaskDetailModal.tsx`: render `BatchBadge` next to `RemoteRunBadge` in the
  header.
- `web/src/locales/translations.ts`: a `batchMember` group in both locales:
  `badge` (`Lot {key}` / `Batch {key}`), `tooltip` (`Lot mené par {key} ·
  ticket {position} sur {size}` / `Batch led by {key} · ticket {position} of
  {size}`), `waiting` (`en attente dans le lot` / `waiting in batch`),
  `processing` (`en cours` / `in progress`).

Live updates need nothing new on the web: every server change above emits
`task_updated`, on which the web refetches tasks.

## Rejected alternatives

- **Membership on the batch run's activity**: the web activity list is capped
  at 100 rows, so a long batch would lose its indicators mid-run.
- **Deriving the processing member from stage transitions**: the batch records
  `reviewed` on every ticket at the end, which would move the marker backwards;
  and a ticket already at a late stage would look done before it is touched.
- **One run per member**: several ordinary active runs, one per ticket, for one
  agent process. It breaks the ownership rules (who finishes which run) and
  `finish_run` on the lead would leave member runs open.
- **A new MCP tool for member progress**: one more tool to install on every
  agent configuration (`genericMCPTools`, `agentmcp`), for what `start_run`
  with the batch run ID already expresses.

## Documentation

- `docs/adrs/0034-batch-membership.md`: the table, the explicit processing
  marker through `start_run`, the busy rule extension and the accepted race.
- `docs/contracts/server-agent-v1.md` and `docs/CAPABILITIES.md`: `start_run`
  with a batch run ID; `batchTaskIds` on `run-skill`; the 409 `batchLeadKey`.
- `CHANGELOG.md`: a `Fixed` line under `## [Unreleased]` (#522).

## Target files

| File | Change |
| --- | --- |
| `internal/db/migrations.go` | migration 30 `batch_members` |
| `internal/db/batch.go` (new) | membership API |
| `internal/db/remoterun.go` | batch reuse in `startRemoteRun` |
| `internal/db/skillresult.go` | `ActiveBusyCause` |
| `internal/db/activerun.go` | `TaskBusyError.Batch` |
| `internal/db/db.go` | `Task.Batch` in `GetTasks` / `GetTaskByID` |
| `internal/models/models.go` | `TaskBatch`, `Task.Batch`, `RunSkillRequest.BatchTaskIDs` |
| `internal/handlers/handlers.go` | batch launch, busy message, 409 body |
| `internal/taskmcp/server.go` | `start_run` description |
| `internal/skills/fragments/contracts/transition.md` + goldens | FR19 |
| `internal/db/migrations_test.go`, `activerun_test.go`, `specartifacts_test.go` | drop `batch_members` |
| `web/src/types/index.ts`, `web/src/context/AppContext.tsx` | batch launch |
| `web/src/lib/batchMembership.ts`, `web/src/components/BatchBadge.tsx` (new) | indicator |
| `web/src/components/TaskCard.tsx`, `ListView.tsx`, `TaskDetailModal.tsx` | display |
| `web/src/locales/translations.ts` | labels |
| `docs/...`, `CHANGELOG.md` | documentation |
