# #322 — Technical plan

References: [`spec.md`](./spec.md), [`docs/clarifications/322.md`](../../docs/clarifications/322.md).

## Stack and scope

Web only: React 19 + TypeScript in `web/`. Tests use the `node --test` suite (`web/tests/*.test.mjs`), which imports the TypeScript
helpers in `web/src/lib/` directly. No Go change, no API change.

## Root cause

- `BoardColumnsEditor` initialises its `boardId` state with `pickBoardId(boards, project.boardId)`, so the `<select>` displays the
  default board as selected when nothing is recorded.
- A native `<select>` fires no `change` event when the user picks the option already selected, so `handleBoardChange` is never
  called for that board.
- `handleBoardChange` also returns early when `nextBoardId === boardId`, the *displayed* value.

## Design

### D1 — Pure decisions in `web/src/lib/boardColumns.ts`

Next to `pickBoardId`, add three small helpers so the behaviour is testable without rendering React:

- `recordedBoardId(boards, current)`: `current` when it names a listed board, else `''` (FR1, FR6).
- `suggestedBoardId(boards, recorded)`: `''` when a board is recorded, else `pickBoardId(boards)` (FR2).
- `shouldImportBoard(next, recorded)`: `true` when `next` is non-empty and differs from `recorded` (FR3, FR4).

`pickBoardId` is unchanged: the server's `resolveBoardID` and the sync keep the same default.

### D2 — The editor tracks the recorded board locally

`BoardColumnsEditor` receives `editingProject` from `ProjectModal`, a snapshot that is not refreshed after an import. Comparing
against `project.boardId` alone would go stale after the first import of the session (a later failure would fall back to the
placeholder although a board is now recorded). The editor keeps a `recordedBoard` state:

- initialised with `recordedBoardId(found, project.boardId)` when the boards load;
- set to the imported project's `boardId` after a successful import (the server's answer, i.e. the persisted value);
- used by the guard (`shouldImportBoard`), by the failure fallback (FR5), by the placeholder and by the suggested marker.

The displayed `boardId` state starts at `recordedBoard` (so `''` shows the placeholder) and is set optimistically to the choice while
the import runs, as today.

### D3 — Picker markup

- When `recordedBoard` is empty, render first `<option value="" disabled>Choisir un board…</option>`.
- Append ` (suggéré)` to the label of the option whose id equals `suggestedBoardId(boards, recordedBoard)`.
- Labels stay in French: the settings UI speaks French (AGENTS.md).

### Rejected alternatives

- **Removing the guard only**: fixes nothing, the browser emits no event (clarification, round 1).
- **A "Retenir ce board" button** or **auto-persisting on open** (Q1 options B and C): declined by the owner.
- **Refreshing `editingProject` from the context** in `ProjectModal`: a broader change to the modal's state flow than this bug needs.

## Data contracts

Unchanged. `importProjectBoardColumns(projectId, boardId): Promise<Project | null>`; the returned `Project.boardId` is the recorded
board.

## Target files

- `web/src/lib/boardColumns.ts` — the three helpers.
- `web/src/components/BoardColumnsEditor.tsx` — `recordedBoard` state, guard, fallback, placeholder and marker.
- `web/tests/boardColumns.test.mjs` — tests for the helpers.

## Verification

`npm test`, `npm run lint` and `npm run build` in `web/`.
