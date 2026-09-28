# Specification #581 - Board filters are respected and kept

- Ticket: https://github.com/sebastienferry/sectile/issues/581
- Branch: `feat/581`
- Clarification: `docs/clarifications/581.md` (rounds 1 and 2, confirmed by
  the owner on 2026-09-28)
- Framework: Spec Kit

## Summary

On the web board, a selected Sprint, Team or Assignee filter sometimes does
not hold: the board shows tickets the filter should hide while the selector
still shows the filter, or the selector itself is emptied after a reload or a
switch of project or view, and stays empty. The board must always show exactly
what the filters on screen select, and a remembered filter must come back with
its project or view.

## Scope

In scope:

- Every board filter, for the guarantee that the board matches the filters on
  screen: search, status, priority, label, sprint, team, macro, assignee, My
  Tasks, tracker statuses, issue types, pinned.
- Sprint, Team and Assignee, for the guarantee that a remembered filter is not
  forgotten on a project or view switch (they are the only filters that are
  dropped when their value leaves the board).
- Every view that reads the shared task list (board, list, backlog and the
  other views fed by the same list), on the web client and on the desktop app,
  which embeds the same interface.

Out of scope:

- New filters, the filter bar layout, the saved board views feature (#387),
  the card sort (#402).
- The server: it already applies every filter it receives.

## User stories

### US1 - The board matches the filters on screen (P1)

As someone filtering the board, I see only the tickets my filters select,
however quickly I change them and whatever refreshes in the background.

Acceptance:

1. **Given** a project with a remembered Sprint filter, **when** the board
   opens and the unfiltered list answers after the filtered one, **then** the
   board shows only the tickets of that sprint.
2. **Given** a Team filter set, **when** I pick an Assignee right after and the
   answer for the Team-only query arrives last, **then** the board shows the
   tickets of that team and that person only.
3. **Given** a filtered board, **when** a background refresh (a ticket update
   pushed by the server, a finished skill run) starts, then I change a filter,
   and the refresh answers last, **then** the board shows the tickets of the
   filters now on screen.
4. **Given** I switch from project A to project B, **when** the answer for
   project A arrives after the one for project B, **then** the board shows
   project B's tickets with project B's filters.
5. The loading indicator and the error banner reflect the latest request only:
   an older request that fails or finishes does not hide the loading of the
   current one, nor raise an error for a board no longer on screen.

### US2 - A remembered filter survives a switch (P1)

As someone who filters each project differently, I find each project's (and
each view's) Sprint, Team and Assignee filters again when I come back to it,
including after a reload.

Acceptance:

1. **Given** project A remembers Sprint "S12" and project B carries no sprint
   "S12", **when** I switch from B to A, **then** A's Sprint selector shows
   "S12" and the board is filtered on it.
2. **Given** a saved view remembers Team "Core", **when** I open the view from
   a project that has no team "Core", **then** the view's Team selector shows
   "Core".
3. **Given** the switches of acceptance 1 and 2, **when** I reload the page,
   **then** the filters are still remembered: nothing was stored as cleared
   during the switch.
4. **Given** project A remembers Sprint "S11" and A's board no longer carries
   sprint "S11" (closed sprint), **when** A's own values are known, **then**
   the Sprint filter is dropped and no longer remembered, as today. The same
   holds for a Team or an Assignee no longer on the board.
5. The "Unassigned" Assignee value is never dropped this way, as today.

## Functional requirements

- **FR1** The shared task list is only ever replaced by the answer to the most
  recent task list request. An answer to an earlier request, whatever started
  it (opening the board, a filter change, a project or view switch, a pushed
  ticket update, the refresh after a skill run, a bookmark toggle, a view
  edit), is ignored.
- **FR2** The loading state, the read-failure reporting and the error message
  of the task list follow the most recent request only (US1-5).
- **FR3** The board's filter values (the lists the Sprint, Team, Assignee and
  other selectors offer) are only ever replaced by the answer to the most
  recent request for them.
- **FR4** Dropping a remembered Sprint, Team or Assignee filter because its
  value is no longer on the board is decided only against the values of the
  project or view the filter belongs to, never against those of the project
  or view shown before.
- **FR5** A filter that FR4 does not drop is neither cleared on screen nor
  stored as cleared.
- **FR6** A filter whose value really left its own project or view is dropped
  and forgotten, as today (US2-4). "Unassigned" is never dropped (US2-5).
- **FR7** Local updates to the list (a ticket created, edited, moved or
  deleted from the interface) keep applying immediately, as today.
- **FR8** `CHANGELOG.md` carries one `Fixed` line under `## [Unreleased]`
  saying the board again respects its Sprint, Team and Assignee filters and
  keeps them when switching projects or views.

## Success criteria

- The acceptance scenarios of US1 and US2 pass in an automated test with
  answers delivered out of order.
- No change to the requests sent to the server: same parameters, same
  endpoints.

## Assumptions

- The desktop app shows the web interface; fixing it there covers the desktop
  app without a change of its own. The owner observed the defect on the web.
- A background refresh started for an older set of filters has no value once
  the filters changed; ignoring its answer is not a loss, the newer request
  brings the same data for the current filters.

## Open points

- None. Every product decision was settled during clarification.
