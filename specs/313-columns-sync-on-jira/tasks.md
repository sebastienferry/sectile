# #313 — Implementation checklist

Ordered so the backend answers before the UI calls it. References: [`spec.md`](./spec.md),
[`plan.md`](./plan.md).

## 1. Backend — status palette (D1, FR4, US3, US5)

- [ ] T1.1 In `internal/db/board.go`, extract the GitHub ProjectsV2 block of
      `GetProjectTrackerStatuses` into a private helper, leaving its behaviour and its
      `open` / `closed` fallback identical.
- [ ] T1.2 Add the non-GitHub arm: resolve the tracker with `trackerReaderFor`, return an
      empty list when `CapBoard` is absent, otherwise call `ListStatuses` under
      `boardAPITimeout` and return the names, de-duplicated case-insensitively, in tracker
      order.
- [ ] T1.3 Propagate the tracker error instead of swallowing it (FR7).

## 2. Backend — column detection (D2, FR3)

- [ ] T2.1 Add `DetectProjectBoardColumns(ctx, projectID) ([]models.TrackerColumn, error)`
      in `internal/db/board.go`: resolve the board (project `BoardID`, else first scrum
      board, else first board) and return `ts.ListBoardColumns` for it.
- [ ] T2.2 In `internal/handlers/handlers.go` `/api/projects/detected-statuses`, when
      `projectId` is present and the tracker has `CapBoard`, add `columns` to the response
      and keep `statuses` as the palette union. Leave the draft-project and GitHub paths
      byte-identical (FR8).

## 3. Backend — automatic sync (D5, D6, FR5, FR6, US4)

- [ ] T3.1 In `SyncProjectBoardColumns`, persist the board resolved implicitly into
      `Project.BoardID` before applying the columns.
- [ ] T3.2 In `internal/db/db.go` `afterTrackerSync`, run the board step whenever
      `ts.Supports(tracker.CapBoard)`, dropping the `BoardID != ""` guard; keep the
      warning-step behaviour on error.
- [ ] T3.3 Verify no merge rule changed: names and order from the tracker, hand-assigned
      unclaimed statuses kept, hidden flag kept, unknown user columns appended, stage
      mappings to vanished columns dropped.

## 4. Backend — English strings (D7, NFR4)

- [ ] T4.1 Translate the user-facing Go strings and the comment block on the lines touched
      by tasks 1-3 in `internal/db/board.go`. Do not translate untouched lines.

## 5. Frontend — board picker (D4, FR1, FR2, US1)

- [ ] T5.1 In `BoardColumnsEditor.tsx`, load the boards with `listProjectBoards` for a
      saved project and hide the picker when the list is empty or the call failed.
- [ ] T5.2 Render the selector above the columns, labelled with each board's name and
      type, pre-selected per D4 (project `boardId`, else first scrum, else first).
- [ ] T5.3 On change, call `importProjectBoardColumns`, push the returned
      `trackerColumns` / `stageColumns` up through the existing callbacks, and refresh the
      palette.

## 6. Frontend — mirror the board columns (D3, FR3, US2)

- [ ] T6.1 Read the optional `columns` field of the detection response; add its type to
      `web/src/types/index.ts` if the response type is declared there.
- [ ] T6.2 When `columns` is non-empty, build the column list from it (board order,
      grouped statuses) and merge with the current columns by lower-cased name, keeping
      the hidden flag, hand-assigned unclaimed statuses, and unknown user columns at the
      end.
- [ ] T6.3 Keep the existing one-column-per-status path when `columns` is absent.
- [ ] T6.4 Report an empty or failed detection with the message the API returned rather
      than the generic "Aucune colonne trouvée" when an error is available (FR7).

## 7. Tests

- [ ] T7.1 `internal/db` — `GetProjectTrackerStatuses` with a fake `CapBoard` tracker
      returns its statuses; with a GitHub project the result is unchanged; without
      `CapBoard` it returns an empty list and no error.
- [ ] T7.2 `internal/db` — `SyncProjectBoardColumns` regression on the merge: hand-assigned
      unclaimed status survives, hidden flag survives, unknown user column kept then
      dropped when emptied, stage mapping to a vanished column dropped.
- [ ] T7.3 `internal/db` — `SyncProjectBoardColumns` with an empty `BoardID` resolves and
      persists the first scrum board.
- [ ] T7.4 `internal/handlers` — `/api/projects/detected-statuses` returns `columns` for a
      `CapBoard` project and the unchanged payload for the GitHub and draft paths.
- [ ] T7.5 `web` — the editor builds one column per board column when `columns` is
      present, and keeps the per-status behaviour when it is absent.

## 8. Validation gates

- [ ] T8.1 `go build ./...`
- [ ] T8.2 `go test ./...`
- [ ] T8.3 The web build and its test suite (per the repository `Makefile`).
- [ ] T8.4 Manual pass on a Jira project: pick a board, press "Détecter", drag a status,
      run a tracker sync, confirm acceptance criteria 1-4 of `spec.md`.
- [ ] T8.5 Manual pass on a GitHub project: detection, palette and sync steps unchanged
      (acceptance criterion 5).

## 9. Documentation

- [ ] T9.1 Update `CHANGELOG.md` if present, under Fixed, referencing #313.
- [ ] T9.2 Update `/docs` only if a page documents the board-column configuration.
