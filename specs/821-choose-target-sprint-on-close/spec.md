# Spec #821 - Choose the target sprint when closing a sprint

Ticket: https://github.com/sebastienferry/sectile/issues/821
Clarification: `docs/clarifications/821.md` (rounds 1 and 2, settled)
Branch: `feat/821`

## Problem

Closing a sprint from the Timeline offers three destinations for its
unfinished work: the next sprint, the backlog, or the closed sprint itself.
The user cannot send the work to any other open sprint. On a Jira project the
dialog can also name one sprint as "next" while the server moves the work into
another, because the dialog reads the next sprint by list order and the server
by start date.

## Scope

In: the close dialog of the Timeline, on a tracker-owned (Jira) project and on
a local board; the sprint close API; the automatic start of the target; the
changelog line.

Out: creating a new sprint from the close dialog, reopening a sprint, and a
close action for projects whose sprints are read-only (GitHub).

## User stories

### US1 - Pick the target sprint (P1)

- **US1.1** Given a sprint with unfinished work and at least one other sprint
  that is not closed, when I open its close dialog, then the first option is
  **Move to a sprint (recommended)** with a list of every sprint of the
  project that is not closed and is not the closing one, `future` and
  `active` alike, ordered by start date (undated sprints last, in board
  order).
- **US1.2** The list preselects the sprint the server picks for "next": the
  earliest sprint not closed starting at or after the closing one, or, when
  no dated sprint follows, the first undated `future` sprint. When there is no
  such sprint, the first sprint of the list is preselected.
- **US1.3** When I pick another sprint and confirm, then every unfinished work
  item of the closing sprint lands in the chosen sprint, on the tracker and on
  the board, and the toast names the chosen sprint.
- **US1.4** Given no sprint can be selected, then the option is not shown and
  **Backlog** is selected, as today.
- **US1.5** The **Backlog** and **Keep in this closed sprint** options are
  unchanged.

### US2 - Start the chosen sprint (P1)

- **US2.1** Given the destination is a sprint and that sprint is `future`,
  then the dialog offers **Start {target} automatically**, checked; confirming
  with it checked starts that sprint after the close.
- **US2.2** Given the chosen sprint is already `active`, then the checkbox is
  not shown and nothing else is started.
- **US2.3** Given the destination is **Backlog** or **Keep**, then the
  checkbox applies to the next sprint by start date (the US1.2 rule), shown
  only when that sprint is `future`, as today.

### US3 - A wrong target moves nothing (P1)

- **US3.1** Given a close request whose target sprint is unknown to the
  project, closed, or the closing sprint itself, then the server refuses it
  before any work item moves, and the sprint stays open.
- **US3.2** The existing `next` and `backlog` destinations keep working.

## Functional requirements

- **FR1** The sprint close patch accepts a third destination, `sprint`, with
  the target's sprint id in a separate field. `next` and `backlog` are still
  accepted; any other destination is refused.
- **FR2** A `sprint` destination without an id, or with an id that is unknown,
  closed, or the closing sprint's, is refused before the tracker is called.
- **FR3** The web client computes "next" with the same rule as the server,
  including the undated `future` fallback.
- **FR4** On a tracker-owned project the dialog sends the chosen sprint's id;
  on a local board it moves the work into the chosen sprint by name, as
  moving tasks between sprints does today.
- **FR5** The automatic start targets the chosen sprint when the destination
  is a sprint, the next sprint by start date otherwise.
- **FR6** `CHANGELOG.md` gains one `Added` line under `## [Unreleased]`.

## Acceptance criteria

- On a Jira project and on a local board, closing a sprint with unfinished
  work lets the user pick any non-closed sprint other than the closing one,
  and the work lands there.
- The sprint preselected in the dialog is the sprint the server would pick
  for `next`.
- A closed, unknown or closing-sprint target is refused before any work item
  moves, and the sprint stays open.
- The automatic start applies to the chosen target when it is `future`.
- `CHANGELOG.md` has the `Added` line.
