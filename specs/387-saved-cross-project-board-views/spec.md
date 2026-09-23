# Saved cross-project board views filtered by labels

Ticket: [#387](https://github.com/sebastienferry/sectile/issues/387).
Replaces the per-project membership label of #349 / PR #354, both closed.

This file describes behaviour only. Implementation choices live in
[plan.md](plan.md).

## Definitions

- **Saved view** (or *view*): a named, personal selection over the existing
  board. It stores a fixed list of projects and a list of labels, nothing else.
  It is a selection rule, never a copy or a frozen list of tickets.
- **Owner**: the signed-in user who created the view. Only the owner sees,
  opens, edits or deletes it.
- **Board filters**: the filters the board already offers (search, status,
  priority, label, sprint, team, macro, assignee, tracker status, issue type,
  pinned only).

## User stories

### US1: Create and open a saved view (P1)

As a Sectile user who follows work spread over several projects, I want to
save a named selection of projects and labels, so that I can reopen that board
in one click instead of rebuilding its filters.

### US2: Read the view like a board (P1)

As the owner of a view, I want its tickets to behave exactly as they do on the
all-projects board, so that I keep their project, workflow, activities, tracker
identity and actions.

### US3: Reach a view directly (P2)

As the owner of a view, I want a sidebar entry and a stable link for it, so
that I can reach it from anywhere in the web interface and from a bookmark.

### US4: Maintain my views (P2)

As the owner of a view, I want to rename it, change its projects and labels,
and delete it, so that it follows my work as it evolves.

### US5: Create a ticket from a view (P3)

As the owner of a view, I want to create a ticket from it, choosing its source
project, so that the new ticket lands where it belongs and, when unambiguous,
already belongs to the view.

## Functional requirements

### Definition and persistence

- **FR-001** A view has a name, a non-empty list of one or more projects, and a
  list of zero or more labels.
- **FR-002** The name is required after trimming. Two views of the same owner
  may not share a name, compared case-insensitively after trimming.
- **FR-003** The project list is a fixed list chosen at creation or edition. A
  view cannot express "every project", present or future.
- **FR-004** Labels are stored trimmed; empty entries are dropped; entries that
  differ only by case are kept once (the first spelling entered is kept).
- **FR-005** A view is personal. It is stored server-side for its owner, so it
  is available on any browser or device the owner signs in from. It is never
  listed, returned or opened for another user; a request naming another user's
  view behaves as if the view did not exist.
- **FR-006** Views are listed in the order the owner created them.

### Selection semantics

- **FR-010** A view shows the tickets of its selected projects that carry **at
  least one** of its labels (ANY / OR).
- **FR-011** Label matching compares whole labels, case-insensitively: the view
  label `Backend` matches a ticket label `backend`, and does not match
  `backend-api` nor `api-backend`.
- **FR-012** The same labels apply uniformly to every selected project; there is
  no per-project label expression.
- **FR-013** A view with no label shows every ticket of its selected projects.
- **FR-014** The selection is evaluated each time the board is loaded or
  refreshed: tickets whose labels changed, tickets added or removed by a
  synchronisation, appear or disappear accordingly.
- **FR-015** No exclusion, ALL-of matching, nested expression or additional
  stored filter exists in this version.
- **FR-016** The rules the all-projects board already applies to every ticket
  keep applying inside a view (for instance, container issue types stay hidden
  unless the issue type filter asks for them).

### Board presentation

- **FR-020** A view renders with the board, backlog and the other views the
  all-projects board offers today, with the same column and grouping behaviour.
  No cross-project column mapping, custom view column or per-project swimlane is
  introduced. The views built from the ticket list (board, backlog) show the
  view's tickets; the activity, statistics, team and synchronisation screens
  keep the scope they have on the all-projects board.
- **FR-021** Card actions (opening the detail, running skills, moving across
  columns, editing) behave as they do on the all-projects board and act on the
  ticket's own source project.
- **FR-022** When the same remote story has several local records (one per
  Sectile project that synchronises it), each record is shown as its own card.
  Records are never merged or collapsed.
- **FR-023** Every card shown in a view identifies its source project, so that
  two records of one remote story can be told apart at a glance.
- **FR-024** The board header names the open view.

### Board filters inside a view

- **FR-030** Every board filter remains available inside a view and narrows the
  view's selection further (the view's rules and the filters both apply).
- **FR-031** Board filters are remembered per view on the current browser, the
  same way they are remembered per project today. Opening a view restores the
  filters last used in that view; they are not stored with the view on the
  server and do not affect the view's definition.
- **FR-032** Leaving a view for a project, or the reverse, restores the filters
  remembered for the destination, without carrying the previous ones over.

### Navigation and direct access

- **FR-040** The sidebar's views section ("Vues") lists the owner's saved views
  by name, and offers an action to create a new one.
- **FR-041** Selecting a saved view opens its board. Selecting a project, or the
  all-projects entry, leaves the view.
- **FR-042** An open view is reflected in the address as `?view=<id>`. Loading
  the interface with that parameter opens the view directly; the link survives
  reloads and can be bookmarked.
