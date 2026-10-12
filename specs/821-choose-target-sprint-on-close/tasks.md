# Tasks #821 - Choose the target sprint when closing a sprint

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Server

- [x] T1.1 Add `MoveOpenToSprint` to `models.SprintPatch` and document the
      `sprint` destination (FR1).
- [x] T1.2 `moveOpenWork` resolves a `sprint` destination and refuses an
      empty, unknown, closed or closing-sprint id before the tracker is called
      (FR1, FR2).
- Tests: close to a chosen sprint; four refusals, each with no tracker
  request and the sprint still open (US1.3, US3.1). Existing `next` tests
  unchanged (US3.2).

## 2. Web helpers

- [x] T2.1 `nextSprintAfter` follows the server's undated fallback (FR3).
- [x] T2.2 Add `closeTargets` (US1.1).
- [x] T2.3 `SprintPatch` type gains `sprint` and `moveOpenToSprint`.
- Tests: `web/tests/sprintManagement.test.mjs` for both helpers.

## 3. Close dialog

- [x] T3.1 Target state, preselection and empty-list fallback (US1.2, US1.4).
- [x] T3.2 Tracker path sends the chosen id; local path moves by name (FR4).
- [x] T3.3 Automatic start follows the target, or the next sprint for
      backlog/keep; checkbox shown only for a `future` sprint (US2).
- [x] T3.4 Dialog markup: the select in the first option; locale strings in
      French and English.
- Tests: `tsc`, `oxlint`, the locale catalog test.

## 4. Documentation

- [x] T4.1 `CHANGELOG.md`: one `Added` line under `## [Unreleased]` (FR6).
