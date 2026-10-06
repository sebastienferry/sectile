# Specification #679 - Store and correct the Jira ticket priority mapping

- Ticket: https://github.com/sebastienferry/sectile/issues/679
- Branch: `feat/679`
- Clarification: `docs/clarifications/679.md` (round 1), on top of
  `docs/clarifications/635.md` (rounds 1 and 2, settled with the owner)
- Extends: `specs/635-epic-axes-tracker-fields/` part B (FR-B1 to FR-B10,
  US6 to US9), which this folder supersedes for part B
- Framework: Spec Kit

## Summary

A Jira project stores how the options of its ticket priority scheme map to
Sectile's four levels (`urgent`, `high`, `medium`, `low`). Each line is sure
(a name Sectile's aliases know, or a level a person set) or guessed (matched
by rank only). The Tracker tab shows the table, marks the guessed lines and
lets a person confirm or change each one, and pick which option a level writes
when several share it. A priority update whose level has no sure line is
refused before anything changes; a creation goes out without the priority and
says so.

## Scope

In scope: the stored mapping, its discovery at sync and on demand, the table
in the Tracker tab, the refusal of an update on every write path, the creation
without priority and its notice, the refusal of a queued tracker write whose
level stopped being writable, and the changelog.

Out of scope:

- GitHub, GitLab and local projects: no mapping, no table, no change.
- The read path: a priority read from Jira still falls back on the rank.
- Sectile's aliases themselves (`Critical` stays high, `Major` medium).
- The epic axes and their custom fields (#635 part C, behind the owner's
  confirmation gate).

## User stories

### US1 (P1) - The mapping is discovered and stored

As a project administrator on a Jira site whose priorities are named `P1` to
`P4`, I want to see how Sectile reads them and fix what it guessed.

1. Given a Jira project with no stored mapping, when the project is fully
   synced or a person refreshes the mapping from the Tracker tab, then each
   option of the project's priority scheme is stored with a level and a
   confidence.
2. Given an option whose name Sectile's aliases know (`Highest`, `Blocker`,
   `Critical`, `Major`, `Minor`, their French translations), then its line is
   sure, with the level the aliases give.
3. Given an option the aliases do not know (`P1`, `S2`), then its line is
   guessed, with the level its rank in the scheme gives today.
4. Given a stored mapping, when a later discovery finds a new option, then only
   that option is added; no existing line changes level or confidence, and no
   hand-set line is ever overwritten.
5. Given a stored mapping, when an option disappears from the scheme, then its
   line is removed at the next discovery, and so is a preferred choice that
   named it.
6. Given an incremental background sync, then the mapping is not rediscovered.
7. Given a person who just changed the Jira scheme, when they refresh from the
   Tracker tab, then the change is seen at once, not after a cache delay.
8. Given a GitHub, GitLab or local project, then there is no mapping and no
   table.

### US2 (P1) - The Tracker tab shows and corrects the mapping

1. Given a Jira project with a stored mapping, then the Tracker tab lists each
   option in the scheme's order with its level, and marks the guessed lines.
2. Given a guessed line, when the person confirms it, then it becomes sure with
   the same level.
3. Given any line, when the person picks another level, then the line takes it
   and becomes sure.
4. Given several sure options on one level, then the table shows which one a
   write sends, the most urgent by default, and lets the person pick another.
5. Given a level no sure option carries, then the table says that level cannot
   be written on this project.
6. Given a project whose scheme Sectile names entirely (Atlassian's default,
   the Server scheme, their French translations), then no line is marked
   guessed.
7. Given a Jira project with no mapping yet, then the table is empty and offers
   the refresh.

### US3 (P1) - An update with a guessed priority is refused

1. Given a Jira project whose high level has only guessed lines, when a person
   changes a ticket's priority to high from the card form, then the change is
   refused before anything is written, locally or on the tracker, with a
   message that names the levels this project accepts and points to the
   Tracker tab of the project settings.
2. Given that refusal, then the card keeps its previous priority, and the next
   sync has nothing to undo.
3. The same refusal applies to the priority chips, to an MCP `update_task`
   carrying a priority, and to the list bulk action, where each refused ticket
   is reported and the others are written.
4. Given an update that changes other fields and a refused priority, then the
   whole update is refused: no field is written.
5. Given an update that sends the priority the ticket already has, then it is
   not refused on that account.
6. Given a level with at least one sure line, then the update is accepted and
   Jira receives the preferred sure option of that level.
7. Given a Jira project with no stored mapping yet, then a priority update
   behaves as today.
8. Given a GitHub, GitLab or local project, then nothing changes.

### US4 (P2) - A creation with a guessed priority is created without it

1. Given a Jira project whose requested level has only guessed lines, when a
   person or an agent creates a ticket with that priority, then the ticket is
   created on Jira without a priority, and the result says the priority was
   not written and why: the web shows it in the creation toast, MCP
   `create_task` in its answer.
2. Given a requested level with a sure line, then the ticket is created with
   the preferred sure option, as today.

### US5 (P2) - A queued write meets a changed mapping

1. Given a priority update accepted locally, when the mapping changes before
   the queued tracker write runs and the level is no longer writable, then the
   activity fails with the same message instead of sending a guessed option.

## Functional requirements

- **FR-1** A Jira project stores a priority mapping: per option of its scheme,
  the option id, its name, a level, whether it is guessed and whether it was
  set by hand, in scheme order; and per level, an optional preferred option.
- **FR-2** Discovery runs at each full sync of a Jira project (not at an
  incremental background sync) and on demand from the Tracker tab, the latter
  reading the scheme fresh. It adds the options that have no line, removes the
  lines of options gone from the scheme and the preferred choices naming them,
  and never changes an existing line.
- **FR-3** A new line is sure when its name is in Sectile's aliases, with the
  aliases' level; otherwise guessed, with the rank-based level.
- **FR-4** A person can set the level of any line, which makes it sure and
  hand-set, confirm a guessed line, and pick the preferred option of a level.
  The server refuses an option or a preferred choice the stored mapping does
  not carry, and a level that is not one of the four.
- **FR-5** A level is writable when at least one sure line carries it. A write
  sends the preferred option when it is sure and offered by the screen, else
  the most urgent sure offered option of the level.
- **FR-6** An update that changes the priority to a level that is not writable,
  on a Jira project with a stored mapping, is refused before any local write,
  with a message naming the writable levels and the Tracker tab. Every update
  path goes through that check: the web update, the bulk action, the chips and
  MCP `update_task`.
- **FR-7** A creation carrying a priority whose level is not writable is
  created without a priority, and its result carries a notice saying so.
- **FR-8** A queued tracker write that finds the level no longer writable fails
  its activity with the same message rather than sending a guessed option.
- **FR-9** The read path is unchanged.
- **FR-10** Before the first discovery (empty mapping), priority writes behave
  as today.
- **FR-11** The refusal, the creation notice and the discovery summary are new
  messages, written in English. The table's labels exist in French and
  English.

## Acceptance criteria

- Every user story above holds, each covered by a test named after it.
- A project on Atlassian's default scheme, the Server scheme or their French
  translations sees no guessed line and no refusal after the upgrade.
- `CHANGELOG.md`, under `Unreleased`: `Added` the priority mapping table of a
  Jira project; `Changed` a Jira priority update that the table only guessed
  is refused, and a creation goes out without it.

## Open points

None. The scope and the refusal behaviour were settled with the owner on
#635 round 2; the drift since was settled in `docs/clarifications/679.md`.
