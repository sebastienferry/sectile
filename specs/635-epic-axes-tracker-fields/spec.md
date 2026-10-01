# Specification #635 - Map the epic axes to tracker fields

- Ticket: https://github.com/sebastienferry/sectile/issues/635 (milestone M-11,
  Roadmap)
- Branch: `feat/635`
- Clarification: `docs/clarifications/635.md` (rounds 1 and 2, every
  recommendation accepted by the owner)
- Framework: Spec Kit
- Study: rows 36 to 38 of `docs/taskativ-roadmap-comparison.md` (rank P10)

## Summary

A project names its own label prefixes for the epic priority, quarter and
readiness axes. On Jira, the project stores how each option of the tracker's
ticket priority scheme maps to Sectile's four levels, shows that table, lets a
person correct it, and refuses a priority write that the table only guessed.
Last, and only after the owner confirms it, a Jira project can map the epic
priority and quarter to custom fields, flat or cascading, written beside the
label and read back before it.

The work ships as three pull requests, in this order:

- **Part A** - configurable axis label prefixes (row 36);
- **Part B** - the stored ticket priority mapping (row 38);
- **Part C** - the axis custom fields (row 37). Part C starts only once the
  owner has confirmed it after A and B are merged, and may be dropped then
  without undoing A or B.

## Scope

In scope: the project settings and their storage, the epic import and the
pending pushes, the axis label writes, the protection of axis labels from the
free label editor, the Jira priority write path (creation and update), the
Tracker tab of the project settings, the French and English strings, and the
changelog.

Out of scope:

- The values of the axes: still `p0` to `p3`, `YYYY-qN`, `idea`, `shaping`,
  `ready`.
- The `roadmap:` horizon prefix, which stays fixed.
- A custom field for the readiness axis.
- Writing one axis to two fields at once.
- The priority of GitHub and GitLab tickets.
- Migrating labels already written under an old prefix.
- How a ticket priority read from the tracker lands on a level: reading keeps
  falling back on the rank, a ticket must land on some level.

## Vocabulary

- **Axis**: one of the epic's own decisions carried as a label, here the
  priority (`p0` to `p3`), the quarter (`YYYY-Qn`) and the readiness (`idea`,
  `shaping`, `ready`). The horizon is an axis too, but its prefix is not
  configurable.
- **Axis prefix**: the text, separator included, that precedes an axis value
  in a label: `priority:` in `priority:p1`, `prio-` in `prio-p1`.
- **Default prefix**: `priority:`, `quarter:` and `readiness:`, used when the
  project names none.
- **Bare quarter**: a quarter label with no prefix, `2026-Q3`, which the import
  already reads as the epic's quarter.
- **Priority scheme**: the options a Jira project offers in its ticket
  `priority` field, most urgent first.
- **Priority mapping**: per project, each option of the scheme with a Sectile
  level (urgent, high, medium, low) and a confidence.
- **Sure line**: a mapping line whose level comes from Sectile's priority name
  aliases, or that a person set or confirmed by hand.
- **Guessed line**: a mapping line whose level comes only from the option's
  rank in the scheme.
- **Preferred option**: when several options share a level, the one a write
  sends; the most urgent of them unless a person picks another.
- **Axis field** (part C): a Jira custom field of the project's epics, a single
  select or a cascading select, mapped to the epic priority or the epic
  quarter.
- **Option map** (part C): for one axis field, each axis value with the option
  id it is written as (`parent/child` for a cascading field).

## User stories

### Part A - Axis label prefixes

#### US1 (P1) - A project names its axis prefixes

As a project administrator whose team already labels its epics `prio-p1`, I
want Sectile to read and write that convention instead of `priority:p1`.

1. Given a project with no prefix set, then the priority, quarter and
   readiness labels are read and written under `priority:`, `quarter:` and
   `readiness:`, exactly as today.
2. Given the Tracker tab of a project whose tracker can carry epic labels, then
   a "Roadmap" section beside the roadmap projects shows one field per axis
   (priority, quarter, readiness), each showing its default prefix as a
   placeholder.
3. Given a project whose tracker cannot carry epic labels (GitHub milestones,
   local), then that section is not shown.
4. Given a person types `Prio-` for the priority, when they save, then the
   prefix is stored as `prio-` (trimmed, lower-cased, a leading `#` removed).
5. Given a prefix containing a space, when the person saves, then the save is
   refused with a message naming the prefix, and nothing is stored.
6. Given a prefix that is empty once cleaned but was not empty as typed (`#`,
   `  #  `), then the save is refused with a message naming the axis.
7. Given two axes whose prefixes start one another (`p` for the priority and
   `priority:` for the quarter, or the same prefix twice), then the save is
   refused with a message naming both axes.
8. Given a prefix that starts `roadmap:` or that `roadmap:` starts, then the
   save is refused: the horizon prefix is not available.
