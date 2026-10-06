# Specification #680 - Map the epic priority and quarter to Jira custom fields

- Ticket: https://github.com/sebastienferry/sectile/issues/680 (milestone M-11,
  Roadmap), part C of #635
- Branch: `feat/680`
- Clarification: `docs/clarifications/680.md` (rounds 1 and 2; gate G1 closed,
  part C confirmed by the owner) and `docs/clarifications/635.md`
- Earlier specification: `specs/635-epic-axes-tracker-fields/`, part C (US10
  to US14, FR-C1 to FR-C7). This file supersedes it for part C and applies the
  corrections of the #680 clarification.
- Framework: Spec Kit

## Summary

On Jira, a project may map the epic priority and the epic quarter each to one
closed-list custom field of its epics, a single select or a two-level
cascading select, picked from an epic's edit screen. Each mapped field carries
an option map, axis value to option, prefilled by generic deductions that only
fill holes and editable by hand. An axis write sets the label, as today, and
the field. The import reads the field first and the label second, and an epic
whose field disagrees with the decided value joins the pending pushes. A
project that maps no field keeps exactly today's label-only behaviour.

Nothing about a field is compiled into Sectile: the field, its kind and its
option maps are project data stored in the database.

## Scope

In scope: the project setting and its storage, field discovery from an epic's
`editmeta`, the option deductions, the priority and quarter pushes, the epic
import, the pending pushes, the Tracker tab of the project settings, the
French and English strings of the web app, and the changelog.

Out of scope:

