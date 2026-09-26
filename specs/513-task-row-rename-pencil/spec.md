# Feature Specification: Pencil rename instead of the task row "…" menu

**Ticket**: [#513](https://github.com/sebastienferry/sectile/issues/513)
**Branch**: `feat/513`
**Clarification**: [docs/clarifications/513.md](../../docs/clarifications/513.md)
**Status**: Specified

## Summary

In the Sectile Desktop sidebar, every local task row ends with a "…" button
that opens a dialog of four actions: Relaunch, Detach to native terminal,
Archive and Rename locally. Only the rename is not available anywhere else:
Relaunch and Detach are toolbar buttons of the selected task, and Archive is
already a button of the row. The "…" button and its dialog go away, and a
pencil button takes their place. The pencil turns the task title into a text
field inside the row, where the new local name is typed.

Out of scope: the project row "…" menu, the "…" menu of the ticket list in the
main panel, the row's archive button, and the rename rules beyond what is
stated below (no reset to the tracker title is added).

## User stories

### US1 - I rename a task from its row, in place (P1)

As a desktop user, I want to rename a task directly in the sidebar row, so that
I give it a name that means something to me without opening a dialog.

**Acceptance scenarios**

1. **Given** a local task row, **when** I hover it or move the keyboard focus
   into it, **then** a pencil button appears at the end of the row, where the
   "…" button used to be, with the accessible name and tooltip
   `Rename <task name>`, `<task name>` being the name the row displays.
2. **Given** a touch device (no hover), **then** the pencil is always visible,
   like the archive button.
3. **Given** the pencil is visible, **when** I press it, **then** the row's
   title is replaced by a text field holding the current name, with the text
   selected and the keyboard focus in the field.
4. **Given** the field is open, **when** I type a new name and press Enter,
   **then** the field closes, the row shows the new name, the toolbar title of
   the task shows it when the task is selected, and the name is kept after the
   app restarts.
5. **Given** the field is open with a changed name, **when** the field loses
   the focus (I click elsewhere or tab away), **then** the name is saved as
   with Enter.
6. **Given** the field is open, **when** I press Escape, **then** the field
   closes and the row shows the name it had before, nothing is saved, and the
   Escape does not also close another surface (the ticket pane, a dialog).
7. **Given** the field is open, **when** I confirm an empty value or a value
   made only of spaces, **then** nothing is saved and the row shows the name it
   had before.
8. **Given** the field is open, **when** I confirm the unchanged name, **then**
   nothing is saved.
9. **Given** a task renamed locally, **when** the tracker title of the task
   changes later, **then** the row and the toolbar keep showing the local name.

### US2 - Every row can be renamed (P1)

As a desktop user, I want the pencil on every row, so that I can rename any
kind of task the sidebar shows.

**Acceptance scenarios**

1. **Given** the sidebar lists a tracker task, a free console and a macro run,
   **then** each of their rows has a pencil, and renaming works on each.
2. **Given** a task grouping several executions, **when** I rename it, **then**
   the name applies to the task as a whole, whichever execution is selected.

### US3 - The row no longer carries a "…" menu (P1)

As a desktop user, I want a lighter row, since the other actions are reachable
from the main panel.

**Acceptance scenarios**

1. **Given** any local task row, **then** it has no "…" button, and no action
   of the row opens the former task actions dialog.
2. **Given** a selected task whose execution has finished, **then** Relaunch is
   reachable through the toolbar's Relaunch button, as today.
3. **Given** a selected task with a running execution in the embedded console,
   **then** Detach to native terminal is reachable through the toolbar's
   Detach button, as today.
4. **Given** any local task row, **then** Archive is reachable through the
   row's archive button, as today.

## Functional requirements

- **FR-1** Each local task row of the sidebar loses its "…" button, and the
  task actions dialog it opened is removed together with its Relaunch, Detach
  to native terminal, Archive and Rename locally controls.
- **FR-2** Each local task row, whatever its kind (tracker task, free console,
  macro run), gains a pencil button at the position the "…" button held,
  after the archive button.
- **FR-3** The pencil follows the visibility of the archive button: hidden
  until the row is hovered or has the keyboard focus, always shown on devices
  without hover. Its accessible name and tooltip are `Rename <task name>`.
- **FR-4** Pressing the pencil replaces the row's title by a text field
  holding the current displayed name, focused, with its text selected. One row
  at most is in edition at a time: pressing another row's pencil ends the
  current edition by saving it (the current field loses the focus).
- **FR-5** Enter or the loss of focus saves the trimmed value as the task's
  local name, when it is non-empty and differs from the current name; the row
  then displays it. An empty, blank or unchanged value saves nothing.
- **FR-6** Escape cancels the edition: nothing is saved and the previous name
  is displayed again. The Escape is consumed by the field.
- **FR-7** When the edition ends (saved or canceled), the keyboard focus
  returns to the row's pencil button.
- **FR-8** The rename rules are unchanged: 120 characters at most (the field
  accepts no more), the name is stored on the workstation only, it is never
  sent to the server or the tracker, and it takes precedence over the tracker
  title everywhere the desktop shows the task name.
- **FR-9** While a row is in edition, the periodic refresh of the sidebar
  neither closes the field, nor loses what was typed, nor moves the focus out
  of it.
- **FR-10** Relaunch, Detach to native terminal and Archive keep their current
  entry points in the toolbar (`Relaunch`, `Detach to native terminal`) and on
  the row (archive button), unchanged.
- **FR-11** `CHANGELOG.md` gains one `Changed` line under `[Unreleased]`.

## Edge cases

- A name longer than 120 characters cannot be typed or pasted beyond the
  limit.
- A task with no local name shows the tracker title (or the run label); the
  field opens with that text, and confirming it unchanged saves nothing, so
  the task keeps following the tracker title.
- The task is archived or disappears from the sidebar while its field is open:
  the edition is dropped without saving.
- The row being edited is the selected row: the edition does not change the
  selection, and pressing the pencil of a non-selected row does not select it.
- Clicking inside the field does not select the task or open its console.

## Success criteria

- No local task row has a "…" button (checked by a UI test).
- A task can be renamed with a pencil press, typing and Enter, and the name
  survives a restart and a tracker title change (checked by UI tests).
- Escape and a blank value leave the name unchanged (checked by a UI test).
- Relaunch and Detach are still reachable from the toolbar (checked by the
  existing UI tests, which go through the toolbar).

## Open requirements

None. Every product question was settled in the clarification. FR-4's "one
row at a time" and FR-7's focus return are reversible choices made in this
specification to keep the inline edition keyboard-usable; they follow from
the settled rule that leaving the field saves.
