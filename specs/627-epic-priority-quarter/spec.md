# Specification #627 - Roadmap: give each epic its own priority and quarter

- Ticket: https://github.com/sebastienferry/sectile/issues/627 (milestone M-11,
  Roadmap)
- Branch: `claude/clarify-issue-gh-11a4f59c-32990c`
- Clarification: `docs/clarifications/627.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

An epic on the roadmap gets a priority (P0 to P3) and a quarter (for example
2026-Q4) that belong to the epic itself. The user sets and clears both from
the epic's panel. Sectile stores them, and on a Jira project also writes them
on the epic as labels. The roadmap priority column shows the epic's own
priority and nothing else, and the roadmap can filter and sort the epics on
it. A seeding action reads both values from the epic titles, previews what it
would set, and writes only the lines the user keeps.

## Scope

In scope: the roadmap view of the web app (rows, panel, toolbar), the macro
storage and API, the tracker label writes and read-back, the French and
English strings, and the changelog.

Out of scope:

- Grouping the roadmap into sections and dragging epics onto them (#628).
- Naming the axis prefixes per project or mapping them to tracker custom
  fields (#635).
- Showing, filtering or editing the other labels of an epic (#626).
- Filtering or sorting on the quarter.
- Rewriting epic titles, ever.
- Sectile Desktop screens other than the web roadmap it embeds.

## Vocabulary

- **Epic priority**: one of P0, P1, P2, P3, or none. P0 is the highest.
- **Quarter**: a year and a quarter number, written `YYYY-Qn` (`2026-Q4`), or
  none.
- **Labelled tracker**: a project whose tracker can carry labels on its epics.
  Today that is Jira. GitHub milestones, GitLab and local projects are not
  labelled trackers.
- **Priority label**: `priority:p0` to `priority:p3`. **Quarter label**:
  `quarter:2026-q4` (prefixed form) or `2026-Q4` (bare form, read only).

## User stories

### US1 (P1) - The roadmap shows the epic's own priority

As a person who runs the roadmap review, I read which epic comes first from
the epic itself, not from its tickets.

1. Given an epic with priority P1 whose tickets are all "low", then its row
   shows P1.
2. Given an epic with no priority, whatever its tickets' priorities, then its
   row shows "no priority" and never a value deduced from its tickets.
3. Given an epic with no ticket and no priority, then its row shows "no
   priority".
4. Given P0, P1, P2 and P3, then each badge uses the colour the roadmap
   already uses for urgent, high, medium and low respectively.

### US2 (P1) - Set and clear an epic's priority from its panel

As a roadmap owner, I decide an epic's priority where I read it.

1. Given an epic selected, when I pick P2 in its panel, then the row and the
   panel show P2 at once, and on a labelled tracker the epic receives
   `priority:p2` and loses any other `priority:` label.
2. Given an epic with a priority, when I clear it, then the row shows "no
   priority", and on a labelled tracker every `priority:` label is removed
   from the epic.
3. Given a project that is not a labelled tracker, or an epic that is a
   milestone, a local key or an epic of another project, when I set a
   priority, then it is stored and shown, the panel says it is kept in
   Sectile and not written to the tracker, and no tracker write is attempted.
4. Given a labelled tracker that refuses the write, then the value stays set
   in Sectile, and the queued activity says the priority is set in Sectile
   but not on the ticket and names the tracker error.

### US3 (P1) - Set and clear an epic's quarter from its panel

As a roadmap owner, I say in which quarter an epic lands.

1. Given an epic selected, when I type `2026-Q4`, `2026.Q4`, `2026 Q4` or
   `2026-q4` in the quarter field and confirm, then the quarter is stored and
   shown as `2026-Q4`.
2. Given the quarter field, when I type anything else (`Q4`, `2026-Q5`,
   `bientôt`), then an inline message says the value is not a quarter, and
   nothing is stored or written.
3. Given an epic with a quarter, when I empty the field and confirm, or use
   its clear control, then the quarter is removed.
4. Given a labelled tracker, when a quarter is set, then the epic receives
   `quarter:2026-q4`, and loses every other quarter label in both forms
   (`quarter:*` and bare `YYYY-Qn`). When the quarter is cleared, every
   quarter label in both forms is removed.
5. Given a project that is not a labelled tracker, then US2.3 and US2.4 apply
   to the quarter in the same way.

### US4 (P1) - Seed both values from the epic titles

As a roadmap owner whose epic titles already carry `[P2]` or `2026.Q4`, I turn
them into values without retyping them.

1. Given the roadmap toolbar, when I open the seeding action, then a preview
   lists, per epic, its key, its title, and the priority and the quarter found
   in its title, and nothing is written.
2. Given an epic that already has a priority, then the preview proposes no
   priority for it, even when its title carries another one. The same holds
   for the quarter.
3. Given an epic whose title carries neither value for an axis it lacks, then
   it proposes nothing for that axis; an epic with nothing to propose is not
   listed.
4. Given the preview, then every proposed value is ticked by default, and I
   can untick any of them.
5. Given the preview, when I confirm, then only the ticked values are set,
   exactly as if I had set each one from the panel (US2, US3), and the result
   names every failure with its epic key.
6. Given the preview, when I cancel, then nothing is set or written.
7. Given the seeding, then no epic title is modified.
8. Given nothing to propose, then the preview says so and offers no confirm.

### US5 (P2) - Filter the roadmap on the epic priority

1. Given the toolbar, when I choose P1 in the priority filter, then every
   horizon tab lists only the epics whose priority is P1, and the tab counts
   follow.
2. Given the filter, when I choose "no priority", then only epics without a
   priority are listed.
3. Given an active priority filter, then a chip names it among the active
   filter chips, and clearing the chip removes the filter.

### US6 (P2) - Sort the epics on the priority

1. Given the toolbar, when I choose "priority, highest first", then each tab
   lists P0, then P1, P2, P3, then the epics without a priority.
2. When I choose "priority, lowest first", then each tab lists P3, P2, P1,
   P0, then the epics without a priority.
3. Given two epics of the same priority, then they keep their backlog order.
4. When I choose "backlog order", then the order is the one the roadmap uses
   today. It is the default each time the view opens.

### US7 (P2) - Values set on the tracker are read back

1. Given a Jira epic that carries `priority:p1` set on the tracker, when the
   epic labels are read (the sync or "import labels"), then Sectile shows P1,
   replacing a different local value.
2. Given a Jira epic carrying a bare `2026-Q3` and no `quarter:` label, then
   its quarter is 2026-Q3, and Sectile does not rewrite or remove that label
   until someone sets the quarter.
3. Given a Jira epic carrying `2026-Q3` and `quarter:2026-q4`, then its quarter
   is 2026-Q4, and neither label is touched by the read.
4. Given a Jira epic carrying no priority label but a local priority, then the
   local value is kept and counted among the pending pushes; pushing them
   writes it.
5. Given a label `priority:p7` or `quarter:soon`, then it is ignored as if
   absent. Case is ignored on read (`Priority:P1` reads as P1).

## Functional requirements

- **FR1** An epic stores an optional priority among P0 to P3 and an optional
  quarter `YYYY-Qn` (n from 1 to 4, any four-digit year).
- **FR2** The roadmap row and panel priority is the epic's stored priority.
  The value derived from the child tickets is no longer computed or shown.
- **FR3** The panel sets and clears both values; the change is visible at once
  and stored before any tracker write.
- **FR4** The quarter field accepts `YYYY-Qn`, `YYYY.Qn`, `YYYY Qn`, case
  ignored, normalizes to `YYYY-Qn`, and refuses any other value inline before
  any request is sent. An empty value clears the quarter.
- **FR5** On a labelled tracker, each change is written on the epic through a
  queued activity, never inside the click: the priority axis is exclusive
  (one `priority:` label at most), the quarter axis is exclusive across both
  forms. No other label, field, title or status of the epic is written.
- **FR6** Milestones, local keys, epics of another project and every epic of
  a project that is not a labelled tracker keep their values in Sectile only,
  are never pushed, never listed as pending, and the panel says so.
- **FR7** A refused tracker write leaves the local value in place and is
  reported in the activity, naming the epic and the tracker error.
- **FR8** Reading the epic labels applies FR1 values from the tracker when the
  epic carries a valid label (prefixed quarter first, then bare), and keeps
  the local value otherwise. An unreadable value is ignored.
- **FR9** The pending pushes cover the horizon, the priority and the quarter:
  an epic is pending when any of the three differs from its labels, and the
  push writes every axis that differs.
- **FR10** The seeding preview reads the priority from `[P0]` to `[P3]` or a
  bare `P0` to `P3` on word boundaries, and the quarter from `YYYY.Qn`,
  `YYYY-Qn` or `YYYY Qn` on word boundaries, case ignored; when a title holds
  several candidates for one axis, the first one wins. It proposes a value
  only for an axis the epic has no value on, and writes nothing.
- **FR11** Confirming the seeding applies the ticked values through the same
  path as the panel (FR3, FR5, FR6) and reports each failure by epic key.
- **FR12** The toolbar offers a priority filter (P0, P1, P2, P3, no priority)
  shown as an active filter chip, and a sort (backlog order, priority highest
  first, priority lowest first), epics without a priority last in both
  priority orders and ties in backlog order. The sort is not remembered.
- **FR13** Every new string exists in French and English.
- **FR14** `CHANGELOG.md` gets one `Added` line under `[Unreleased]` for the
  epic priority and quarter, and one `Changed` line saying the roadmap
  priority column no longer comes from the child tickets.

## Acceptance criteria

- **AC1** US1 to US7 scenarios pass, on a Jira project and on a GitHub
  project.
- **AC2** After upgrading, every epic shows "no priority" and no quarter
  until one is set, imported or seeded; no existing macro data is lost.
- **AC3** A priority or quarter change on a Jira epic produces exactly one
  queued tracker activity carrying only label changes.
- **AC4** No epic title changes in any scenario.
- **AC5** The web unit tests, the Go tests (SQLite and PostgreSQL) and the
  typecheck pass.

## Edge cases

- An epic closed on the tracker keeps its values and can still be changed.
- Two tabs changing the same epic: last write wins, as for the horizon today.
- `2026-Q4` inside a word (`X2026-Q4`) or `P1` inside a key (`PEP1`) is not
  read by the seeding.
- The title `2026.Q4 [P2] - P0 migration` yields P2 and 2026-Q4: the first
  candidate of each axis wins.
- A priority filter combined with the other filters narrows by all of them.

## Open points

None. Every product question of the clarification is settled.
