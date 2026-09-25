# Specification #487 - Drop the specification artefacts

- Ticket: https://github.com/sebastienferry/sectile/issues/487
- Branch: `feat/487`
- Clarification: `docs/clarifications/487.md` (rounds 1 to 3, confirmed by the
  owner), plus one decision taken during specification (FR9)
- Framework: Spec Kit

## Summary

A project can choose to keep the specification artefacts of its tasks out of
its repository. When it does, the clarify and specify stages still write their
files in the task worktree, but Git ignores them through a block that Sectile
maintains in the checkout's local exclude file: nothing is committed, nothing
is pushed, and the pull request carries code only. The choice is a server
project setting, off by default, that each workstation may override. Dropped
artefacts live as long as the worktree; the stage reports posted on the ticket
are the durable trace.

## Scope

In scope: the task stages (clarify, specify, implement, and the pickup skills
that embed them), the server project setting, the desktop override, and the
local exclude block.

Out of scope:

- The macro workflow and its specifications folder (#443).
- The choice of SDD framework and the layout of the specification files.
- Rewriting history: specifications already committed stay where they are.
- Preserving dropped artefacts anywhere else than the worktree.

## Definitions

- **Task artefacts**: the files a task's stages write to describe it, and
  nothing else:
  - `specs/<K>-<slug>/` (Spec Kit),
  - `openspec/changes/<K>-<slug>/` (OpenSpec),
  - `docs/clarifications/<K>.md`,

  where `<K>` is the task key without its leading `#` (`487` for `#487`,
  `SFE-12` for a Jira key).
- **Keep** (default): task artefacts are committed on the task branch and
  reviewed with the pull request, as today.
- **Drop**: task artefacts are written in the worktree, ignored by Git, and
  never committed or pushed.
- **Server setting**: the project's choice, the same for every workstation.
- **Workstation override**: this workstation's choice for one project, one of
  *follow the server* (nothing stored), *keep*, *drop*.
- **Effective value**: the workstation override when one is stored, else the
  server setting.
- **Exclude block**: the lines Sectile writes, between two marker lines naming
  the project, in the local exclude file of a checkout (shared by all of its
  worktrees and never part of the repository).

## User stories (prioritised)

### US1 - A project keeps its specifications out of the repository (P1)

As a project owner, I turn on one option in the web project options and my
tasks' clarifications and specifications stop appearing in their branches and
pull requests.

**Acceptance**

- **Given** the web project options, **then** the "Local agent execution
  defaults" panel shows an option to keep specifications out of the
  repository, unchecked for every existing and new project.
- **Given** the option is checked and saved, **when** the project is read
  back through the web or the API, **then** the server setting is *drop*; unchecking and saving makes it *keep* again.
- **Given** an API client that omits the field on project update, **then** the
  stored value is unchanged.
- **Given** an API client that sends a value other than `keep` or `drop`,
  **then** the request is refused with a message naming the accepted values.

### US2 - Clarify and specify write without committing (P1)

**Acceptance**

- **Given** a project whose effective value on this workstation is *drop*,
  **when** a task stage is launched there (or its branch is checked out from
  the board), **then** before the agent session starts the checkout's exclude
  block contains the rules that ignore that task's artefacts.
- **Given** that block, **when** the clarify stage runs, **then**
  `docs/clarifications/<K>.md` exists in the worktree, no commit contains it,
  and the clarified report on the ticket carries the settled decisions in full
  and says the report file is local to the worktree.
- **Given** that block, **when** the specify stage runs, **then** the
  specification files exist in the worktree, no commit contains them, and the
  specified report on the ticket carries the requirements, the open points and
  says the files are local to the worktree.
- **Given** dropped artefacts in a worktree, **then** `git status --porcelain`
  does not list them and `transition_stage` does not refuse the checkout as
  dirty because of them.
- **Given** dropped artefacts, **when** the implement stage runs, **then** it
  reads the specification from the worktree, and neither it nor the adjust
  stage commits the artefacts (no forced add).
- **Given** a task of a project whose effective value is *drop*, **when** its
  artefacts are missing from the worktree (another workstation, or a worktree
  recreated), **then** the implement stage stops and reports that the
  specification is not available on this workstation rather than rewriting
  it.
- **Given** a standalone run (a stage typed in a terminal, not launched by
  Sectile) in a checkout whose block already covers the task, **then** the
  skill behaves as in a launched run: it decides from Git whether the artefact
  path is ignored.

### US3 - A workstation overrides the project's choice (P2)

**Acceptance**

- **Given** the desktop project settings, **then** a "Specifications" row
  offers *Keep* and *Drop*, with a reset control to follow the server, and a
  hint that reads "Inherited · Server default: Keep" (or "Drop"), or "Local
  override · Server default: …" once a value is chosen.
- **Given** no override, **when** the server setting changes, **then** the
  next launch on this workstation follows the new server value.
- **Given** an override *drop* on a project whose server setting is *keep*,
  **then** launches on this workstation drop the artefacts and launches on
  other workstations keep them.
- **Given** an override, **when** the user resets the row and saves, **then**
  no override is stored for that project any more.
- **Given** the settings file of a workstation that never used the override,
  **then** it gains no new key until an override is saved, and removing the
  last override leaves no empty map behind.

### US4 - Turning the option off removes only Sectile's rules (P2)

**Acceptance**

- **Given** an exclude block for a project and lines written by the user above
  and below it, **when** the effective value becomes *keep* and a launch runs
  (or the desktop settings are saved with *keep*), **then** the block,
  markers included, is removed from every checkout this workstation maps for
  the project, and every other line of the file is left byte for byte.
- **Given** two projects that share a checkout, each with its own block,
  **when** one project turns the option off, **then** the other project's
  block stays.
- **Given** a web change from *drop* to *keep*, **then** each workstation
  removes its block at its next launch for that project, or when its desktop
  settings for the project are saved.
- **Given** artefacts of an in-flight task that were dropped, **when** the
  block is removed, **then** they are left on disk untouched and appear as
  untracked files; Sectile deletes nothing.

### US5 - A repository that already tracks specifications (P3)

**Acceptance**

- **Given** a checkout that already tracks files under `specs/`,
  `openspec/changes/` or `docs/clarifications/`, **when** the desktop project
  settings show an effective value of *drop*, **then** the row shows the
  warning: "This repository already tracks N specification files. They stay
  in its history; only the next tasks' specifications are dropped."
- **Given** the web project options, **then** the option's help text says
  that specifications already committed stay in the history.
- **Given** such a repository and the option on, **then** Sectile changes no
  tracked file and rewrites no history; tracked specifications of other tasks
  are unaffected because the rules name only each task's own artefacts.

### US6 - Pull request at the specified stage (P3)

**Acceptance**

- **Given** a project that creates its pull request after specification and
  an effective value of *drop* on the workstation running specify, **when**
  the specify stage completes, **then** it opens no pull request, its report
  says the pull request is deferred to the implemented stage, and the
  transition to *specified* is accepted without a pull request.
- **Given** the same project, **when** the implement stage completes, **then**
  it creates (or reuses) the draft pull request and the implemented transition
  requires it, as for a project that creates its pull request after
  implementation.
- **Given** the same project and an effective value of *keep*, **then**
  nothing changes: specify still opens the draft pull request and the
  specified transition still requires it.

## Functional requirements

- **FR1 - Server setting.** Projects carry a specification artefacts setting,
  `keep` or `drop`, `keep` by default and for every existing project after
  upgrade. It is edited in the web project options and exposed to local
  agents with the rest of the project's execution configuration.
- **FR2 - Workstation override.** The desktop project settings store an
  optional override per project, `keep` or `drop`. No stored override means
  following the server. The override never leaves the workstation.
- **FR3 - Effective value.** Everywhere the agent acts on the option, it uses
  the override when present, else the server setting.
- **FR4 - Rules per task.** With an effective value of *drop*, each launch of
  a task stage and each board checkout of a task branch ensures that the
  primary checkout's exclude block for the project contains the rules for
  that task's artefacts under both frameworks. Rules for other tasks already
  in the block stay. A task key that cannot be written safely as an ignore
  pattern gets no rule, and the launch proceeds as *keep* with a log line.
- **FR5 - Removing the block.** With an effective value of *keep*, the same
  moments, and saving the desktop project settings, remove the project's
  block from the checkouts concerned. Nothing outside the markers is ever
  modified. The file is rewritten atomically.
- **FR6 - Skills decide from Git.** The clarify, specify and implement stage
  bodies (and the pickup skills that embed them) tell the agent to check
  whether each artefact path is ignored by Git before committing it; an
  ignored artefact is written and read in the worktree, never committed,
  never force-added, and the report says it stays local.
- **FR7 - Launch notice.** A launched stage whose effective value is *drop*
  receives one extra line in its prompt stating that the task's specification
  artefacts are dropped on this workstation.
- **FR8 - Durable trace.** Under *drop*, the clarified and specified reports
  carry the substance of the artefact (settled decisions; requirements and
  open points), since the file will not survive the worktree.
- **FR9 - Pull request at specified.** When the project creates its pull
  request after specification and the effective value on the reporting
  workstation is *drop*, the specified stage opens none and its transition
  does not require one; the implemented stage creates it and requires it.
  An agent that cannot report its effective value leaves the requirement as it
  is today.
- **FR10 - Tracked warning.** The desktop project settings count the files the
  checkout already tracks under `specs/`, `openspec/changes/` and
  `docs/clarifications/`, and show the US5 warning when the effective value is
  *drop* and the count is not zero.
- **FR11 - Lifetime.** Dropped artefacts are deleted with the worktree at
  handoff cleanup, as ignored files are today. Sectile preserves them nowhere.
- **FR12 - Changelog.** `CHANGELOG.md` gains an `Added` line under
  `[Unreleased]` describing the option for users.

## Edge cases

- **No worktrees** (the project works in the main checkout): the block is
  written in that checkout's exclude file; behaviour is otherwise identical.
- **Multi-repo project**: the rules go to the checkout of the task's primary
  repository, where the stages write their artefacts. Context repositories get
  no rule.
- **Framework switched mid-task**: rules cover both frameworks' folders, so
  the switch does not expose the artefacts.
- **Key case**: when `<K>` contains upper-case letters, the lower-case form is
  written too, since some tools lower-case change identifiers.
- **Stale rules**: rules of finished tasks stay in the block until the option
  is turned off; they only match those tasks' own paths.
- **Old agent**: an agent that predates the feature ignores the setting and
  keeps committing, and FR9 falls back to requiring the pull request.

## Decisions taken during specification

- **FR9** (asked to the owner on 2026-09-25): with *drop* and a pull request
  created after specification, the pull request is deferred to the
  implemented stage. Rejected: forbidding the combination in the settings, an
  empty commit to open the pull request.
- The exact patterns, the moments the block is written and removed, how the
  skills learn the value, and the warning's wording and placement were left to
  the specification by the clarification and are fixed above.

## Open points

None blocking. The desktop warning (FR10) and the web help text are the only
places the "already tracked" situation is surfaced; the web cannot count
tracked files because the server has no checkout.