9. Given a field cleared, then the axis returns to its default prefix.

#### US2 (P1) - The roadmap reads the epics under the project's prefixes

1. Given a priority prefix `prio-` and an epic labelled `prio-p1`, when the
   epics are imported, then the epic's priority is P1.
2. Given the same project and an epic labelled `priority:p1` only, then the
   import finds no priority on it, and the epic keeps its local priority as
   today for an epic without the label.
3. Given a quarter prefix `target/` and an epic labelled `target/2026-q4`, then
   its quarter is 2026-Q4.
4. Given any quarter prefix and an epic labelled with a bare `2026-Q3` only,
   then its quarter is 2026-Q3, as today.
5. Given a readiness prefix `stage-` and an epic labelled `stage-ready`, then
   its readiness is ready.
6. Given the values under a prefix, then only the existing values are read:
   `prio-p4`, `target/2026-q5` and `stage-done` set no axis.

#### US3 (P1) - The roadmap writes under the project's prefixes

1. Given a priority prefix `prio-`, when a person sets P2 on an epic, then the
   label `prio-p2` is added and `prio-p0`, `prio-p1`, `prio-p3` are removed.
2. Given a quarter prefix `target/`, when a person sets 2026-Q4, then
   `target/2026-q4` is added, and every other label under `target/` and every
   bare quarter label is removed, as today for `quarter:`.
3. Given a readiness prefix `stage-`, when a person sets shaping, then
   `stage-shaping` is added and `stage-idea`, `stage-ready` are removed.
4. Given an epic still carrying `priority:p1` after the prefix became `prio-`,
   when a person sets P2, then `priority:p1` is left on the epic untouched: a
   push removes axis labels under the current prefix only.
5. Given an epic of a declared roadmap project (#632) with axis writes opened,
   then the reading and the writing use the prefixes of the project whose
   roadmap shows it.

#### US4 (P1) - Changing a prefix migrates nothing

1. Given epics whose stored priority came from `priority:` labels, when the
   prefix becomes `prio-`, then the stored values are kept, and the next import
   no longer finds them under the new prefix.
2. Given that state, then those epics appear in the pending pushes, which the
   person confirms as today; a confirmed push writes `prio-<value>`.
3. Given the old `priority:p1` left on an epic, then after the change it is a
   free label: it is shown, filtered and edited as one.

#### US5 (P1) - The project's axis labels stay protected

1. Given a priority prefix `prio-`, when a person adds `prio-p1` from the free
   label editor of an epic, then the edit is refused, in the web app before any
   request and on the server, with the existing "belongs to a roadmap axis"
   message.
2. Given the same project, when a person adds `priority:p1` as a free label,
   then it is accepted: it no longer belongs to an axis.
3. Given any prefixes, then `roadmap:` labels and bare quarters stay refused as
   free labels.
4. Given an epic's labels shown in the roadmap, then the labels under the
   project's current prefixes are hidden from the free labels, and those under
   former prefixes are shown.

### Part B - Ticket priority mapping (Jira)

#### US6 (P1) - The mapping is discovered and stored

As a project administrator on a Jira site whose priorities are named `P1` to
`P4`, I want to see how Sectile reads them and fix what it guessed.

1. Given a Jira project with no stored mapping, when the project is synced or
   the person refreshes the mapping from the Tracker tab, then each option of
   the project's priority scheme is stored with a level and a confidence.
2. Given an option whose name Sectile's aliases know (`Highest`, `Blocker`,
   `Critical`, `Major`, `Minor`, their French translations), then its line is
   sure, with the level the aliases give (`Critical` high, `Major` medium).
3. Given an option the aliases do not know (`P1`, `S2`), then its line is
   guessed, with the level its rank in the scheme gives today.
4. Given a stored mapping, when a later discovery finds a new option, then only
   that option is added; no existing line changes level or confidence, and no
   hand-set line is ever overwritten.
5. Given a stored mapping, when an option disappears from the scheme, then its
   line is removed at the next discovery.
6. Given a GitHub, GitLab or local project, then there is no mapping and no
   table.

#### US7 (P1) - The Tracker tab shows and corrects the mapping

1. Given a Jira project with a stored mapping, then the Tracker tab lists each
   option in the scheme's order with its level, and marks the guessed lines.
2. Given a guessed line, when the person confirms it, then it becomes sure with
   the same level.
3. Given any line, when the person picks another level, then the line takes it
   and becomes sure.
4. Given several options on one level, then the table shows which one a write
   sends, the most urgent by default, and lets the person pick another.
5. Given a level no option maps to, then the table says that level cannot be
   written on this project.
6. Given a project whose scheme Sectile names entirely (Atlassian's default,
   the Server scheme, their French translations), then no line is marked
   guessed.

