# Specification #632 - Roadmap: show the epics of declared tracker projects

- Ticket: https://github.com/sebastienferry/sectile/issues/632 (macro M-11,
  Roadmap)
- Branch: `claude/clarify-issue-gh-11a4f59c-cf19db`
- Clarification: `docs/clarifications/632.md` (rounds 1 to 3, confirmed by the
  owner)
- Framework: Spec Kit

## Summary

A Jira project can already declare other Jira projects of its roadmap, but
their only effect is that their story keys attach to slicing lines. With this
change the roadmap also reads the epics of those declared projects. The viewer
picks which projects the roadmap shows, each with its epic count, and the
choice is remembered. An epic of a declared project is marked read only: its
horizon, priority and quarter are kept in Sectile. A slicing line can create
its story in a declared project, under its epic. A project can opt in to
writing the priority and the quarter on the epics of its declared projects,
one epic at a time.

## Scope

In scope: the epic read of a Jira project, the roadmap view of the web app
(toolbar, rows, panel, slicing target picker), the story creation of a slicing
line, the project's tracker settings, the French and English strings, the
changelog and an ADR restating the read-only rule of declared projects.

Out of scope:

- Importing any work item of a declared project other than its epics. The
  board, the counters and the facets stay those of the project's own key.
- Declared projects on GitHub, GitLab or local projects: the setting exists for
  Jira projects only.
