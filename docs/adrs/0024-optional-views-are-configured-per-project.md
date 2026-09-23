# ADR 0024: The planning views are optional and configured per project

Status: Accepted

## Context

Sectile carries three planning views beside the board and the backlog: Triage
(work items missing a sprint, a macro, a team or an assignee), Roadmap (macros
laid on the NOW / NEXT / FUTURE horizons) and Timeline (the sprint schedule).

None of them means anything on every project. A project tracking a single
stream of tickets has no macros to place and no sprints to read, so Roadmap and
Timeline open on an empty screen; a project whose tracker fills every field on
creation has nothing to triage. The three views were removed once for that
reason (commit `0d93a04`), and Roadmap and Timeline were then put back (#273)
because other projects depend on them daily. Both decisions were right for the
projects their authors had in mind, and neither could be right for all of them.

A permanent sidebar entry that opens on nothing costs more than it gives: it
takes vertical space from the entries people do use, and it teaches them that
some entries are not worth clicking.

## Decision

The three views are optional, and each project names the ones it shows.

The setting is one column, `projects.enabled_views`, holding a JSON array of
view identifiers, rather than one boolean column per view. A fourth optional
view then costs a value, not a migration, a pair of scan sites and a column in
two SELECT lists.

The list is normalized on every read and every write: unknown identifiers are
dropped, case and spacing are forgiven, and the order is the canonical one of
the sidebar. A view retired between two releases therefore leaves no dead entry
behind, and two projects that enabled the same set store the same value.

The default is an empty list, so every project that existed before this change
shows none of the three. Enabling a view is a deliberate act, taken in the
project settings under General.

A view that is off is absent from the sidebar and from the command palette
alike: a command that opens a hidden view is the same dead end as a sidebar
entry that opens an empty screen. When the active view is not available on the
project being switched to, the interface falls back to the board rather than
render a view with nothing to show.

## Consequences

Upgrading hides Triage, Roadmap and Timeline everywhere until each project asks
for them. That is the intended cost: the alternative, keeping them on for
projects that already have them, would require guessing which projects those
are from data that does not record it.

Selecting "all projects" rather than one shows none of the three, because the
setting belongs to a project and these views read a project's macros and
sprints.

The fallback to the board writes the remembered view, so returning to a project
where a planning view is enabled lands on the board rather than on the view
that was open before. Last view visited wins, which is what the rest of the
interface already does.