#### US8 (P1) - An update with a guessed priority is refused

1. Given a Jira project whose only option on the high level is a guessed line,
   when a person changes a ticket's priority to high from the card form, then
   the change is refused before anything is written, locally or on the
   tracker, with a message that names the levels this project accepts and
   points to the Tracker tab of the project settings.
2. Given that refusal, then the card keeps its previous priority, and the next
   sync has nothing to undo.
3. The same refusal applies to the list bulk action (each refused ticket is
   reported, the others are written), to the priority chips, and to an MCP
   `update_task` carrying a priority.
4. Given the same project, when the level has at least one sure line, then the
   update is accepted and the tracker receives the preferred sure option.
5. Given an update that changes other fields and a guessed priority, then the
   whole update is refused: no field is written.
6. Given a Jira project with no stored mapping yet, then a priority update
   behaves as today.
7. Given a GitHub, GitLab or local project, then nothing changes.

#### US9 (P2) - A creation with a guessed priority is created without it

1. Given a Jira project whose requested level has only guessed lines, when a
   person or an agent creates a ticket with that priority, then the ticket is
   created on the tracker without a priority, and the result says the priority
   was not written and why.
2. Given a requested level with a sure line, then the ticket is created with
   the preferred sure option, as today.

### Part C - Axis custom fields (Jira, after the owner's confirmation)

#### US10 (P2) - A project maps an axis to a custom field

1. Given a Jira project with at least one epic, when the person opens the
   Roadmap section of the Tracker tab, then for the priority and for the
   quarter they can pick a field among the closed-list custom fields of an
   epic's edit screen: single select and cascading select only.
2. Given a project with no epic yet, then the field pickers are disabled with a
   message saying an epic is needed to discover the fields.
3. Given no field picked, then the axis is written and read as labels only, as
   in parts A and B, and no extra tracker request is made.

#### US11 (P2) - The option maps are prefilled and editable

1. Given a priority field, then the map shows four selects, one per level `p0`
   to `p3`, prefilled only with an option whose label equals the level, case
   ignored (`P0` for `p0`); anything else is left empty for the person to set.
2. Given a flat quarter field, then each option labelled `2026 - Q4`,
   `2026-Q4` or `2026 Q4` is prefilled as that quarter.
3. Given a cascading quarter field, then a year parent with a `Q4` or
   `2026 - Q4` child is prefilled as that quarter, stored as `parent/child`.
4. Given any line of either map, then the person can change or clear it, and a
   value set by hand is never replaced by a deduction.
5. Given an option format the deduction does not recognise, then the person
   maps it by hand in the settings.

#### US12 (P2) - Writing an axis with a field mapped

1. Given a mapped priority field, when a person sets P1, then the label is
   written as in part A and the field is set to the option mapped to `p1`.
2. Given a quarter value missing from the map, when it is written, then the
   field's options are looked up at write time; a match found by the deduction
   is written and stored on the project.
3. Given a value with no option in the field, then the label is still written,
   the field is left as it is, and the activity says which value is missing
   from which field.
4. Given a value cleared, then the label is removed and the field is cleared.

#### US13 (P2) - Reading an axis field back

1. Given a mapped field, when the epics are imported, then the field is read
   first and the label second: a person changing the field in Jira is seen at
   the next sync.
2. Given a field value with no entry in the map, then it is ignored and the
   label decides.
3. Given an epic whose field and label disagree, then it appears in the pending
   pushes, and a confirmed push writes both.

#### US14 (P1) - Guardrails of part C

1. No identifier of a real instance appears in the code, the tests, the
   fixtures or the documentation: no `customfield_<n>` id and no field or
   option name taken from a real site. Fixtures are synthetic.
2. Nothing about a field is compiled in: candidates come from `editmeta`, the
   field id and the option ids are data stored on the project.
3. Every deduced mapping only fills holes and stays editable by hand.
4. A project that picks no field keeps exactly the behaviour of parts A and B.

## Functional requirements

### Part A

- **FR-A1** A project stores three optional axis prefixes: priority, quarter,
  readiness. Empty means the default prefix.
- **FR-A2** A prefix is cleaned before storage: trimmed, lower-cased, a leading
  `#` removed. It is refused when it contains whitespace, when it is empty once
  cleaned but was not empty as typed, when it starts or is started by another
  axis's effective prefix, or when it starts or is started by `roadmap:`.
- **FR-A3** The import reads each axis under the project's effective prefix,
  with the existing values only. The bare quarter is read whatever the quarter
  prefix is.
- **FR-A4** Every axis push adds the label under the effective prefix and
  removes the other values under that prefix only; the quarter push also
  removes bare quarters, as today.
- **FR-A5** The pending pushes compare the stored value with the value read
  under the effective prefix.
