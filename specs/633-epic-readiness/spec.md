# Specification #633 - Roadmap: let a person decide an epic's readiness

- Ticket: https://github.com/sebastienferry/sectile/issues/633 (milestone M-11,
  Roadmap)
- Branch: `feat/633`
- Clarification: `docs/clarifications/633.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

An epic on the roadmap gets a readiness level of its own: idea, shaping or
ready. A person decides it from the epic's panel, or by dropping the epic on a
readiness section. Until somebody decides, Sectile shows a suggested level
deduced from what the epic carries (its framing, its slicing, its stories),
marked with `?`. The level is stored in Sectile and, on a Jira project, also
written on the epic as a `readiness:*` label. It becomes a third grouping axis
of the roadmap, next to the priority and the quarter of #628.

The maturity the roadmap already derives from the child tickets stays, and is
relabelled so that it reads as the stage of the tickets rather than as a second
"ready".

## Scope

In scope: the roadmap view of the web app (rows, panel, grouping, drop), the
macro storage and API, the tracker label write and read-back, the French and
English strings, and the changelog.

Out of scope:

- Replacing or removing the maturity derived from the child tickets.
- A readiness filter or sort in the toolbar.
- Naming the label prefix per project, or mapping the level to a tracker
  custom field (#635).
- Writing the level on another project's epic (#632).
- A dedicated ideation view.
- Sectile Desktop screens other than the web roadmap it embeds.

## Dependencies

- **#627** (epic priority and quarter), merged: the storage, the label push,
  the read-back, the pending pushes and the panel controls this axis extends.
- **#626** (epic labels), merged: the `readiness:` prefix joins the axis
  prefixes, so the label is never shown or edited as a free label.
- **#628** (group and drag epics by priority or quarter), clarified, not
  implemented: US6 and US7 below reuse its axis control, sections, folding,
  selection, drop and drop report. They are implemented only once #628 is
  merged. US1 to US5 do not depend on it.

## Vocabulary

- **Readiness level**: one of `idea`, `shaping`, `ready`, shown as "Idée / En
  cadrage / Prête" in French and "Idea / Shaping / Ready" in English.
- **Decided level**: the level a person set. Empty means nobody decided; empty
  is not `idea`.
- **Suggested level**: the level Sectile deduces from the epic while nobody
  decided. It is never stored and never written to the tracker.
- **Framing**: the epic's description, whether typed in the panel or imported
  from the tracker.
- **Slicing**: the epic's list of slicing lines (todos). A line is covered when
  it is ticked or already has a story.
- **Readiness label**: `readiness:idea`, `readiness:shaping`,
  `readiness:ready`.
- **Tickets' stage**: the maturity derived today from the child tickets (Draft,
  Clarified, Specified, Ready): the stage the least advanced open ticket has
  reached.
- **Labelled tracker**: as in #627, a project whose tracker can carry labels on
  its epics. Today that is Jira. GitHub milestones, GitLab and local projects
  are not labelled trackers.

## User stories

### US1 (P1) - Sectile suggests a level while nobody decided

As a person reading the roadmap, I see at a glance how far each epic is from
being ready to deliver, even before anyone judged it.

1. Given an undecided epic with at least one child ticket, then its suggested
   level is Ready.
2. Given an undecided epic with no child ticket, a non-empty framing, and a
   slicing of at least one line where every line is covered, then its
   suggested level is Ready.
3. Given an undecided epic with no child ticket, and either a framing or at
   least one slicing line, but not both conditions of US1.2, then its
   suggested level is Shaping.
4. Given an undecided epic with no child ticket, no framing and no slicing
   line, then its suggested level is Idea.
5. Given an epic with a decided level, then no suggestion is shown, whatever
   the epic carries.
6. Given a framing made only of whitespace, then it counts as no framing.

### US2 (P1) - One badge shows the level on every row

1. Given a decided epic, then both its condensed and its expanded row show one
   badge with the decided level's name.
2. Given an undecided epic, then both rows show the suggested level's name
   followed by `?`, styled as a suggestion, with a tooltip saying that nobody
   decided yet and that this is Sectile's suggestion.
3. The rows carry no readiness chips: the badge is not a control.

### US3 (P1) - Decide or clear the level from the panel

As a roadmap owner, I decide an epic's readiness where I read its framing.

1. Given an epic selected, then its panel shows three chips, Idea, Shaping and
   Ready, next to the priority. When nobody decided, the chip of the
   suggestion is highlighted and marked `?`, with the same tooltip as US2.2.
2. When I click a chip, then that level becomes the decided level, and the
   row, the badge and the panel show it at once.
3. When I click the chip of the decided level again, then the level is cleared,
   and the badge shows the suggestion with `?` again.
4. Given a labelled tracker, when a level is set, then the epic receives the
   matching readiness label and loses the other two; when the level is
   cleared, every readiness label is removed from the epic.
5. Given a project that is not a labelled tracker, or an epic that is a
   milestone, a local key or an epic of another project, when I set a level,
   then it is stored and shown, the panel says it is kept in Sectile and not
   written to the tracker (the line #627 already shows), and no tracker write
   is attempted.
6. Given a labelled tracker that refuses the write, then the level stays set
   in Sectile, and the queued activity says the readiness is set in Sectile
   but not on the ticket, naming the tracker error.

### US4 (P1) - The tickets' stage no longer reads as a second "ready"

1. Given any epic, then the badge derived from the child tickets keeps its
   four values and its colours, but reads "Tickets : Brouillons / Clarifiés /
   Spécifiés / Prêts" in French and "Tickets: Draft / Clarified / Specified /
   Ready" in English.
2. Its tooltip says it is the stage the least advanced open ticket has
   reached.
3. Given an epic decided Ready whose tickets are all past specification, then
   the row shows both badges, and no text on the row says "Ready" without its
   "Tickets" prefix except the readiness badge.

### US5 (P2) - A level set on the tracker is read back

1. Given a Jira epic that carries `readiness:shaping` set on the tracker, when
   the epic labels are read (the sync or "import labels"), then Sectile shows
   Shaping as decided, replacing a different local level.
2. Given a Jira epic carrying `readiness:idea` and `readiness:ready`, then its
   level is Ready: the most advanced wins. The read changes no label.
3. Given a Jira epic carrying no readiness label but a local level, then the
   local level is kept and the epic is counted among the pending pushes;
   pushing them writes it.
4. Given a label `readiness:soon`, then it is ignored as if absent. Case and a
   leading `#` are ignored on read (`Readiness:Ready` reads as Ready).
