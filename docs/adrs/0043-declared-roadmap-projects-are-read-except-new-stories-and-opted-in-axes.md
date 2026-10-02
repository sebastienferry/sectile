# ADR 0043: Declared roadmap projects are read, except new stories and opted-in axes

- Status: Accepted
- Date: 2026-09-30
- Issue: [#632](https://github.com/sebastienferry/sectile/issues/632)

## Context

A Jira project can declare other Jira projects as its roadmap projects. #426
made them strictly read-only: their story keys attach to slicing lines, and
Sectile creates nothing there, writes no parent and sets no label. The roadmap
itself never saw their epics.

#632 brings those epics to the roadmap, and with them two needs the strict rule
refused: creating a slicing line's story in the project where the work will be
done, and writing the order a roadmap decided on another team's epics. Another
team's board is not Sectile's to reorganise, and a declared project can carry
hundreds of epics, so any relaxation had to be narrow and explicit.

## Decision

Sectile never changes an existing item of a declared roadmap project, with one
exception, and may create one kind of new item there:

- **New stories are allowed.** A slicing line may create its story in a declared
  project, under its epic, without any setting. The gesture names one line and
  its target, and it adds work rather than changing someone else's. The story is
  not imported: it enters neither the board, the counters nor the epic's
  progress, which is the rule that no work item of a declared project other than
  an epic is imported.
- **The priority and the quarter can be opted in.** A project setting,
  `roadmap_axis_writes`, closed by default and only open on a Jira project that
  declares at least one project, lets a panel edit write the priority and the
  quarter of one foreign epic. The write goes through the same crossing point
  as an own epic's axes: labels today, the mapped tracker fields once #635 lands.
- **Everything else stays refused**: the horizon, the free labels, the framing
  comment, and every write covering several epics (the title seeding, the
  pending pushes), whatever the setting says.

The crossing point, `macroTracker`, takes the axis being written and enforces
the rule. The queued write checks it again when it runs, so closing the setting
or removing the project from the declaration between a click and its write
refuses the write.

Each declared project's epics are read through a request of their own, so one
project nobody can read does not stop the others.

## Consequences

- A person can create Jira stories in another team's project from Sectile.
  They see them on the slicing line and on Jira, never on Sectile's board.
- Once a project opts in, Sectile's `priority:` and `quarter:` labels appear on
  other teams' epics until #635 maps the axes to shared fields.
- A foreign epic's horizon, priority and quarter can still be set locally for
  the roadmap's own reading; the panel says they stay in Sectile.
- The read-only statement of #426 no longer holds as written; this record
  replaces it.
