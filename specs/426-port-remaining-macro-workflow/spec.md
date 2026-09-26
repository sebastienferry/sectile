# #426: Port the remaining taskativ macro-workflow features

Ticket: https://github.com/sebastienferry/sectile/issues/426
Parent: M-7 "Ux improvements and fixes".
Branch: `feat/426`.
Clarification: [`docs/clarifications/426.md`](../../docs/clarifications/426.md)
(confirmed in Round 2; the specification-time decisions are recorded there under
Round 3).

## Context

PR #423 ported the first half of taskativ's macro workflow: the origin fields on
a slicing line (`sourceKind`, `sourceEntry`), the slicing import from `tasks.md`
and `spec.md`, and the `phase:` / `goal:` label axes. This ticket ports the rest
and closes the gaps #423 documents in place:

- a team whose specifications live apart from its code has nowhere to declare
  them;
- two macros specified at once share one checkout, and Git silently carries
  untracked specification files across a branch switch;
- once a slicing is edited by hand, nothing brings the specification back in
  line with it;
- a slicing line's target project is recorded but ignored, and a story created
  under a Jira epic gets its parent only on the local board;
- only the project's own Jira key attaches a story to a slicing line;
- Sectile shows the sprints of a Jira board and assigns tickets to them, but its
  timeline creates, renames, closes and deletes sprints only locally, where the
  next board sync overwrites them;
- the `scenarios` slicing source is declared but has no reader, and will not
  get one.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

## Terms

- **Code repository**: the project's `repoPath` (the agents' working
  directory), as today.
- **Specifications repository**: the checkout that carries the project's
  specifications. It is the project's declared specifications path when one is
  set, else the code repository.
- **Macro branch**: the branch of the specifications repository on which a
  macro's specification is written. It is the existing branch whose last path
  segment is the macro key or starts with `<KEY>-` (case-insensitive), else a
  new `<KEY>-<slug of the macro title>` branch.
- **Macro worktree**: the Git worktree of the specifications repository that
  holds the macro branch, at `.tasks/worktrees/<KEY>` in that repository.
- **Slicing line**: one todo of a macro (`MacroTodo`). Its origin is its
  `sourceKind` (`tasks`, `spec`, `stories`, or empty for a line typed by hand)
  and its `sourceEntry` (the entry title as the artifact writes it).
- **Specification entry**: a unit the slicing is imported from. Under Spec Kit,
  a numbered user story of `spec.md` or a `##` group of `tasks.md`; under
  OpenSpec, a `### Requirement:` of `specs/<capability>/spec.md` or a `##`
  group of `tasks.md`.
- **Same tracker instance**: two projects whose trackers are the same system at
  the same address, such that a parent link between their tickets can exist:
  - Jira: both projects on Jira with the same resolved Jira base URL
    (compared without scheme case, trailing slash or path differences);
  - GitHub: both projects on the same GitHub API URL **and** the same
    repository, since a milestone belongs to one repository;
  - local: both projects on the local board.
  Any other pair is not the same instance.
- **Roadmap projects**: additional Jira project keys a project declares as read
  sources for its slicing. They are read, never written.

## Decisions being specified

From the clarification (rounds 1 to 3), restated:

1. Everything ships from `feat/426` in a single pull request, one commit per
   item, in dependency order.
2. The specifications repository is an optional second path per project,
   falling back to the code repository.
3. One worktree per macro, in the specifications repository, on the macro
   branch, based on an up-to-date default branch, gated on `useWorktrees`.
4. `realign-macro` is a macro-scoped skill with Spec Kit and OpenSpec variants.
   It is surgical: it adds, renames and marks orphans as to be removed; it never
   touches an existing entry's body and never deletes. It writes only on the
   macro branch.
5. `realign-macro` is launched from a button in the macro panel through the
   desktop app's Run, **and** the macro worktree can be prepared through a new
   MCP tool, so the skill invoked by hand in an agent session reaches the same
   place (Round 3).