- Mapping the axes to tracker custom fields (#635). The opt-in writes whatever
  the own epics' axis writes write: labels today.
- Epic template sections (rows 44 to 46 of the #619 study, left out by #426).
- Creating the stories of several slicing lines at once (#634).
- Sectile Desktop screens other than the web roadmap it embeds.

## Vocabulary

- **Own key**: the Jira project key of the Sectile project (`PE`).
- **Declared key**: a Jira project key listed in the project's "Roadmap
  projects" setting (`DATA`).
- **Origin** of an epic: the key prefix of the epic's key (`DATA-12` has origin
  `DATA`).
- **Foreign epic**: an epic whose origin is not the own key.
- **Axes**: the horizon, the priority and the quarter of an epic.
- **Axis opt-in**: the project setting "Write the priority and the quarter on
  the epics of the roadmap projects". Closed by default.

## User stories

### US1 (P1) - The roadmap reads the epics of the declared projects

As a person who runs a roadmap spanning several Jira projects, I see their
epics beside mine without importing their tickets.

1. Given a project `PE` declaring `DATA` and `OPS`, when the epics are read
   (the sync or "import labels"), then the roadmap carries the epics of `PE`,
   `DATA` and `OPS`, with their title, status, closed state and labels.
2. Given the same read, then no story, task or bug of `DATA` or `OPS` enters
   the board, the backlog, the counters or the facets of `PE`.
3. Given `OPS` unreachable (unknown key, no permission, timeout), when the epics
   are read, then the epics of `PE` and `DATA` are read, and the read's summary
   names `OPS` and its error.
4. Given `PE` itself unreachable, then the read fails as it does today, whatever
   the declared keys answer.
5. Given a project declaring nothing, then the read is exactly today's.
6. Given a foreign epic carrying a horizon, priority or quarter label on the
   tracker, when the epics are read, then Sectile shows those values, as for an
   own epic (#627 US7).
7. Given a declared key removed from the setting, when the epics are read again,
   then its epics already stored are kept, and are no longer refreshed.

### US2 (P1) - Choose which projects the roadmap shows

As a viewer, I read my project's roadmap with only the projects I care about.

1. Given a project declaring `DATA` and `OPS`, then the roadmap toolbar offers
   an origin selection listing `PE`, `DATA` and `OPS`, each with the number of
   its epics.
2. Given a first visit, then only `PE` is selected.
3. When I tick `DATA`, then the epics of `DATA` join every horizon tab, and the
   tab counts follow.
4. When I untick every origin, then the roadmap shows the epics of `PE`, and
   `PE` is shown as selected.
5. Given a declared key with no epic, then it is offered with the count 0.
6. Given epics stored for a key that is no longer declared, then that key is
   still offered with its count, so the rows it left behind can be shown.
7. Given a count, then it is the number of epics of that origin the roadmap
   would list with the other current filters (closed epics, search, priority,
   labels), across every horizon tab.
8. Given a selection made on one project, when I leave the roadmap and come
   back, or reload the page, then the same selection is restored for that
   project. Another project keeps its own selection.
9. Given a project declaring nothing and carrying no foreign epic, then no
   origin selection is shown.
10. Given an active selection other than the own key alone, then a filter chip
    names it among the active filter chips, and clearing the chip returns to the
    own key alone.

### US3 (P1) - A foreign epic is marked read only

As a viewer, I know at a glance that an epic belongs to another team.

1. Given a foreign epic in a row, then the row shows its origin key.
2. Given a foreign epic selected, then its panel shows a "read only" mark naming
   its origin and saying that Sectile does not write on it.
3. Given a foreign epic, then its panel offers no free label edit, no framing
   comment post and no epic edit on the tracker.
4. Given a foreign epic, when I change its horizon (tab move or drag), then the
   horizon is stored and shown, no tracker write is queued and no failed
   activity appears, whatever the axis opt-in says.
5. Given a foreign epic, then it is never listed among the pending pushes, and
   "push the pending labels" never writes on it, whatever the axis opt-in says.
6. Given a foreign epic, then its slicing and TODO lines stay editable in
   Sectile, as for an own epic.

### US4 (P1) - A slicing line creates its story in a declared project

As a roadmap owner, I create the story where the work will be done, even when
it is another team's Jira project.

1. Given a slicing line of an epic of `PE`, then its target picker offers the
   Sectile projects it offers today, then `DATA` and `OPS`, each with a sublabel
   saying it is a Jira project of the roadmap whose stories stay in Jira.
2. Given the target `DATA`, when I create the story, then a story is created in
   the Jira project `DATA`, with the line's text as title, and the epic as its
   parent.
3. Given that creation, then the line records the created key and links to it on
   the tracker, and a second creation on that line is refused as today.
4. Given that creation, then the story does not enter the board, the counters or
   the epic's progress, and the creation's result says so.
5. Given the target `DATA`, when `DATA` was removed from the setting after the
   line was saved, then the creation is refused before anything is written, and
   the message says `DATA` is no longer a roadmap project.
6. Given Jira refusing the creation (missing mandatory field, no permission),
   then nothing is recorded on the line, and the message names the Jira error
   and the fields it asked for.
7. Given the story created but its parent refused, then the line records the
   key, and the result says the parent was not set on Jira.
8. Given no axis opt-in, then US4.2 behaves the same: the creation does not
   depend on it.
9. Given a project that is not a Jira project, then no declared key is offered.

### US5 (P2) - Opt in to writing the priority and the quarter on foreign epics

As a roadmap owner whose roadmap decides the order of another team's epics, I
write that decision on their epics, one epic at a time, once I chose to.

1. Given the project's tracker settings, then the axis opt-in is shown under the
   "Roadmap projects" field when at least one key is declared, and is closed on
   every existing and new project.
2. Given the axis opt-in closed, when I set the priority or the quarter of a
   foreign epic from its panel, then the value is stored and shown, no tracker
   write is queued, and the panel says the value is kept in Sectile and that the
   project's axis opt-in would write it on the tracker.
3. Given the axis opt-in open, when I set the priority of a foreign epic from its
   panel, then the epic receives the priority exactly as an own epic does
   (#627 US2), through one queued activity.
4. Given the axis opt-in open, when I set the quarter of a foreign epic from its
   panel, then US5.3 applies to the quarter (#627 US3).
5. Given the axis opt-in open, then the horizon, the free labels, the framing
   comment and every other field of a foreign epic stay unwritten.
6. Given the axis opt-in open, then the title seeding of the priority and the
   quarter (#627 US4), the pending pushes and every pass over several epics
   write nothing on a foreign epic.
7. Given the axis opt-in open and Jira refusing the write, then the value stays
   set in Sectile, and the activity names the epic and the Jira error.
8. Given the axis opt-in closed again, then later panel edits on foreign epics
   stay local; labels already written stay on the tracker.

## Functional requirements

- **FR1** Reading a Jira project's epics reads the own key, then each declared
  key through a request of its own. A declared key that fails does not stop the
  others; the read's summary names every failing key with its error. A failure
  of the own key fails the read as today.
- **FR2** The epics of a declared key are stored and read back like the own
  epics (title, status, closed state, labels, axes read from labels). No other
  work item of a declared key is imported.
- **FR3** An epic's origin is its key prefix. It is computed, never stored.
- **FR4** The roadmap offers an origin selection when the project declares at
  least one key or carries at least one foreign epic. It lists the own key
  first, then the declared keys in their declared order, then the other
  origins carried by stored epics in alphabetical order, each with the count of
  FR5.
- **FR5** The count of an origin is the number of its epics that the other
  active filters let through, across every horizon tab.
- **FR6** The selection defaults to the own key alone; an empty selection reads
  as the own key alone. It is remembered per project and per browser, and an
  origin that is no longer offered is dropped from it on read.
- **FR7** A non-default selection is shown as an active filter chip; clearing it
  returns to the default.
- **FR8** A foreign epic's row shows its origin, and its panel shows a read-only
  mark that names its origin.
- **FR9** The panel of a foreign epic offers no free label edit, no framing
  comment post and no tracker edit of the epic itself.
- **FR10** The horizon of a foreign epic is stored in Sectile only: no tracker
  write is queued for it, and none fails.
- **FR11** The priority and the quarter of a foreign epic are set from the panel
  and stored in Sectile. They are written on the tracker only when the axis
  opt-in is open, one epic per panel edit, through the same write as an own
  epic's axes.
- **FR12** No write covering several epics (pending pushes, title seeding
  confirmation, any batch pass) writes on a foreign epic, whatever the axis
  opt-in says, and none lists a foreign epic as pending.
- **FR13** The panel of a foreign epic says why its axes are kept in Sectile
  while the axis opt-in is closed, and where to open it.
- **FR14** The axis opt-in is a project setting, closed by default on existing
  and new projects, shown in the tracker settings of a Jira project under
  "Roadmap projects" when at least one key is declared.
- **FR15** A slicing line's target picker offers the declared keys after the
  Sectile projects, with a sublabel saying their stories stay in Jira. It offers
  none on a project that is not a Jira project.
- **FR16** Creating the story of a line aimed at a declared key rechecks the key
  against the current setting, creates the story in that Jira project with the
  line's text as title and the epic as parent, records the created key on the
  line, and imports nothing. The result says the story stays in Jira.
- **FR17** A refused creation records nothing on the line and names the Jira
  error; a refused parent keeps the key and says the parent was not set.
- **FR18** The help text of the "Roadmap projects" setting says what the setting
  now does: their epics are read into the roadmap, their stories attach to
  slicing lines, a slicing line can create its story there, and nothing of
  theirs is modified unless the axis opt-in is open.
- **FR19** Every new string exists in French and English.
- **FR20** `CHANGELOG.md` gets `Added` lines under `[Unreleased]` for the
  declared projects' epics in the roadmap with the origin selection, for the
  creation of a slicing line's story in a declared project, and for the axis
  opt-in.
- **FR21** An ADR records the read-only rule of declared projects as restated by
  the clarification: Sectile never changes an existing item of a declared
  project, except the priority and the quarter of one epic at a time when the
  project opted in; creating a new story there is allowed.

## Acceptance criteria

- **AC1** US1 to US5 scenarios pass on a Jira project declaring two keys, one of
  them unreachable.
- **AC2** After upgrading, every project behaves as before on its own epics, the
  axis opt-in is closed, and the roadmap shows the own key alone until the viewer
  changes the selection.
- **AC3** With the axis opt-in closed, no tracker request other than the epic
  reads and the story creation of US4 reaches a declared key in any scenario.
- **AC4** With the axis opt-in open, a panel edit of a foreign epic's priority or
  quarter produces exactly one queued tracker activity carrying only label
  changes, and no other write reaches a declared key.
- **AC5** A GitHub, GitLab or local project shows no change.
- **AC6** The web unit tests, the Go tests (SQLite and PostgreSQL) and the
  typecheck pass.

## Edge cases

- A foreign epic already stored before this change (attached through a story's
  parent) is grouped under its origin like any other.
- A declared key whose epics number in the thousands: the default selection
  keeps the roadmap on the own key.
- A key declared twice or in lowercase is normalized by the existing setting, so
  it is offered once.
- An own epic whose key is typed in lowercase in the tracker still reads as the
  own origin: the comparison ignores case.
- A line aimed at a declared key that later becomes a Sectile project of its own:
  the line keeps aiming at the declared key until someone changes it.
- The selection names an origin whose epics were all closed while closed epics
  are hidden: the origin is offered at 0 and stays selected.

## Open points

None. Every product question of the clarification is settled, including the one
raised during this specification (round 3).