- **FR-A6** A label under an effective prefix, a `roadmap:` label and a bare
  quarter are axis labels: refused by the free label editor on the server and
  in the web app, and hidden from the epic's free labels. A label under a
  former prefix is a free label.
- **FR-A7** Changing a prefix changes no stored value and writes nothing on
  the tracker.
- **FR-A8** The settings are project settings stored on the server, never
  workstation overrides, and are shown only when the tracker can carry epic
  labels.

### Part B

- **FR-B1** A Jira project stores a priority mapping: per option of its
  scheme, the option id, its name, its rank, a level, a confidence (sure or
  guessed) and whether it was set by hand; and per level, an optional preferred
  option.
- **FR-B2** Discovery runs at each sync of the project and on demand from the
  Tracker tab. It adds the options that have no line, removes the lines of
  options gone from the scheme, and never changes an existing line.
- **FR-B3** A new line is sure when its name is in Sectile's aliases, with the
  aliases' level; otherwise guessed, with the rank-based level.
- **FR-B4** A person can set the level of any line, which makes it sure and
  hand-set, confirm a guessed line, and pick the preferred option of a level.
- **FR-B5** A level is writable when at least one sure line carries it. A write
  sends the preferred option when it is sure, else the most urgent sure option
  of the level.
- **FR-B6** An update carrying a priority whose level is not writable on a Jira
  project with a stored mapping is refused before any local write, with a
  message naming the writable levels and the Tracker tab. Every update path
  goes through that check: the web update, the bulk action, the chips and the
  MCP `update_task`.
- **FR-B7** A creation carrying a priority whose level is not writable is
  created without a priority, and its result carries a notice saying so.
- **FR-B8** A queued tracker write that finds the level no longer writable
  (the mapping changed since the local write) fails its activity with the same
  message rather than sending a guessed option.
- **FR-B9** The read path is unchanged: a priority read from the tracker still
  falls back on the rank.
- **FR-B10** Before the first discovery, priority writes behave as today.

### Part C

- **FR-C1** A Jira project may store, for the epic priority and for the epic
  quarter, one custom field: its id, its kind (single or cascading select) and
  an option map. No field means labels only.
- **FR-C2** Field candidates are the single and cascading select custom fields
  of the edit screen of one epic of the project.
- **FR-C3** The priority map is prefilled only from options whose label equals
  a level, case ignored. The quarter map is prefilled from recognised quarter
  labels, flat or `parent/child`. A deduction never overwrites a hand-set
  entry.
- **FR-C4** An axis write with a field mapped writes the label and the field.
  A quarter missing from the map is looked up in the field's options at write
  time and, when deduced, stored on the project. A value with no option leaves
  the field untouched and is named in the activity.
- **FR-C5** The import requests the mapped field ids with the labels and reads
  the field first, the label second. An unmapped field value is ignored.
- **FR-C6** The pending pushes include an epic whose mapped field and label
  disagree with the stored value.
- **FR-C7** No real instance identifier appears in the repository.

## Edge cases

- A prefix equal to its default is stored as typed; it behaves as the default.
- A prefix `p` for the priority reads `pp1`, not `p1`: the value always follows
  the full prefix.
- A label `prio-P1` is read as `p1`: values are compared case-insensitively, as
  today.
- An epic carrying two values under the priority prefix: the first valid one
  wins, as today; for the readiness the most advanced wins, as today.
- A Jira scheme with a single option: that option's line carries the level the
  aliases or the rank give; the other levels are not writable unless set by
  hand.
- A Jira screen whose priority options differ from the project's scheme: the
  write picks among the screen's options using the stored lines; a screen
  option with no stored line is treated as guessed until the next discovery
  stores it.
- A Jira project whose screen does not carry the priority field: unchanged,
  the creation goes out without it, and an update sends no priority.

## Acceptance criteria

- **AC1** Parts A, B and C each ship as one pull request, in that order. Part C
  is not started before the owner confirms it after A and B are merged.
- **AC2** A project with no prefix, no stored mapping and no field behaves
  exactly as before, against the existing test suite.
- **AC3** Every user story scenario above is covered by an automated test,
  server side or web side.
- **AC4** New project columns are added in `internal/db/migrations.go` only,
  never in the frozen baseline, and the rewind test helpers drop them.
- **AC5** The strings shown to users exist in French and English.
- **AC6** `CHANGELOG.md` carries one line under `## [Unreleased]` per part, in
  the pull request of that part.
- **AC7** (part C) No `customfield_<n>` id and no real field or option name in
  the repository; a test fixture names only synthetic fields.

## Open points

None blocking. One assumption is settled by this specification rather than by
the clarification, and is easy to reverse: the epics of a declared roadmap
project (#632) are read and written under the prefixes of the project whose
roadmap shows them (US3.5), since those prefixes are the only ones Sectile
knows.
