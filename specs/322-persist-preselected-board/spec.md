# #322 — Persist the pre-selected board when it is confirmed unchanged

- Ticket: https://github.com/sebastienferry/sectile/issues/322 (Bug, `#clarified`)
- Branch: `feat/322`
- Follow-up of: #313 (PR https://github.com/sebastienferry/sectile/pull/321)
- Clarification: [`docs/clarifications/322.md`](../../docs/clarifications/322.md)
- Technical approach: [`plan.md`](./plan.md) — implementation checklist: [`tasks.md`](./tasks.md)

## Context

On a tracker that exposes boards (Jira today), the project settings show a "Board du tracker" picker. When the project has no
recorded `BoardID`, the picker displays the board Sectile would choose by default (the first scrum board, else the first board) as if
it were selected. Choosing that very board writes nothing: the picker cannot emit a change for the option it already shows, and the
editor also ignores a choice equal to the displayed one. The board only lands on the project through "Détecter les statuts" or the
next tracker sync.

The clarification settled the interaction (Q1 → A): while no board is recorded, the picker starts on a placeholder and the default
board is marked as suggested, so choosing it is a real choice.

Out of scope: how the default board is resolved, the "Détecter les statuts" flow, the tracker sync, and any server-side change.

## User stories

### US1 — Confirm the suggested board (priority: P1)

As the owner of a project with no recorded board, I open the project settings and pick the suggested board, so that it is recorded on
the project and its columns are imported, without going through "Détecter" or waiting for a sync.

**Acceptance scenarios**

1. **Given** a project with no recorded board and a tracker listing boards,
   **When** the settings open,
   **Then** the picker shows the placeholder "Choisir un board…" and the board Sectile would choose by default is labelled "(suggéré)".
2. **Given** that picker,
   **When** the user picks the suggested board,
   **Then** the board is recorded on the project and its columns are imported, exactly as for any other board.
3. **Given** that picker,
   **When** the user picks a board other than the suggested one,
   **Then** that board is recorded and its columns are imported.
4. **Given** a board was just recorded from the placeholder,
   **When** the picker is shown again,
   **Then** the recorded board is selected, the placeholder cannot be chosen back and no board is labelled "(suggéré)".

### US2 — Keep a recorded board as it is (priority: P1)

As the owner of a project whose board is already recorded, I see the picker behave as before.

**Acceptance scenarios**

1. **Given** a project with a recorded board present in the tracker's list,
   **When** the settings open,
   **Then** the picker shows that board selected, with no placeholder and no "(suggéré)" marker.
2. **Given** that picker,
   **When** the user picks another board,
   **Then** the new board is recorded and its columns are imported.
3. **Given** a choice equal to the board already recorded,
   **When** the editor handles it,
   **Then** nothing is imported again.

### US3 — A failed import changes nothing (priority: P2)

**Acceptance scenarios**

1. **Given** a project with no recorded board,
   **When** the user picks a board and the import fails,
   **Then** the picker returns to the placeholder and the project still has no recorded board.
2. **Given** a project with a recorded board,
   **When** the user picks another board and the import fails,
   **Then** the picker returns to the recorded board.

## Functional requirements

- **FR1.** While the project has no recorded board among the listed boards, the picker's displayed value is a placeholder option
  "Choisir un board…", which has an empty value and cannot be chosen.
- **FR2.** While no board is recorded, the board the default resolution would choose carries the "(suggéré)" marker in its label. No
  other board carries it, and no board carries it once a board is recorded.
- **FR3.** Choosing any board that differs from the recorded one records it on the project and imports its columns through the
  existing board import. This includes the suggested board when nothing is recorded.
- **FR4.** "Unchanged" means equal to the board recorded on the project, not to the board displayed in the picker.
- **FR5.** A failed import leaves the picker on the recorded board, or on the placeholder when none is recorded.
- **FR6.** A recorded board that the tracker no longer lists is treated as no recorded board (the default resolution already treats
  it that way).
- **FR7.** The server contract is unchanged: `POST /api/projects/{id}/board-columns` with `{ boardId }`.

## Success criteria

- Picking the suggested board from the picker records it; the next opening of the settings shows it selected.
- The decisions (recorded board, suggested board, whether a choice imports) are covered by `node --test` in
  `web/tests/boardColumns.test.mjs`.

## Open requirements

None. The clarification closed every product question.
