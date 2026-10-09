# Tasks #806 - Desktop project board

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Shared macro colour

- [ ] T1.1 Add `shared/epicColor.mjs` and `.d.mts` (`EPIC_PALETTE`, `fnv1a`,
      `epicColorIndex`, `epicColorHex`) (FR3).
- [ ] T1.2 Make `web/src/lib/epicColor.ts` delegate to it, `ACCENT_COLORS`
      unchanged.
- Tests: the web's existing epic colour tests unchanged; a new web test that
  `ACCENT_COLORS` hex values equal `EPIC_PALETTE`; a Desktop unit test that a
  pinned parent key gives the same hex as the web test pins (US6.2).

## 2. Shared stage resolution and move

- [ ] T2.1 Add `shared/workflowStage.mjs` and `.d.mts` (`WORKFLOW_STAGES`,
      `INTERNAL_STATUS_BY_STAGE`, `STAGE_LABELS`, `trackerBoard`,
      `columnOfTask`, `stageFromColumn`, `explicitStage`, `stageMove`)
      (FR2, FR4).
- [ ] T2.2 Make `web/src/lib/workflow.ts`, `stageMapping.ts` and
      `moveTaskWorkflowStage` delegate to it, results unchanged.
- Tests: the web's workflow and stage mapping tests unchanged; new unit tests
  for `stageMove` (label replaced, other labels kept, status per stage,
  tracker status from the first mapped column's first status, from the
  column name, kept when unmapped, per-tracker mapping) (US8.1, US8.2).

## 3. Desktop stage resolution

- [ ] T3.1 `taskStage(task, board)` in `desktop/src/workflow.mjs` with the
      column step (plan, Stage resolution) (US2.4, US2.5).
- [ ] T3.2 Pass the project's `board` from the tickets list and the sidebar
      grouping where they hold it.
- Tests (`workflow.test.mjs`): existing cases unchanged; explicit label beats
  column; column beats status; no board reads as today; a ticket of a second
  tracker reads that tracker's mapping.

## 4. Agent

- [ ] T4.1 Add the optional `board` field to `/desktop/project`, read from
      `/api/projects/:id`, omitted on failure (FR1).
- [ ] T4.2 Add `POST /desktop/tasks/stage-move` and the `stage-move`
      capability (FR5).
- Tests (`agent_desktop_test.go`): `board` copied from a fake server project,
  absent when the server fails; stage-move forwards exactly the allowed
  fields with `stageProjectId`, omits an empty tracker status, refuses GET,
  missing project, missing task, unknown status, passes a server 4xx through;
  capability listed (AC3).

## 5. Desktop IPC

- [ ] T5.1 `moveTaskStage` in `preload.cjs` and `move-task-stage` in
      `main.cjs`.
- [ ] T5.2 `fake-agent.cjs`: answer `board` on `/desktop/project`, finished
      tasks on `/desktop/tasks`, `/desktop/tasks/stage-move` (recording the
      request, with a switch to refuse it), and a switch to drop the
      `stage-move` capability.

## 6. Board helpers

- [ ] T6.1 `desktop/src/board.mjs`: `boardOptions`, `saveBoardOption`,
      `boardColumns`, `boardCardLabels`, `cardEpicColor`.
- Tests (`board.test.mjs`): defaults (finished hidden, condensed), saved
  values, throwing storage; six columns in order with empty ones; order
  within a column; workflow labels filtered; colour only with `epicColors`
  and a parent (US2.1, US2.6, US4, US5, US6).

## 7. Board page

- [ ] T7.1 Extract `taskActionsMenu` from `ticketRow` without behaviour
      change; the tickets UI tests stay green.
- [ ] T7.2 `openBoard`, toolbar, search, load, columns, collapsed strip,
      cards in both displays, mutual exclusion with the tickets list
      (US1.2, US1.4, US1.5, US2, US3, US4, US5, US6, US7).
- [ ] T7.3 Drag and drop behind the capability (US8).
- [ ] T7.4 `Open board` in the project menu, `Project board` in the palette
      (US1.1, US1.3).
- [ ] T7.5 Styles in `style.css`, light and dark.
- Tests (`board.ui.cjs`, after `npx vite build`): entry and palette open the
  board, close restores focus; six columns and counts; finished collapsed by
  default, expanded and remembered across a reload; card display toggle and
  persistence; condensed and full card content; colour bar on and off; card
  menu launches through the same request as a tickets row; drop sends the
  expected labels/status/tracker status and reloads; drop on the collapsed
  strip; refused drop shows the message and keeps the card; same-column drop
  sends nothing; no `draggable` without the capability; a board without
  `board` data still opens.
- Existing suites to rerun: `project-open-tasks.ui.cjs`,
  `stage-grouping.ui.cjs`, `task-engine.ui.cjs`, `declare-reviewed.ui.cjs`,
  `palette-add-project.ui.cjs`, `workflow.ui.cjs`.

## 8. Documentation

- [ ] T8.1 `docs/USER_GUIDE.md`: the board, its entries, its options, the
      drop and what it does not do (no skill launched) (AC5).
- [ ] T8.2 `CHANGELOG.md`, `[Unreleased]` → `Added`: one line naming the
      Desktop project board and moving a card between stages (AC4).

## 9. Checks

- [ ] `go test ./internal/agent/...` (outside the sandbox for httptest).
- [ ] `node --test desktop/tests/*.test.mjs`.
- [ ] Web unit tests and `tsc` for the moved helpers.
- [ ] Desktop UI suites listed in step 7 (sandbox off, run serially).
