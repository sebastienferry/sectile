# Spec #755 - Desktop: archiving a task removes its worktree

- Ticket: [#755](https://github.com/sebastienferry/sectile/issues/755)
- Clarification: `docs/clarifications/755.md` (rounds 1 and 2, settled 2026-10-06)

## Context

Archiving a task in the Sectile Desktop sidebar ("Archive <task>", or "Stop
and archive <task>" when it has active executions) only hides its runs. The
task's worktrees stay under `.tasks/worktrees/` on the workstation, and so does
its local branch, long after the pull request is merged.

## User stories

### US1 (P1) - Archiving cleans the task's worktrees

As a Desktop user, when I archive a ticket task, its worktrees are removed from
my workstation, so archived work leaves no folder behind.

**Acceptance**

1. **Given** a ticket task whose code worktree is clean, **when** I archive
   it, **then** the worktree is removed and the task's runs are hidden.
2. **Given** a ticket that changed several repositories (#456), **when** I
   archive it, **then** its worktree in each of them is removed.
3. **Given** a project with a distinct specifications folder (#736) where the
   task has a specifications worktree, **when** I archive it, **then** that
   worktree is removed too.
4. **Given** a task with no worktree on disk (never launched here, or already
   removed), **when** I archive it, **then** it is archived at once.
5. **Given** a task with active executions, **when** I choose "Stop and
   archive", **then** the executions stop first, then the worktrees are
   removed, then the runs are hidden.

### US2 (P1) - Nothing unsaved is ever lost

As a Desktop user, archiving never destroys uncommitted work and never
archives a task whose worktrees could not be checked.

**Acceptance**

1. **Given** a worktree with uncommitted or untracked changes, **when** I
   archive the task, **then** the archive is refused, the worktree and its
   changes stay, the runs stay visible, and a dialog names the repository,
   Git's reason, and asks me to commit or discard the changes.
2. **Given** the local agent is stopped, unreachable, or too old to clean a
   worktree, **when** I archive a ticket task, **then** the archive is refused
   and the dialog tells me to start or update the agent.
3. **Given** a repository of the ticket that is not found on this
   workstation, **when** I archive, **then** the archive is refused and the
   dialog names that repository.
4. **Given** one repository's worktree is removed and another's is refused,
   **when** I archive, **then** the archive is refused, the removed one stays
   removed, and a later archive, once the other is clean, succeeds.
5. **Given** "Stop and archive" whose removal is refused, **then** the
   executions stay stopped, the dialog stays open with the reason, and I can
   archive again once the worktree is clean.

### US3 (P2) - The local branch goes when its work is merged

As a Desktop user, archiving a task whose pull requests are all merged also
deletes its local branch, and keeps it in every other case.

**Acceptance**

1. **Given** every pull request recorded on the task is merged and the local
   branch has no commit missing from its upstream, **when** the archive
   removes the worktree, **then** the local branch is deleted, even though a
   squash merge leaves it "unmerged" for Git.
2. **Given** a task with no pull request, an open, closed-unmerged or unknown
   one, **then** the local branch is kept.
3. **Given** a local branch with no upstream, or with commits its upstream does
   not have, **then** it is kept.
4. **Given** a kept branch, **then** the archive still succeeds: a kept branch
   is never an error.

### US4 (P2) - What archiving never touches

**Acceptance**

1. **Given** a free console or a macro run, **when** I archive it, **then** it
   is archived as today, with no call to the agent.
2. **Given** a ticket of a batch whose worktree (same branch) is shared with
   another ticket of the project, **when** I archive it, **then** the shared
   worktree and branch are kept and the archive succeeds.
3. **Given** worktrees are off for the project, **when** I archive, **then**
   nothing is removed and the archive succeeds; the main checkout is never
   touched.
4. Context folders (attached repositories), remote branches, pull requests and
   the tracker are never changed by an archive.

### US5 (P3) - The archive control says what it does

**Acceptance**

1. **Given** a ticket task row, **then** its archive button reads "Archive
   <task> and remove its worktree" ("Stop, archive <task> and remove its
   worktree" with active executions). Free consoles and macro runs keep
   "Archive <name>".

## Functional requirements

- **FR-001** Archiving a ticket task asks the local agent, through the server,
  to clean the task's worktrees before the runs are hidden. There is no option
  to skip it.
- **FR-002** A worktree is removed with a plain `git worktree remove`, never
  `--force`.
- **FR-003** The archive happens only when every worktree of the task is
  removed, absent, shared, or out of scope (worktrees off). Any other outcome,
  or any failure to reach the server or the agent, refuses it.
- **FR-004** A refusal shows each repository that blocked, the reason, and the
  action to take; the runs stay visible.
- **FR-005** The local branch is deleted (`git branch -D`) in a repository
  only when the task records at least one pull request, all of them are
  merged, its worktree in that repository was removed or absent, the branch is
  checked out nowhere, it has an upstream, and it has no commit its upstream
  lacks. Otherwise it is kept, and keeping it never blocks the archive.
- **FR-006** A worktree on a branch another task of the project records is
  shared: it is neither removed nor blocking, and its branch is kept.
- **FR-007** Free consoles and macro runs archive as today.
- **FR-008** An agent or a server too old to clean a worktree on archive is
  reported as such ("update the agent" / "update the server").
- **FR-009** New user-facing strings are in English.
- **FR-010** `CHANGELOG.md` gains a line under `## [Unreleased]` / `Changed`.

## Out of scope

- Un-archiving: the next launch recreates the worktree from its branch.
- A forced removal of a dirty worktree.
- Remote branches, pull requests, tracker state, the web board, the handoff
  skill's own clean-up.

## Open points

None. Pull request states are those the server last refreshed; a state not yet
refreshed to "merged" keeps the branch, which is the safe side.
