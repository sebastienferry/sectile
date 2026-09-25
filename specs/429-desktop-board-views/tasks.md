# Tasks: #429 desktop board views

Spec: [`spec.md`](spec.md). Plan: [`plan.md`](plan.md).

## Server

- [ ] T1 Migrations 23 (`board_views.repository`) and 24
  (`tasks.view_repository`, `tasks.view_repository_by`); extend the rewind and
  replay helpers (`forgetSchemaVersion`, `dropRepositoryColumns` siblings,
  `activerun_test.go`).
- [ ] T2 `BoardView.Repository`: model, request, SQL, validation, 400 mapping.
  Tests: create, update keeps it, clear it, invalid remote refused.
- [ ] T3 `Task.ViewRepository`: read in every task SELECT;
  `RecordTaskViewRepository`. Test: recorded, read back, user kept.
- [ ] T4 run-skill `viewId`: ownership, project selection, recording after the
  busy check. Tests: recorded; foreign view 404; view without the project 400;
  view without repository keeps the recorded one.
- [ ] T5 Evidence: `resolveStagePRTarget` with the task, `evidencePrimary`,
  adjust prerequisite. Tests: mono-repo accepts the view repository's pull
  request; mono-repo still refuses another; lookup by branch in the view
  repository without a URL; adjust reads there.
- [ ] T6 Discovery in the view repository. Tests: attached with a tracker that
  cannot discover; gate respected; failure is a warning.

## Agent

- [ ] T7 `Overrides.ViewDirectories` read, merged and written. Test: round trip
  and clearing.
- [ ] T8 `/desktop/views` GET and POST, `board-views` capability. Tests: list with
  directory; save with a matching origin; warning on a mismatch; refusal outside
  a checkout; clear.
- [ ] T9 `/desktop/tasks` with `viewId` (GET and POST), view root order, recorded
  root used by `admitProjectRun` and `prepareDispatchLocked`. Tests: GET forwards
  `viewId`; POST forwards `viewId` and launches in the view directory; the mapping
  of the view repository is used without a directory; a launch without a view
  forgets the root.

## Desktop

- [ ] T10 IPC and preload entries, gated on the capability.
- [ ] T11 Chooser with views; Tickets pane with a view scope and per-row project
  info; launch with the row's project and the view ID; relaunch and next step
  keep the view.
- [ ] T12 View directory action and warning.
- [ ] T13 UI test `desktop/tests/view-tickets.ui.cjs`: chooser lists a view; pane
  lists two projects' rows; launches carry each row's project and the view ID;
  the directory dialog shows the warning; without the capability only projects
  are offered. Existing `project-open-tasks.ui.cjs` unchanged.

## Web

- [ ] T14 `repository` in types, request and `BoardViewModal`; validation rule in
  `lib/boardViews.ts` with a unit test in `web/tests/boardViews.test.mjs`.

## Wrap-up

- [ ] T15 `CHANGELOG.md` `Added` line under `[Unreleased]`.
- [ ] T16 `go build ./...`, `go vet ./...`, `go test ./...`; `web`: `npm run
  build` (tsc) and `node --test`; `desktop`: `npm test`, `npx vite build`,
  `npm run test:ui` for the touched UI tests.
