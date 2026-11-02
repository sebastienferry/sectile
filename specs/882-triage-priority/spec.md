# Spec #882 - Triage: add priority

Ticket: https://github.com/sebastienferry/sectile/issues/882
Clarification: `docs/clarifications/882.md` (rounds 1 and 2, settled)
Branch: `feat/882`

## Problem

The Triage view lets the owner set, row by row, the macro, sprint, team and
assignee of a ticket, but it neither shows nor sets the priority. The owner has
to leave the triage for the List view or the task detail to prioritise what
they are sorting.

Worse, on a GitHub or GitLab project a priority set anywhere in Sectile does
not last: every synchronisation writes `medium` back over it, because neither
tracker carries a priority. A priority column in the triage would be undone at
the next sync.

## Scope

In:

- A priority column in the Triage view of the web client, which Sectile
  Desktop inherits through the shared component.
- The synchronisation of GitHub and GitLab tickets keeps the priority stored in
  Sectile.

Out (clarification, round 2):

- A batch priority action in the triage's selection bar.
- A priority filter in the triage's Filters panel, and a chip for the global
  priority filter, which keeps narrowing the triage's tickets as it does today.
- A "missing priority" pill.
- Writing the priority to GitHub or GitLab (no `priority:*` label, no field):
  the priority of those tickets stays local to Sectile.
- The priority levels (`urgent`, `high`, `medium`, `low`), the Jira priority
  mapping (#679), the epic priority of the Roadmap (`p0` to `p3`), and how the
  other views show the priority.

## User stories

### US1 - See and change a ticket's priority in the triage (P1)

- **US1.1** Given the Triage view lists tickets, each row shows the ticket's
  priority as a coloured dot, in a "Priority" column placed right after the
  title column, with the same colours as the List view.
- **US1.2** Hovering the dot names the priority level.
- **US1.3** Given a row, when the owner picks another level from the dot, the
  ticket takes that priority, the dot changes colour, and the change is
  confirmed as any other task update is.
- **US1.4** Given a Jira ticket whose priority the tracker refuses (#679), when
  the owner picks a level, the refusal is shown as an error, as for any other
  failed task update, and the dot keeps the stored priority.
- **US1.5** The empty state of the table still spans every column.

### US2 - A GitHub or GitLab ticket keeps its priority across a sync (P1)

- **US2.1** Given a GitHub ticket whose priority was changed in Sectile, when
  its tracker is synchronised (full sync, project import or single-ticket
  refresh), the ticket keeps the priority set in Sectile.
- **US2.2** The same holds for a GitLab ticket.
- **US2.3** Given a Jira ticket, when its tracker is synchronised, the ticket
  takes the priority Jira gives it, as today, including when that priority is
  Medium.
- **US2.4** Given a GitHub or GitLab ticket imported for the first time, it
  starts at `medium`, as today.
- **US2.5** Every other field a sync writes (title, description, status,
  labels, assignee, and so on) is written as today.

## Functional requirements

- **FR1** The Triage table has a "Priority" column after "Title", with one
  editable priority control per row offering the four levels, named in the
  interface's language.
- **FR2** A pick writes the priority through the existing task update, with its
  existing success and error feedback; no new endpoint.
- **FR3** On update of an existing ticket, the synchronisation overwrites the
  stored priority only when the ticket's tracker carries a priority of its own
  (Jira). The decision is made from the ticket's source, never from the
  incoming value.
- **FR4** The insertion of a new ticket by a synchronisation is unchanged.
- **FR5** No priority is written to GitHub or GitLab.
- **FR6** The new strings exist in French and English.
- **FR7** `CHANGELOG.md` gains, under `## [Unreleased]`, one `Added` line (the
  triage's priority column) and one `Fixed` line (a GitHub or GitLab ticket's
  priority no longer resets to medium at each sync).

## Acceptance criteria

- The Triage view shows a "Priority" column after "Title", one coloured dot per
  row, named on hover.
- Picking a level on a row changes the ticket's priority, visible in the List
  view and the task detail.
- A refused update shows an error and leaves the dot unchanged.
- A GitHub ticket and a GitLab ticket set to `high` in Sectile are still `high`
  after a re-import of the same tickets carrying `medium`.
- A Jira ticket stored as `high` takes the tracker's `medium` after a
  re-import.
- A ticket imported for the first time from GitHub or GitLab is `medium`.
- The column header and the control's title read correctly in French and
  English.
- `CHANGELOG.md` has the `Added` and `Fixed` lines.

## Open points

None. Both product questions were answered in round 2 of the clarification:
the persistence fix is part of this ticket (Q1), and the triage gets the
editable column only (Q2).
