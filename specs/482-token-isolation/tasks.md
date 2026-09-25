# Tasks #482 - The server tracker credential never signs a person's action

Ordered checklist. Each group is one commit (Conventional Commits). Before
starting, `git fetch origin` and integrate `origin/main` (merge, the branch is
pushed); grep `git log origin/main` for `(#482)` and for `ForWrite` /
`WithUnattended` in case a parallel run landed part of it.

## 1. The rule (FR-2, FR-3, FR-8, FR-10) - `feat(tracker)`

- [x] T1.1 `tracker.WithUnattended` / `tracker.Unattended` in
  `internal/tracker/tracker.go`, with tests (a named user wins over the marker;
  nil context answers false).
- [x] T1.2 `trackerapi.MissingPersonalCredentialError{Tracker}` and
  `ErrNoActingUser`; `(*Client).ForWrite(ctx, tracker, projectID)` per plan
  §1.2.
- [x] T1.3 Table test for `ForWrite`: five rows × `jira`, `github`, `gitlab`,
  including a sealed personal credential (error, no fallback).
- [x] T1.4 `JiraAdapter.forWrite`; switch every write method listed in plan
  §1.3 (including `jira_sprints.go`); the old Jira refusal string is replaced
  by the typed error.
- [x] T1.5 `GithubAdapter.forWrite`; switch `CreateIssue`, `UpdateIssue`,
  `DeleteIssue`, `AddComment`, `UpdateLabels`; rewrite the `forProject` doc
  comment (reads only).
- [x] T1.6 Adapter tests: each write method refused for a user without a token
  with no HTTP request sent; reads unchanged (GitHub: server token; Jira: as
  today).

## 2. Unattended callers and queued ops (FR-1, FR-3, FR-4) - `fix(db)`

- [x] T2.1 Inventory every tracker write call site (grep in plan §2) and
  classify it; keep the table for the PR description.
- [x] T2.2 Mark the synchronisation pass (timer and manual) with
  `tracker.WithUnattended` at its entry point; mark any other unattended site
  found in T2.1.
- [x] T2.3 `TrackerOp.Unattended`, set by `EnqueueTrackerOp` /
  `enqueueTrackerOpUnsafe` from the context; the worker adds the marker only
  when set. An op with neither fails with `ErrNoActingUser`.
- [x] T2.4 Tests: sync writes are unattended and carry the server token; a
  queued op with neither user nor marker fails its activity.

## 3. Stage report comment (US1, FR-7) - `fix(db)`

- [x] T3.1 `runStageOp`: `AddTaskCommentAs(ctx, …)`; error → failure step,
  activity failed, local stage kept; success step only on success.
- [x] T3.2 Tests: comment made as `op.UserID` (Jira and GitHub fakes); user
  without a token → stage kept locally, activity failed, failure step present.

## 4. MCP and REST entry points (US2, US4, FR-6) - `fix(taskmcp)`

- [x] T4.1 `requireCaller` in `internal/taskmcp`; called first by
  `create_task`, `update_task`, `add_comment`, `transition_stage`.
- [x] T4.2 `create_task` uses `CreateTaskAs(tracker.WithActingUser(ctx,
  caller.UserID), …)`.
- [x] T4.3 Handler error mapping: `ErrNoActingUser` and
  `MissingPersonalCredentialError` → 403 with their message.
- [x] T4.4 Tests: each MCP write tool with an unresolved caller refused, no
  tracker call; read tools still answer; `create_task` as a resolved caller
  reaches the adapter with that user; REST write through an agent credential
  without a device → 403.

## 5. Conversion and macros (US2) - `fix(db)`

- [x] T5.1 `ConvertTaskToRemote(ctx, …)`; handler passes
  `h.actingContext(r)`; refusal releases the claim, task stays local.
- [x] T5.2 `(*DB).trackerForWrite(ctx, tracker, projectID)`.
- [x] T5.3 Add `ctx` to `UpdateMacro`, `CreateMacro`, `DeleteMacro`,
  `CreateStoryUnderMacro`, `CreateStoryFromMacroTodo`, `MigrateMacro`; thread
  it from handlers and MCP; update every caller and test.
