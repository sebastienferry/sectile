# ADR 0025: Saved board views are personal overlays on the all-projects board

Status: Accepted

## Context

People follow work that spans several Sectile projects: a platform effort
tagged `platform` in three repositories, a customer's tickets spread over two
Jira projects. Rebuilding that selection by hand, filter by filter, every time
is what #349 asked to avoid.

PR #354 answered with one membership label per project: synchronisation keeps
every ticket, and the board hides those that do not carry the project's label.
It solved a different problem, and it could not solve this one. When several
Sectile projects synchronise the same tracker scope, each keeps its own local
record of a remote story, because a local identity includes its project. Hiding
records by label leaves that duplication in place, and a project still selects
only itself. #349 and #354 were closed in favour of #387 (clarified 2026-09-23).

A shared cross-project board also raises questions a first version cannot
settle: which columns when projects map tracker statuses differently, which
transition a drop requests, which project owns a card that stands for two
records.

## Decision

**A saved view is a personal selection laid over the existing all-projects
board**, and nothing more. It stores a name, a fixed list of projects and a list
of labels, in one table, `board_views`, owned by the account that created it.

- **Selection.** A ticket belongs to a view when it sits in one of its projects
  and carries at least one of its labels, compared whole and regardless of case.
  No label selects every ticket of the projects. There is no ALL-of matching,
  exclusion or nested expression.
- **Resolved by the server.** The task list and its facets take `viewId`, which
  replaces the project scope; every other filter narrows the view further. The
  interface never sends the view's projects or labels itself, so a direct link
  (`?view=<id>`) works from a cold load and ownership is enforced in one place.
- **The label predicate is SQL.** `tasks.labels` is a JSON array written by
  `json.Marshal`, so a whole label is exactly its quoted JSON token:
  `LOWER(labels) LIKE '%"backend"%'`, with LIKE wildcards escaped. The facets
  run a dozen queries over one scope condition; expressing the view as that
  condition keeps their counts consistent with the list without rewriting them.
  An SQL JSON function (`json_each`, `jsonb_array_elements_text`) was rejected
  for needing one variant per engine and a cast of a TEXT column.
- **Presentation is the all-projects board's.** Columns, grouping, drag and drop
  and card actions are exactly what the all-projects board already does. No
  cross-project column mapping or swimlane is introduced. Only what is built
  from the ticket list follows the view: the activity, statistics, team and
  synchronisation screens keep the all-projects scope in this version.
- **Duplicates are shown, not merged.** Each local record is its own card, and
  every card in a view names its project. Collapsing records would require
  choosing which project owns actions and activities, and would hide the
  duplication rather than resolve it.
- **Personal only.** Another account's view answers exactly like a missing one.
  Sharing, and the permission model it would need, is out of scope.
- **Filters are remembered per view**, in the browser, beside the per-project
  memory that already exists. They are not part of the view's definition.

## Consequences

A view never owns data: deleting one changes no ticket, label or project, and
deleting a project removes it from the views that selected it, which remain and
select nothing until edited.

A label always matches its own spelling. Case is ignored for every letter
under PostgreSQL, and for ASCII letters only under SQLite, whose `LOWER` leaves
the others alone: the view label is lowered the same way as the column on each
engine (`labelFold`), so `Équipe` finds `Équipe` everywhere, and `équipe` finds
it under PostgreSQL only.

A remote story synchronised by two projects of a view shows twice. That is the
honest picture of the board; a fix belongs to how projects share a tracker
scope, not to the view.

Creating a ticket from a view asks for its project among the view's, and
prefills the label only for a single-label view, the one case where the label
that makes the ticket appear in the view is unambiguous.