6. Sprint management writes to the tracker, synchronously, and mirrors the
   answer locally. It is delivered **on Jira only**: Sectile has no GitLab
   tracker, and GitLab iterations are a follow-up ticket that depends on one
   (Round 3; follow-up #430). Nothing on GitHub.
7. Sprint operations: batch creation (name pattern, count, start date, duration
   of 1 to 4 weeks), rename and dates, close, delete.
8. `TargetProjectID` is consumed: a story is created in the target project when
   it is on the same tracker instance as the macro; any other target is refused
   with a message naming why, and nothing is created.
9. A story created under a Jira epic, in the macro's project or in a target
   project, gets the epic as its parent **on Jira**, not only locally (Round 3).
10. Roadmap projects are extra Jira key prefixes, read-only.
11. The `scenarios` slicing source is removed from the code, not kept as a dead
    constant.

Out of scope: the EPIC template field map and anything built on it, a
`specify-macro` skill, sprints on GitHub, GitLab iterations (follow-up ticket #430),
a GitLab tracker adapter.

## User stories

### US1 (P1): Declare where the specifications live

As the owner of a project whose specifications live in another repository than
its code, I want to declare that repository once, so that the slicing, the macro
worktree and the realignment all read and write there while the agents keep
running in the code repository.

**Acceptance**

- **Given** a project with no specifications path, **when** its slicing is
  imported, **then** it is read from the code repository exactly as today.
- **Given** a project whose specifications path names another checkout that
  holds `specs/M-7-…/tasks.md`, **when** the slicing of M-7 is imported from
  `tasks`, **then** the lines come from that file, and the code repository is
  not read.
- **Given** a specifications path, **then** the agents' working directory is
  still the code repository; nothing else that uses `repoPath` changes.
- **Given** the project options, **then** the specifications path is editable
  next to the other repository settings, labelled in French ("Dépôt des
  spécifications"), with a hint that the code repository stays the agents'
  working directory; clearing it restores the fallback.
- **Given** no specification folder is found for a macro, **then** the refusal
  names the repository searched and says that the specifications repository can
  be declared in the project options.
- The path is stored trimmed; an empty or blank value means "not set".

### US2 (P1): One worktree per macro

As someone specifying two macros at once, I want each macro's specification to
live in its own worktree on its own branch, so that files of one never appear
in the other.

**Acceptance**

- **Given** `useWorktrees` on and no macro worktree for M-7, **when** the macro
  worktree of M-7 is prepared, **then** the specifications repository is
  fetched, and a worktree is created at `.tasks/worktrees/M-7` on the macro
  branch: the existing M-7 branch when there is one, else a new
  `M-7-<slug>` branch started from the remote default branch (the local
  default branch when there is no remote one).
- **Given** that worktree exists and holds uncommitted changes, **when** it is
  prepared again, **then** it is reused as is: never reset, never re-created,
  its changes kept.
- **Given** the macro branch is already checked out elsewhere (the main
  checkout or another worktree), **then** that checkout is returned rather than
  a second one being created.
- **Given** a path `.tasks/worktrees/M-7` that is not a valid worktree, **then**
  Git's stale record is pruned and the worktree is re-created when the directory
  is empty; a directory that still holds files is never deleted, and the
  preparation is refused naming it. A valid worktree of another branch at that
  path is refused the same way.
- **Given** macros M-7 and M-8 prepared one after the other, **then** they get
  two worktrees on two branches, and a file written in one never shows in the
  other.
- **Given** a fetch that fails (no network, no remote), **then** preparation
  continues from what is known locally and says the base may be stale.
- **Given** a workstation, **then** the specifications checkout the agent
  prepares the worktree in is the one declared for the project in the desktop
  app ("Specifications repository"), else the project's local checkout. The
  project's server-side specifications path is read by the server's slicing
  import only.
- **Given** `useWorktrees` off, **then** no worktree is created; preparation
  returns the specifications repository and the macro branch, and says that the
  checkout is used directly.
- **Given** the specifications repository is not a Git checkout, **then**
  preparation fails with a message naming the path; nothing is created.
- The worktree directory is excluded from the repository's status without
  editing its `.gitignore`.
- The macro branch is never the default branch.

### US3 (P1): Realign the specification with a hand-edited slicing

As a macro owner who edited the slicing by hand, I want to launch a
realignment that brings the specification back in line, adding, renaming and
flagging entries but never rewriting what the specification says, so that the
scenarios, decisions and prose it carries survive.

**Launch**

- **Given** a macro whose project has a desktop agent connected, **when** the
  owner clicks the realignment button of the macro panel ("Réaligner la spec"),
  **then** the desktop app runs `realign-macro` for that macro in the code
  repository, as it runs a task skill, and the macro shows the run as active
  until it ends.
- **Given** no connected agent, **then** the button is disabled and says why.
- **Given** a realignment already running for this macro, **then** a second
  launch is refused, even when both arrive at the same instant, and the panel
  offers to stop the running one. Stopping asks the owner's agent to end the
  process; an agent that cannot be reached closes the run only on an explicit
  confirmation, saying no local process was stopped.
- **Given** an agent session outside the desktop app, **when** the owner
  invokes `/realign-macro M-7`, **then** the skill prepares the macro worktree
  through the MCP tool and proceeds the same way.
- The skill is installed with the other skills, for the project's SDD framework
  (Spec Kit or OpenSpec), and appears in the skills list as a macro skill.

**Behaviour of the skill** (each case per slicing line, against the
specification entries of the macro folder on the macro branch):

- **Given** a line whose origin entry exists with the same text, **then**
  nothing is changed.
- **Given** a line whose origin entry exists and whose text differs, **then**
  the entry's title is renamed to the line's text and its body is left
  untouched.
- **Given** a line typed by hand (no origin), **then** a stub entry is added:
  under Spec Kit, a user story with the next free number (existing stories are
  never renumbered) or a `##` group in `tasks.md`; under OpenSpec, a
  requirement or a `##` group.
- **Given** an entry no line points to any more, **then** its title gets the
  suffix `(to be removed: no longer in the slicing)` and its body is kept;
  nothing is deleted.
- **Given** a line whose origin is `stories`, **then** it is left alone and
  reported, since it points at a ticket, not at an entry.
- **Given** no specification folder for the macro on the macro branch, **then**
  the skill stops, says so, and writes nothing.
- **Given** OpenSpec, **then** the skill runs `openspec validate <change>
  --strict` and fixes only what it reports about its own edits.
- The skill writes only in the macro worktree (or, with worktrees off, in a
  specifications checkout that is on the macro branch; otherwise it stops). It
  never switches a checkout's branch, never writes on the default branch, never
  saves the slicing, never creates a story or a pull request.
- When it wrote something, it commits on the macro branch and pushes it; when it
  wrote nothing, it pushes nothing.
- Its report lists the entries added, renamed (old → new), marked, left alone
  and why, the files touched and the branch.

### US4 (P2): Create a slicing line's story in its target project

As a macro owner whose stories belong to several projects of one Jira instance,
I want each line's story created in its target project under the macro's epic,
so that the epic gathers the work wherever it lands.

**Acceptance**

- **Given** a line with no target project, **when** its story is created,
  **then** it is created in the macro's project, as today.
- **Given** a Jira macro and a line whose target is another project of the same
  Jira instance, **when** its story is created, **then** it is created in the
  target project, its key is recorded on the line, and on Jira its parent is the
  macro's epic.
- **Given** a Jira macro and a line with no target, **when** its story is
  created, **then** on Jira its parent is the macro's epic (today it is only
  local).
- **Given** a target on another tracker kind, another Jira instance, or another
  GitHub repository, **when** the story is created, **then** it is refused with
  a French message naming the target project and the reason (other tracker,
  other instance, other repository); no ticket is created and the line is
  unchanged.
- **Given** a target project that no longer exists, **then** it is refused with
  a message naming the missing project; nothing is created.
- **Given** the story is created but writing its Jira parent fails, **then** the
  story key is still recorded on the line, and the outcome says that the parent
  was not written and why.
- **Given** a GitHub macro, **then** the milestone is set as today.
- **Given** a slicing line without a story, **then** a project picker on the
  line offers the macro's project and every project on the same tracker
  instance; choosing one saves it as the line's target, and choosing the
  macro's project clears it. A line that already has a story shows its target
  read-only.
- **Given** a line whose saved target is no longer on the same tracker instance,
  **then** the picker shows it flagged as invalid, and creating the story is
  refused as above.

### US5 (P2): Attach stories from declared roadmap projects

As the owner of a Jira project whose roadmap also reads other Jira projects, I
want a story key from those projects to attach to a slicing line on import, so
that the slicing shows the work they already carry.

**Acceptance**

- **Given** roadmap projects `ABC, def` on project `SFE`, **then** they are
  stored as `ABC`, `DEF`: upper-cased, split on commas and blanks, deduplicated,
  and without the project's own key.
- **Given** a `tasks.md` entry titled `ABC-12 Do the thing`, **when** the
  slicing is imported, **then** the line is attached to `ABC-12`, as it would be
  to `SFE-12`.
- **Given** an entry titled with a key of an undeclared project, **then** the
  key is not attached, as today.
- **Given** a line attached to a roadmap project's story, **then** Sectile never
  writes to that project from the slicing: no story creation, no parent, no
  sprint move, no label.
- **Given** the project options of a Jira project, **then** the roadmap projects
  are editable as a comma-separated list, labelled in French; the field is not
  offered on other trackers.

### US6 (P2): Manage a Jira board's sprints from the timeline

As a team lead on Jira, I want to create, rename, re-date, close and delete
sprints from Sectile's timeline, and have Jira hold them, so that tickets can be
moved to them and the next sync keeps them.

**Acceptance**

- **Given** a Jira project with a board, **when** the owner creates 3 sprints
  with pattern `Sprint {n}`, start `2026-10-05` and 2 weeks, **then** Jira
  receives `Sprint 1` starting 2026-10-05 at 09:00, `Sprint 2` starting
  2026-10-19 at 09:00 and `Sprint 3` starting 2026-11-02 at 09:00, each ending
  one second before the next one starts (the last on 2026-11-16 at 08:59:59),
  and the timeline shows the three sprints with their Jira ids.
- A pattern without `{n}` names a single sprint as typed and numbers a batch
  `"<pattern> 1"`, `"<pattern> 2"`, …; an empty pattern defaults to
  `Sprint <start date>`.
- The count is 1 to 12 and the duration 1 to 4 weeks; other values are refused
  before any write.
- **Given** a name that already exists on the board (ignoring case and
  surrounding blanks), **then** the batch is refused before any write, naming
  the taken name and suggesting another.
- **Given** the third creation of a batch fails, **then** the first two are
  kept and shown, and the outcome says "2 sprints created, then failed on
  Sprint 3" with Jira's reason.
- **Given** a sprint, **when** it is renamed or its dates changed, **then** Jira
  holds the new values and the timeline shows Jira's answer.
- **Given** an active sprint with unfinished tickets, **when** it is closed with
  "move to the next sprint" (or "to the backlog"), **then** its unfinished
  tickets are moved there on Jira first, then the sprint is closed on Jira.
- **Given** Jira refuses an operation (closing a sprint that never started,
  deleting an active one, missing permission), **then** Jira's reason is shown
  and the local copy is unchanged.
- **Given** a sprint, **when** it is deleted after confirmation, **then** it is
  deleted on Jira, then locally; a sprint Jira no longer knows counts as
  deleted.
- **Given** a Jira project without a board, **then** the sprint controls say a
  board must be chosen in the project options first.
- **Given** a ticket moved to a sprint from the timeline, **then** the move
  uses the sprint's Jira id, never its name.
- **Given** a sync after any of these operations, **then** the timeline still
  shows the same sprints.
- **Given** a GitHub project, **then** the timeline offers no sprint creation,
  edition, closing or deletion. **Given** a local project, **then** the
  timeline behaves as today.
- **Given** a GitHub project that already holds sprints stored locally, **then**
  the timeline still shows them, read-only: no edit, close, delete, and no
  ticket move to them.

### US7 (P3): The `scenarios` source is gone

As a maintainer, I want the slicing source that has no reader removed, so that
the code no longer promises it.

**Acceptance**

- `models.MacroTodoFromScenarios` and `'scenarios'` in the web
  `MacroTodoSource` type no longer exist, and no comment counts three
  repository or tracker sources.
- **Given** a line saved with `sourceKind: "scenarios"` (none is expected),
  **then** it is loaded and saved back unchanged, like any origin this version
  does not know (the forward-compatibility rule of the slicing), and no reader
  or button offers `scenarios`.

## Functional requirements

- **FR1** A project has an optional specifications path; the specifications
  repository is that path when set, else the code repository. Every macro
  specification read or write (slicing import, macro worktree, realignment)
  uses it; nothing else does.
- **FR2** Preparing a macro worktree follows US2: fetch, reuse, re-create an
  invalid one, never reset, base on the remote default branch, gated on
  `useWorktrees`, never on the default branch.
- **FR3** Preparation is available to the web client (for the Run launch) and
  as an MCP tool taking the macro key and its project, returning the path, the
  branch, whether a worktree is used, and any warning.
- **FR4** Concurrent preparations of the same macro are serialised; they return
  the same worktree.
- **FR5** `realign-macro` is a macro-scoped skill, rendered for Spec Kit and
  OpenSpec, obeying the rules of US3, launched from the macro panel through the
  desktop Run or invoked by hand.
- **FR6** A macro skill run is visible on the macro while it runs and records
  its outcome; it does not change any task's stage.
- **FR7** Story creation from a slicing line uses the line's target project
  when set, refuses a target not on the same tracker instance (Terms) before
  any write, and on Jira writes the epic as parent of the created story.
- **FR8** Roadmap projects are a normalised list of Jira keys on the project.
  They widen the keys a specification entry may carry to attach to a line, and
  are never the target of a write.
- **FR9** On a Jira project with a board, sprint creation, update, closing and
  deletion call Jira synchronously and mirror Jira's answer in the project's
  sprints; a failure leaves the local copy as Jira left it.
- **FR10** Ticket moves to a sprint on Jira use the sprint id.
- **FR11** The `scenarios` source is removed from the Go model and the web type.
- **FR12** New user-facing strings are in French; code, comments and docs in
  English. The changelog gets one line per user-visible item under
  `[Unreleased]`.

## Success criteria

- Automated tests cover US1 to US7 as listed in [`tasks.md`](tasks.md), with
  Git fixtures for the worktree cases and a fake Jira for sprints and parents.
- The existing slicing, macro and sprint tests pass.
- Manual, after the merge: the owner declares a specifications repository on a
  test project, launches "Réaligner la spec" on a macro with a hand-edited
  slicing, and finds the entries added, renamed and marked on the macro branch,
  the bodies unchanged.

## Open requirements

None. O1 (target project picker) and O2 (local sprints of GitHub projects,
kept visible read-only) were settled by the owner after the first draft; see
Round 4 of the clarification.