- [x] T5.4 Milestone writes via `trackerForWrite`; milestone reads keep
  `d.tracker`; dropped write errors surfaced (plan §3.4); `CreateStoryUnderMacro`
  uses `CreateTaskAs(ctx, …)` and `SetParent` uses `ctx`. Check `SetTaskMacro`
  and `MoveTasksToMacro`.
- [x] T5.5 `trackerAs` doc comment: read helper only.
- [x] T5.6 Tests: conversion as the user and refused conversion; story under a
  macro (create + parent) as the user; GitHub milestone create / update /
  delete / set / migrate as the user, listing with the server token.

## 6. Managed-run post-back (US3, FR-5) - `fix(db)`

- [x] T6.1 `PostBackTask` builds its tracker context from the run's launcher,
  or `WithUnattended` when none is recorded.
- [x] T6.2 Tests: launcher with a token (writes as launcher); launcher without
  (stage kept, tracker step failed with the typed message); no launcher
  (server token).

## 7. Dead code - `refactor(db)`

- [x] T7.1 Delete `pushStageToTracker` (`internal/db/interactive.go`).

## 8. Documentation - `docs`

- [x] T8.1 ADR `0029-server-tracker-credential-signs-unattended-work-only.md`
  (verify the free number on `origin/main`); ADR 0028 marked *amended by 0029*.
- [x] T8.2 `CHANGELOG.md` `[Unreleased]`: breaking **Changed** line and
  **Fixed** line (spec § Documentation acceptance).
- [x] T8.3 `README.md` § personal tracker credentials (~line 307).

## 9. Verification

- [x] T9.1 `go vet ./...` and
  `go test ./internal/tracker/... ./internal/trackerapi/... ./internal/db/... ./internal/taskmcp/... ./internal/handlers/...`
  (the handlers keepalive test can flake under load: rerun before blaming the
  change).
- [x] T9.2 PostgreSQL run of `internal/db` when a throwaway DSN is available
  (never the dev database: the suite truncates it).
- [x] T9.3 Re-run the T2.1 grep: no tracker write left on
  `context.Background()` without `WithUnattended`, and none on `d.tracker(` or
  `trackerAs(`.

## Implementation notes

What the implementation found that the plan did not say, and what it did
about it. Acceptance criteria are unchanged.

- **T6, managed-run post-back.** `PostBackTask` writes nothing to the tracker:
  it records the result locally and refreshes pull-request states (reads). A
  managed run reaches the tracker through a stage transition, over MCP
  (`transition_stage`) or REST (`/api/tasks/{id}/stage`), made with the key of
  the agent that launched it, so the report and labels go out as that person
  through the stage op (T3). No job in the code base transitions a stage for
  nobody; a transition that names nobody now fails with `ErrNoActingUser`
  (covered by `TestQueuedWriteNeedsAPersonOrTheUnattendedMarker`).
- **More paths lost their author than the plan listed.** `MoveTask` (board
  drag and drop), `CloneTask` and `CompleteInteractiveStep` queued or made
  their tracker writes with nobody named. They now take the acting person
  (`MoveTaskBy`, `CloneTask(ctx, …)`, `CompleteInteractiveStep(actorID, …)`),
  and the actorless `enqueueTrackerUpdateUnsafe` and `AddTaskComment` are gone.
- **`SaveMacroMeta` is local only.** It used to call `UpdateMacro`, which sent a
  milestone PATCH even when nothing the milestone carries had changed. Its
  callers only save the horizon, framing or slicing; it now writes the local
  row alone, and `UpdateMacro` skips the milestone when title, description and
  state are all untouched.
- **Macro refusals.** Create, edit and delete keep their local effect, as any
  failed milestone write did, and return the refusal (a 403 over REST). A
  migration resolves its GitHub credentials first and is refused before
  anything moves. Other milestone failures stay best effort, as before.
- **MCP anonymous caller.** `Caller.Anonymous` is set for the shared server key
  (a credential paired to no device); `requireCaller` refuses it and a caller
  with no user id.
