# #432: Link to the created ticket in the toast

Parent macro: M-7. Clarification: [`docs/clarifications/432.md`](../../docs/clarifications/432.md).

## Problem

When a ticket is created, the success toast prints its key and title and disappears
after 3.5 s. To look at the ticket the user has to find it on the board, which may not
even show it (another project, a saved view, the Roadmap).

## User stories

### US1 (P1): open the created ticket from the toast

- **Given** a ticket is created through quick add, **when** the success toast appears,
  **then** it offers a link naming the ticket's key, and activating it opens the ticket's
  detail view and closes the toast.
- **Given** a story is created from the Roadmap, typed by hand or from a shaping todo
  line, **when** its success toast appears, **then** it offers the same link.

### US2 (P1): reach the tracker page

- **Given** the created ticket has a tracker page (GitHub issue, Jira issue), **when** the
  toast appears, **then** an external-link control next to the in-app link opens that page
  in a new tab.
- **Given** the created ticket is local (no tracker page), **then** only the in-app link
  is offered.

### US3 (P1): time to click

- **Given** a toast carries a link, **then** it stays 8 s instead of 3.5 s.
- **Given** the pointer is over the toast, or focus is inside it, **then** it does not
  dismiss itself; when the pointer leaves and focus leaves, the remaining time runs again.
- **Given** a toast carries no link, **then** its behaviour is unchanged.

## Functional requirements

- **FR1** A toast may carry a link: a label, an action opening a ticket in the app, and
  an optional tracker URL.
- **FR2** The quick add, Roadmap typed story and Roadmap todo story success toasts carry
  a link to the created ticket. Clone and macro toasts do not change.
- **FR3** The todo story endpoint returns the created ticket alongside its key; the
  existing response fields are kept.
- **FR4** A toast with a link defaults to 8 s; an explicit duration still wins.
- **FR5** Auto-dismiss pauses while the toast is hovered or holds focus and resumes with
  the time that was left.
- **FR6** The link labels follow the interface language (French, English).
- **FR7** `CHANGELOG.md` gains an `Added` line under `[Unreleased]`.

## Out of scope

- Toasts for updates, moves, deletions, clones, macros and errors.
- How tickets are created on the tracker.
