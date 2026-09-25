# Priority colour dot in the task edit forms

Scope restated from the clarification (`docs/clarifications/434.md`).

## User stories

### P1 — Read the priority of the task being edited at a glance

As someone editing a task, I want the priority field to show the same colour dot
as the task's card, so that the form reads the same way as the board.

### P2 — Choose a priority by colour when creating or cloning a task

As someone creating or cloning a task, I want the priority field of those forms
to show the same dot, so that the colour I pick is the colour the card will
carry.

## Functional requirements

1. The priority field of the task detail modal, of the quick add form and of
   the clone form shows a round dot next to the selected priority label.
2. The dot colour is the one the board card, the backlog row, the filter bar
   and the pinned bar use for that priority: urgent — danger, high — warning,
   medium — info, low — muted.
3. Changing the priority changes the dot immediately, before any save
   completes.
4. The field keeps its current behaviour: same four levels in the same order
   (urgent, high, medium, low), same labels, same save on change in the detail
   modal, operable with the keyboard, and its accessible name is unchanged. The
   dot is decorative and is not announced.
5. The colours shown by the card, the backlog row, the filter bar and the
   pinned bar do not change.
6. `CHANGELOG.md` carries one line under `[Unreleased]`.

## Acceptance scenarios

- **Given** a task with priority "high" open in the detail modal, **when** the
  modal renders, **then** the priority field shows a dot of the warning colour,
  the same as the task's card.
- **Given** the detail modal is open, **when** the user selects "urgent",
  **then** the dot turns to the danger colour and the task is saved with
  priority "urgent".
- **Given** the quick add form is opened, **when** it renders, **then** the
  priority field shows "medium" with the info-coloured dot.
- **Given** the clone form for a "low" task, **when** it renders, **then** the
  priority field shows the muted dot.
- **Given** a screen reader focuses the priority field, **when** it announces
  it, **then** it reads the label and the selected value only.

## Out of scope

- The roadmap `PRIORITY_META` badge, which uses its own palette on purpose.
- Colouring the options of the open list: native options cannot render a
  coloured shape on every platform.
- The desktop app, which has no task edit form.

## Open requirements

None.
