# Specification #628 - Roadmap: group and reorder epics by priority or quarter

- Ticket: https://github.com/sebastienferry/sectile/issues/628 (milestone M-11,
  Roadmap)
- Branch: `feat/628`
- Clarification: `docs/clarifications/628.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

A roadmap horizon tab can be split into sections by the epic's own priority or
by its quarter (both from #627). Each section is named after its value and
shows its count; a "no priority" or "no quarter" section comes last. Every
section folds, and its folded state is remembered. Dragging one epic, or a
selection built with Ctrl or Cmd click and Shift click, onto a section sets
that value on each epic through the same save as the panel; the epics already
at that value are skipped, and the epics whose save fails are named in one
report.

## Scope

In scope: the roadmap view of the web app (toolbar, list, rows, drag and drop,
selection), its per-browser preferences, the French and English strings, and
the changelog.

Out of scope:

- The readiness axis (#633, Part B), which adds itself as a third axis once
  this ticket is merged.
- Changing the horizon by dragging: the horizon keeps its own controls.
- Reordering epics by hand inside a section.
- Per-project axis prefixes and tracker custom fields (#635).
- A batch write in one server operation, and any server or API change.
- The Hidden tab, which stays a flat list.

## Dependencies

- **#627** (epic priority and quarter), merged: `saveAxes` / `saveMacroMeta`,
  its Jira label write, its local-only storage on GitHub milestones, GitLab,
  local projects and other projects' epics, and its refusal message.
- **#626** (epic labels), merged: axis labels already stay out of badges and
  the label filter.
- **#629** (remember the view), merged: `roadmapViewPrefs.ts` stores the new
  preferences the same way.
- **#631** (timeline selection), open: the range selection helper written here
  is meant to be shared with it.

## Vocabulary

- **Axis**: what the sections are built on: `none` (no grouping, today's flat
  list), `priority` or `quarter`.
- **Section**: the epics of the tab that share one value of the axis, or that
  have none ("no value" section).
- **Groupable tab**: NOW, NEXT, LATER and Unclassified. Hidden is not.
- **Selection**: the epics picked with Ctrl, Cmd or Shift click, carried
  together by a drag.

## User stories

### US1 - Group the tab by priority (P1)

As a person reviewing the roadmap, I choose "Priority" in the grouping control
and the tab shows one section per priority, so that I see at a glance what is
P0 and what has no priority yet.

- **AC1.1** Given the grouping control on "Priority", when NOW is shown, then
  sections P0, P1, P2, P3 and "No priority" appear in that order, each with its
  count, a count of 0 included.
- **AC1.2** Given the priority filter, the search, the closed toggle, the label
  filter or the "only issues" filter, when sections are built, then they only
  hold the epics the flat list would show, and the tab counts do not change.
- **AC1.3** Given the priority sort of #627, when grouped, then the sort applies
  inside each section (backlog order by default).

### US2 - Group the tab by quarter (P1)

As a person planning quarters, I choose "Quarter" and the tab shows one section
per quarter in chronological order, with the coming quarters present even when
empty, so that I can drop epics on a quarter nobody used yet.

- **AC2.1** Given today is 2026-09-30 and epics of the tab carrying 2026-Q1 and
  2027-Q3, when grouped by quarter, then the sections are 2026-Q1, 2026-Q3,
  2026-Q4, 2027-Q1, 2027-Q2, 2027-Q3 and "No quarter", in that order.
- **AC2.2** A past quarter shows only while an epic of the tab carries it; the
  current quarter and the next three always show; "No quarter" always shows.
- **AC2.3** The current quarter is computed from the browser's local date.

### US3 - Fold sections and keep the view (P2)

As a person coming back to the roadmap, I find the axis and the folded sections
as I left them.

- **AC3.1** Clicking a section header folds or unfolds it; a folded section
  still shows its header and count.
- **AC3.2** The axis and the folded sections survive a change of view and a
  reload, per browser. The folded state is kept per axis and value
  (`priority:p1`, `quarter:2026-Q4`, `priority:none`) and shared by the tabs.
- **AC3.3** A missing, refused or unknown stored value falls back to no
  grouping and nothing folded.

### US4 - Drag one epic onto a section (P1)

As a person reprioritizing, I drag an epic onto another section and it takes
that section's value.

- **AC4.1** Given the priority axis, when an epic is dropped on P1, then its
  priority becomes P1 through the panel's save, and it moves to P1 after the
  reload; its horizon, title and other axes are unchanged.
- **AC4.2** Dropping on "No priority" / "No quarter" clears the value.
- **AC4.3** Dropping on the section the epic comes from saves nothing and shows
  nothing.
- **AC4.4** A folded section, or an empty one, accepts a drop on its header.
- **AC4.5** Drag is only offered while an axis is chosen, in full and condensed
  rows alike, and never on the Hidden tab.

### US5 - Select several epics and drag them together (P1)

As a person reorganizing a quarter, I pick several epics and drop them at once.

- **AC5.1** Ctrl or Cmd click toggles an epic in the selection without opening
  it; a plain click opens the panel as today and keeps the selection.
- **AC5.2** Shift click selects the range from the last toggled epic to the
  clicked one, in the displayed order (sections in order, folded sections
  skipped), added to the selection.
- **AC5.3** Dragging a selected epic carries the whole selection; dragging an
  unselected one carries only it and keeps the selection.
- **AC5.4** A counter shows the selection size with a control that clears it;
  Escape clears it too when nothing else takes the key.
- **AC5.5** Epics no longer shown (tab change, filter, folding, axis back to
  none) leave the selection.
- **AC5.6** On the Hidden tab, and while no axis is chosen, a Ctrl, Cmd or
  Shift click behaves as a plain click.

### US6 - One report for a drop (P1)

- **AC6.1** A drop saves the epics one at a time, skips those already at the
  target value, never stops on a failure, reloads the macros once, and shows
  one toast: how many were set, how many skipped, and the keys whose save
  failed.
- **AC6.2** The epics that moved leave the selection; those that failed stay
  selected so the drop can be retried.
- **AC6.3** Tracker refusals arrive later in the activities, as for any queued
  write; nothing new is shown for them here.

## Functional requirements

- **FR1** A grouping control in the toolbar, next to the priority sort, with
  "No grouping", "Priority", "Quarter". Default "No grouping".
- **FR2** The control applies to every groupable tab at once and keeps its
  value on the Hidden tab, where the list stays flat.
- **FR3** Priority sections: P0, P1, P2, P3, then "No priority", all always
  shown.
- **FR4** Quarter sections: the union of the quarters used by the tab's shown
  epics and the current quarter plus the next three, chronological, then "No
  quarter", always shown.
- **FR5** Section headers show the value and the count, fold on click, and are
  drop targets whatever their state.
- **FR6** Axis and folded sections persisted per browser in
  `roadmapViewPrefs.ts`, tolerant of missing or foreign values.
- **FR7** Drag carries epic keys only; a drop writes the axis shown and nothing
  else, through `saveMacroMeta` with `quiet`, one epic at a time. A drop that
  saves several epics is a bulk edit (`bulk`, #632), as the seeding is: on
  another team's epic the value stays in Sectile. A drop that saves one epic is
  a single edit, as from the panel.
- **FR8** Selection rules of US5, pruned against the epics shown.
- **FR9** One report per drop (US6).
- **FR10** Every new string in French and English; one `Added` line in
  `CHANGELOG.md` under `[Unreleased]`.

## Edge cases

- An epic whose stored quarter does not read as `YYYY-Qn` goes to "No quarter"
  (the server normalizes, so this is defensive).
- The selected epic of the panel may be in a folded section: the panel keeps
  showing it.
- A drop of an epic from another project (a foreign epic shown on the roadmap)
  saves locally, as the panel does.
- An empty tab with an axis chosen still shows its always-present sections, so
  that a drop target exists; the empty-state message is kept for the flat list
  only.
- A drag started from a text selection or a button inside the row carries
  nothing (the row is the drag source).
