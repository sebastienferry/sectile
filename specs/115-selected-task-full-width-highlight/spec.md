# #115 — Selected task marker full-width highlight

## Context

In the desktop sidebar, a local task row (`.local-task`) is composed of the task ID link
(`.task-number`), the execution button (`.run`, holding the description and the status
indicator) and trailing controls (PR indicator, archive, menu). Today the selection
background is carried by the inner `.run` button only, so the highlight stops short of the
task ID on the left and of the trailing controls on the right.

## User stories

### US1 — See at a glance which task is selected (P1)

As a desktop user browsing the project sidebar, I want the selected task to be highlighted
across the entire column width, so that the selection reads as one row instead of a partial
band inside it.

- **Given** a project group with several local tasks
- **When** I select the execution of one of them
- **Then** the highlight background covers the full row width, enclosing the task ID link,
  the description and the status indicator.

### US2 — Keep selection exclusive (P2)

As a desktop user, I want only the selected row highlighted, so that selecting another task
clears the previous highlight.

- **Given** task A is selected
- **When** I select task B
- **Then** row A loses the highlight and row B carries it.

### US3 — Keep every control usable (P2)

As a desktop user, I want the row controls to keep behaving as before, so that the visual
change costs no functionality.

- **Given** a selected task row
- **When** I click the task ID link, the archive button or the actions menu
- **Then** the same action as before is triggered, and the selection is unchanged by the
  task ID click.

## Functional requirements

- FR1: The row container `.local-task` carries the `selected` class when any execution of
  that task is the selected execution.
- FR2: The selection background (`#203033`) is painted by `.local-task.selected` over the
  full row width, with a `8px` border radius.
- FR3: The inner `.run` button keeps its `selected` class for locator compatibility, and
  renders with a transparent background so a single background is visible.
- FR4: Free-console rows (no task ID) follow the same rule.
- FR5: Click handlers of `.task-number`, `.run`, `.pr-indicator`, `.task-archive` and
  `.task-menu` are unchanged.

## Out of scope

Backend execution orchestration, the remote web interface, the execution queue rendering,
and any change to hover or focus styling.