5. Given any readiness label, then it never shows among the epic's free
   labels, is never offered by the label filter, and cannot be added or
   removed through the free-label editor.

### US6 (P2, after #628) - Group the roadmap by readiness

1. Given the grouping control of #628, then it offers a third axis,
   "readiness", next to priority and quarter.
2. When the readiness axis is chosen, then the groupable tabs show the
   sections Idea, Shaping, Ready in that order, each with its name and its
   count. These three always show, with 0 when empty.
3. Given at least one undecided epic in the tab, then a "Non décidé" / "Not
   decided" section follows the three levels, with its count; each of its rows
   shows its suggested level with `?`. Without undecided epics in the tab, the
   section does not show.
4. An epic sits in the section of its decided level, never in the section of
   its suggestion.
5. The order inside a section, the folding, its memory per axis and value
   (`readiness:ready`, `readiness:none`), the filters applied first, the
   unchanged tab counts and the flat Hidden tab behave as #628 specifies for
   its two axes.

### US7 (P2, after #628) - Drop an epic on a readiness section

1. When I drop one epic, or a selection of epics, on the Idea, Shaping or
   Ready section, then each dropped epic takes that level as its decided
   level, through the same save as the panel (US3.4 to US3.6).
2. When I drop on "Not decided", then each dropped epic's level is cleared.
3. Given dropped epics already at the target (the same decided level, or
   already undecided for "Not decided"), then they are skipped.
4. The drop changes nothing but the readiness: not the horizon, the priority,
   the quarter, the title nor anything else.
5. The drop reports once, as #628 does: how many were set, how many skipped,
   and the keys of those that failed.

## Functional requirements

- **FR1** An epic stores an optional readiness level among `idea`, `shaping`,
  `ready`. Empty means undecided. After upgrading, every epic is undecided.