- A field for the readiness axis or for the horizon.
- Writing one axis to two fields.
- The ticket priority (part B, #679).
- GitHub and GitLab.
- Migrating values already written, labels or fields.
- Any field id, field name or option name in the repository.

## Vocabulary

- **Axis field**: a Jira custom field of the project's epics mapped to the
  epic priority or to the epic quarter. Its kind is `select` (single select)
  or `cascade` (two-level cascading select).
- **Option path**: how an option is named in a map: its id for a select,
  `parentId/childId` for a cascade.
- **Option map**: for one axis field, each axis value (`p0` to `p3`, or a
  quarter `YYYY-Qn`) with the option path it is written as.
- **Hand-set value**: an axis value whose line a person set, changed or
  cleared in the settings. No deduction changes it again.
- **Deduction**: a generic rule matching an axis value with an option label:
  an option labelled exactly the level (`P0` for `p0`, case ignored), or a
  quarter label in a recognised format.

## User stories

### US1 (P1) - A project maps an axis to a custom field

1. Given a Jira project with at least one epic, when the person opens the
   Tracker tab of the project settings and asks for the fields, then for the
   priority and for the quarter they can pick a field among the single select
   and cascading select custom fields of one epic's edit screen.
2. Given a project with no epic yet, then the pickers stay empty and a message
   says that an epic is needed to discover the fields.
3. Given no field picked for an axis, then that axis is written and read as
   labels only, exactly as before, and no extra tracker request is made.
4. Given a project whose tracker is not Jira, then the section is not shown.
5. Given a picked field, when the person picks "no field", then the mapping of
   that axis is removed on save.

### US2 (P1) - The option maps are prefilled and editable

1. Given a priority field, then its map shows one line per level `p0` to `p3`,
   prefilled only with an option whose label equals the level, case and
   surrounding spaces ignored; any other level is left empty.
2. Given a flat quarter field, then each option labelled `2026 - Q4`,
   `2026-Q4` or `2026 Q4` (case ignored) is prefilled as that quarter.
3. Given a cascading quarter field, then a parent labelled with a year and a
   child labelled `Q4` or `<same year> - Q4` (any recognised format) is
   prefilled as that quarter, as the path `parent/child`.
4. Given any line of either map, then the person can change or clear it, and a
   value set, changed or cleared by hand is never replaced by a deduction.
5. Given an option the deductions do not recognise, then the person maps it by
   hand, adding a quarter line when needed.
6. Given a cascading priority field, then the same rules apply to its leaf
   options (a child labelled `P1` under any parent), stored as
   `parent/child`.

### US3 (P1) - Writing an axis with a field mapped

1. Given a mapped priority field, when a person sets P1 on an epic, then the
   label is written as before and the field is set to the option mapped to
   `p1`.
2. Given a quarter missing from the map, when it is written, then the field's
   options are read from that epic's edit screen at write time; an option the
   deduction matches is written, and the learned line is stored on the
   project, filling a hole only.
3. Given a value with no option in the field, then the label is still written,
   the field is left as it is, and the activity says which value has no
   option in which field.
4. Given a value cleared, then the label is removed and the field is cleared.
5. Given an epic whose edit screen does not carry the mapped field, then the
   label is written and the activity says the field is absent from that epic.
6. Given a field write that fails on the tracker after the label was written,
   then the activity fails, says the label was written and the field not, and
   the epic stays in the pending pushes.

### US4 (P1) - Reading an axis field back

1. Given a mapped field, when the epics are imported, then the field is read
   first and the label second: a person changing the field in Jira is seen at
   the next sync.
2. Given a field value that no line of the map carries, then it is ignored and
   the label decides.
3. Given an epic whose mapped field disagrees with the decided value, then it
   appears in the pending pushes, and a confirmed push writes both the label
   and the field.
4. Given a decided value that has no option in the map, then the field alone
   does not make the epic pending: it cannot be written.

### US5 (P2) - Epics of a declared roadmap project (#632)

1. Given an epic of a declared roadmap project shown on this project's
   roadmap, then its field is read and written with this project's mapping, as
   part A reads it under this project's prefixes.
2. Given such an epic, then the field is written only where
   `RoadmapAxisWrites` already allows the label write.

### US6 (P1) - Guardrails

1. No identifier of a real instance appears in the code, the tests, the
   fixtures or the documentation: no `customfield_<n>` id and no field or
   option name taken from a real site. Test fields and options are synthetic.
2. No tracker-specific mapping lives in Sectile's code: candidates come from
   `editmeta` at runtime, the field id, its kind and the option maps are data
   stored on the project.
3. Every deduction only fills holes and stays editable by hand.
4. A project that maps no field keeps exactly the behaviour of parts A and B,
   with no extra tracker request.

## Functional requirements

- **FR-1** A Jira project stores, for the epic priority and for the epic
  quarter, at most one axis field: its id, its display name, its kind
  (`select` or `cascade`), its option map and its hand-set values. No field
  means labels only.
- **FR-2** Field candidates are the custom fields of the edit screen of one
  epic of the project whose schema `custom` type ends with `:select` or
  `:cascadingselect`. No field id or name decides it.
- **FR-3** The priority map is prefilled only from options whose label equals
  a level, case ignored. The quarter map is prefilled from options whose label
  is a quarter in a recognised format, flat or `parent/child`. A deduction
  never overwrites a line, never fills a hand-set value, and is never applied
  to a value the person cleared.
- **FR-4** A save of the setting refuses an unknown kind, an empty field id,
  an axis value outside its set and an empty option path, with a sentence. An
  edit of the map of the stored field marks every changed, added or cleared
  value hand-set. Picking another field replaces the whole entry.
- **FR-5** An axis push with a field mapped writes the label, then the field.
  A quarter missing from the map is looked up in the field's options on the
  epic's edit screen and, when deduced, stored on the project. A value with no
  option, or a field absent from the epic, leaves the field untouched and is
  named in the activity. A cleared value clears the field.
- **FR-6** The epic search requests the mapped field ids along with the labels
  when the project maps any, and only then. The import reads the field first
  through the reverse option map, then the label. An unmapped field value is
  ignored.
- **FR-7** The pending pushes include an epic whose mapped field differs from
  the option of the decided value, when that value has an option.
- **FR-8** The settings are project settings stored on the server, edited in
  the Tracker tab, under the axis prefixes, and shown only for Jira.
- **FR-9** No real instance identifier appears in the repository.

## Edge cases

- A priority option labelled ` p1 ` matches `p1`; `P1 - High` matches nothing.
- Two options matching the same value: the first in the field's order wins.
- A quarter option labelled `2026 - Q4` under a cascade parent `2026` matches
  only when its year equals the parent's.
- A stored option path that the field no longer offers: the write is refused
  by Jira, the activity fails with its message, and the person corrects the
  line by hand.
- The project picks a field of the wrong kind for an axis (a cascade for the
  priority): allowed, the deductions run on its leaves.
- A field whose stored id is no longer on the epic's screen: same as US3.5.
- An epic imported with the field empty and no label: keeps its local value.

## Acceptance criteria

- **AC1** Every user story scenario above is covered by an automated test,
  server side or web side.
- **AC2** A project with no field behaves exactly as before against the
  existing test suite, and a test asserts the epic search requests no extra
  field and the push makes no extra request.
- **AC3** The new project column is added in `internal/db/migrations.go` only
  (migration 48), never in the frozen baseline, and the rewind test helpers
  drop it.
- **AC4** The web strings exist in French and English; new server messages
  are in English.
- **AC5** `CHANGELOG.md` carries one line under `## [Unreleased]` / `Added`.
- **AC6** `grep -rnE 'customfield_[0-9]+'` finds nothing the change added, and
  no real field or option name appears; fixtures name only synthetic fields.
- **AC7** No tracker-specific mapping lives in Sectile's code, only in a
  project's stored settings (owner's condition, round 2).

## Open points

None. Every product question is settled in `docs/clarifications/680.md`.