- **FR-043** When the `view` parameter names a view that does not exist or
  belongs to another user, the interface opens the board it would otherwise
  open and states that the view is not available. It reveals nothing about a
  view of another user.

### Maintenance

- **FR-050** The owner can rename a view and change its projects and labels.
  Changes apply immediately to the open view.
- **FR-051** The owner can delete a view after a confirmation. Deleting a view
  deletes no ticket, label or project. If the deleted view was open, the
  interface falls back to the all-projects board.
- **FR-052** When a project is deleted, it is removed from every view that
  selected it. A view left with no project stays listed, shows an empty board
  that says why, and can be edited or deleted.

### Creating a ticket from a view

- **FR-060** Creating a ticket from a view asks for its source project, chosen
  among the view's projects. It is never inferred silently.
- **FR-061** When the view has exactly one label, that label is added to the
  new ticket, so it appears in the view. The user can still remove it before
  submitting.
- **FR-062** When the view has zero or several labels, no label is added
  automatically.

### Out of scope

- Shared, team or public views; permissions on views.
- Cross-project column mapping, custom view columns, swimlanes.
- Collapsing duplicate records of one remote story.
- Label-editing actions offered by the view ("add to this view", "remove from
  this view").
- Desktop-specific entry points beyond what the web interface they embed offers.
- Any change to project membership labels (#354 is closed).
- Scoping the activity, statistics, team and synchronisation screens to a view.

## Acceptance scenarios

1. **Create**: Given projects A and B, when the user creates the view
   "Platform" with A, B and the label `platform`, then "Platform" appears in the
   sidebar views section and opens a board showing the tickets of A and B that
   carry `platform`.
2. **ANY semantics**: Given a view with labels `api` and `ui`, when a ticket
   carries only `ui`, then it is shown; a ticket carrying neither is not.
3. **Whole-label, case-insensitive**: Given a view with the label `Backend`,
   then a ticket labelled `backend` is shown, and tickets labelled only
   `backend-api` or `api-backend` are not.
4. **Empty labels**: Given a view with projects A and B and no label, then
   every ticket of A and B is shown, and no ticket of project C.
5. **Live selection**: Given an open view with the label `platform`, when a
   ticket of a selected project gains `platform` and the board refreshes, then
   the ticket appears; when it loses the label, it disappears on the next
   refresh.
6. **Duplicates**: Given projects A and B both synchronising the remote story
   `#42`, and a view selecting A and B, then two cards for `#42` are shown, each
   identifying its project, and running a skill on one does not act on the
   other.
7. **Columns**: Given a view over two projects with different tracker
   columns, then the board uses the same columns and grouping the all-projects
   board shows today for those tickets.
8. **Filters per view**: Given the view "Platform" with the sprint filter set
   to "S12", when the user opens project A then comes back to "Platform", then
   project A shows its own remembered filters and "Platform" shows "S12" again.
9. **Filters narrow**: Given an open view and the assignee filter set to the
   user, then only the view's tickets assigned to the user are shown.
10. **Direct link**: Given the view "Platform" with id `v1`, when its owner
    loads `/?view=v1` in a new tab, then "Platform" opens directly.
11. **Foreign link**: Given the view `v1` owned by user U, when user W loads
    `/?view=v1`, then W sees their usual board and a message that the view is
    not available, and nothing of `v1` (name, projects, labels) is disclosed.
12. **Edit**: Given the open view "Platform", when the owner removes project B
    and saves, then B's tickets leave the board immediately.
13. **Delete**: Given the open view "Platform", when the owner deletes it and
    confirms, then it leaves the sidebar, the all-projects board opens, and no
    ticket or label changed.
14. **Project deleted**: Given a view selecting only project A, when A is
    deleted, then the view remains listed, opens on an empty board explaining
    that it selects no project, and can be edited.
15. **Create, one label**: Given a view over A and B with the single label
    `platform`, when the user creates a ticket from it, then they must pick A or
    B, the label `platform` is prefilled, and once created the ticket appears in
    the view.
16. **Create, several labels**: Given a view with labels `api` and `ui`, when
    the user creates a ticket from it, then no label is prefilled.
17. **Validation**: Given an existing view "Platform", when the user tries to
    save another view named " platform ", or a view with no project, or with an
    empty name, then saving is refused with a message saying why.
18. **Changelog**: `CHANGELOG.md` gains one line under `[Unreleased]` → `Added`
    describing saved cross-project board views.

## Open points

None blocks implementation. Two notes for the reviewer:

- The full clarification report (`docs/clarifications/387.md`, commits
  `f62113a`, `008c1a2`, `516d41e`) was never pushed and is absent from this
  branch. This specification applies the decisions recorded in the
  clarification comment on the ticket (2026-09-23). If that report carried a
  detail the comment omitted, it should be reconciled here.
- FR-023 shows the project on every card of a view, not only on duplicated
  records: that is the reading of "duplicate local records are shown, badged
  with their project" that keeps one card layout inside a view. Showing the
  badge on duplicates only would be a narrower reading of the same decision.