- **FR2** The suggested level is computed in the web app from the row, in this
  order, the first matching rule winning: at least one child ticket gives
  Ready; a non-blank framing and a non-empty slicing whose every line is
  covered gives Ready; a non-blank framing or at least one slicing line gives
  Shaping; otherwise Idea. It is never stored, sent, pushed or read back.
- **FR3** Every roadmap row, condensed and expanded, shows one readiness badge:
  the decided level, or the suggested level followed by `?` with the
  "not decided" tooltip.
- **FR4** The panel sets the level with three chips next to the priority; a
  click on the decided chip clears it. The change is visible at once and
  stored before any tracker write.
- **FR5** On a labelled tracker, each change is written on the epic through a
  queued activity, never inside the click. The axis is exclusive: at most one
  readiness label; setting a level removes the other two, clearing removes all
  three. No other label, field, title or status of the epic is written.
- **FR6** Milestones, local keys, epics of another project and every epic of a
  project that is not a labelled tracker keep their level in Sectile only, are
  never pushed, never listed as pending, and the panel says so.
- **FR7** A refused tracker write leaves the local level in place and is
  reported in the activity, naming the epic and the tracker error.
- **FR8** Reading the epic labels applies the level from the tracker when the
  epic carries a valid readiness label, the most advanced one winning when it
  carries several, and keeps the local level otherwise. An unknown value is
  ignored. Case and a leading `#` are ignored.
- **FR9** The pending pushes cover the readiness with the horizon, the priority
  and the quarter: an epic with a decided level is pending when its labels do
  not read that level, and the push writes that axis.
- **FR10** `readiness:` is an axis prefix: its labels are hidden from the free
  labels, the label filter and the free-label editor, on the client and on the
  server.
- **FR11** The tickets' stage badge keeps its values and computation, gains the
  "Tickets" prefix in its text and a tooltip naming what it measures.
- **FR12** (after #628) The grouping control offers the readiness axis, with
  the sections, counts, order, folding and drop of US6 and US7.
- **FR13** Every new or changed string exists in French and English.
- **FR14** `CHANGELOG.md` gets one `Added` line under `[Unreleased]` for the
  epic readiness level (decided from the panel or by drop, suggested until
  then, written as a Jira label, groupable), which also says the tickets'
  stage badge now reads "Tickets: ...".

## Acceptance criteria

- **AC1** US1 to US5 scenarios pass, on a Jira project and on a GitHub project.
- **AC2** Once #628 is merged, US6 and US7 scenarios pass.
- **AC3** After upgrading, every epic is undecided and shows its suggestion;
  no existing macro data (horizon, framing, slicing, priority, quarter,
  labels) is lost.
- **AC4** A readiness change on a Jira epic produces exactly one queued tracker
  activity carrying only label changes.
- **AC5** No row shows the word "Ready" / "Prête" for the tickets' stage
  without its "Tickets" prefix.
- **AC6** The web unit tests, the Go tests (SQLite and PostgreSQL) and the
  typecheck pass.

## Edge cases

- An epic whose tickets are all closed still has child tickets: its suggestion
  is Ready.
- A slicing line with a story key counts as covered even when unticked.
- An epic with an empty slicing and a framing is Shaping, not Ready: an empty
  list is not "every line covered".
- An epic with no ticket still shows the tickets' stage as "Tickets :
  Brouillons" / "Tickets: Draft", as its derived maturity is "Draft" today;
  this ticket changes the label only.
- An epic closed on the tracker keeps its level and can still be changed.
- Two tabs changing the same epic: last write wins, as for the other axes.
- An epic moved to another project carries its level.
- A browser that remembered the readiness axis (#629 preferences) and is then
  served a build without it falls back to no grouping, as #628 does for any
  unknown stored value.

## Open points

- **Name of the axis itself** (open, non-blocking). The clarification named the
  three levels but not the axis, which labels the panel chip group, the
  grouping option and the activity texts. The plan uses "Maturité" /
  "Readiness", the prefix "Tickets" being what tells it apart from the
  tickets' stage. The owner confirms or renames it at review; a rename touches
  strings only, never behaviour.

Every product question of the clarification is otherwise settled. The only
sequencing constraint is US6 and US7 waiting for #628.
