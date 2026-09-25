# #456: Run each task of a multi-repo project in its own repositories

Ticket: https://github.com/sebastienferry/sectile/issues/456
Branch: `feat/456`.
Clarification: [`docs/clarifications/456.md`](../../docs/clarifications/456.md) (confirmed in Round 2).

## Context

A run gets exactly one working directory today: the project root mapped on the
workstation. On a project whose tickets span several repositories, nothing says
which repository a task belongs to, the agent cannot see the sibling
repositories, and a task that must change two repositories has nowhere to do it.
The per-task working directory that the task detail already lets users edit is
ignored by the local agent, so a task pinned to another repository still runs in
the project root.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

Out of scope: per-task selection of context folders, an enforced read-only
guarantee on context folders, additional-directory support for providers whose
flag is not attested, merging the specifications repository into the project's
repositories, and forges other than GitHub and GitLab.

## Terms

- **Repository**: a git repository identified by its remote, compared as
  `host/path`, case-insensitively, without scheme, user, port or `.git` suffix,
  so that the SSH and HTTPS forms of one remote are the same repository.
- **Project repositories**: the ordered list of repositories a project declares.
  The project's code remote, when it has one, is always the first.
- **Mapping**: on one workstation, the local folder that holds a checkout of a
  repository. Mappings never leave the workstation.
- **Primary repository** of a task: the repository the task is pinned to, or the
  project's single repository when there is only one.
- **Context folder**: a mapped project repository that is not the task's
  primary repository and in which the task has no worktree. The agent reads it
  as it is checked out on the workstation.
- **Changed repository**: a repository in which the task has a worktree on its
  branch. The primary repository is always one.
- **Folder map**: the description of the task's repositories given to the agent
  at launch.
- **Ambiguous task**: a task of a multi-repo project (`monoRepo=false`) with no
  pinned repository, when more than one project repository is mapped on the
  workstation.

## Decisions being specified

From the clarification, restated:

1. Repositories are identified by remote; mappings are per workstation.
2. Existing per-task paths and the project's list of known paths are converted
   to repositories; paths that cannot be resolved are dropped and reported.
3. Context folders are declared per project: every mapped repository other than
   the primary one.
4. Read-only on context folders is an instruction to the agent, not enforced.
5. The specifications repository stays separate and is listed in the folder map.
6. A task may change several repositories. A worktree in a secondary repository
   is created on demand, by the local agent, on the task's branch.
7. The repository is chosen before the agent CLI starts, by the launcher, and
   only for an ambiguous task. Skills never choose it.
8. Every changed repository needs its own pull request, checked at the
   transitions that require pull request evidence.
9. Handoff removes the task's worktrees in every changed repository.

## User stories

### US1 (P1): A project declares its repositories and each workstation maps them

As the owner of a multi-repo project, I want to list the project's repositories
once and map each of them to a folder on my workstation, so that every task can
find the code it needs without anyone typing a path into a ticket.

**Acceptance**

- **Given** a project, **when** its owner adds a repository by remote URL,
  **then** the project lists it, and two spellings of one remote (SSH and HTTPS)
  are refused as a duplicate.
- **Given** a project with a code remote, **then** that remote is listed first
  and cannot be removed from the list while it is the code remote.
- **Given** a project with several repositories, **when** the user opens the
  project's local settings in the desktop, **then** each repository is shown with
  its mapped folder or as not mapped, and a folder can be chosen for each.
- **Given** a folder is chosen for a repository, **when** its checkout's
  `origin` is not that repository, **then** the mapping is refused with a message
  naming both remotes.
- **Given** a mapping, **then** it is stored on the workstation only and is
  never sent to the server.
- **Given** one repository is used by two projects, **then** mapping it once
  serves both.

### US2 (P1): Existing per-task paths become repositories

As a user of a project that already pinned working directories on tasks, I want
them converted, so that nothing I pinned is silently lost or silently wrong.

**Acceptance**

- **Given** a task whose working directory is a checkout on the migrating
  workstation, **when** the conversion runs, **then** the task is pinned to that
  checkout's `origin` repository, and that repository is added to the project's
  repositories.
- **Given** a path from the project's list of known paths that resolves, **then**
  its repository is added to the project's repositories.
- **Given** a path that does not exist on the migrating workstation, is not a git
  checkout, or has no `origin`, **then** it is dropped, and the conversion
  reports each dropped path with its reason and the tasks that referenced it.
- **Given** the conversion has run for a project, **then** it does not run again
  for that project, on this workstation or any other.

### US3 (P1): A pinned task runs in its repository

As a user, I want a task pinned to a repository to run in that repository on
every stage, so that clarify, specify, implement and adjust all see the same code.

**Acceptance**

- **Given** a task pinned to a mapped repository that is not the project's code
  remote, **when** any stage is launched, interactive or autonomous, **then** the
  task worktree is created or reused in that repository, and the agent starts
  there.
- **Given** a pinned repository that is not mapped on this workstation, **when**
  a stage is launched, **then** the launch does not start and says which
  repository must be mapped, and where.
- **Given** a mono-repo project, or a project with a single repository, **then**
  launching is unchanged.

### US4 (P1): An ambiguous task asks which repository it belongs to

As a user launching an ambiguous task, I want to be asked for its repository
before the agent starts, so that the agent never works in the wrong one.

**Acceptance**

- **Given** an ambiguous task, **when** a stage is launched interactively,
  **then** the agent CLI does not start, the launch shows the mapped project
  repositories to choose from, and choosing one pins the task and starts the
  stage in that repository.
- **Given** that choice, **when** a repository is listed but not mapped, **then**
  the user can map it to a folder from the same choice, with the checks of US1.
- **Given** an ambiguous task, **when** a stage is launched autonomously,
  **then** the agent CLI does not start, the run shows it is waiting for input
  with a message asking to pin the task's repository, and nothing is guessed.
- **Given** a run waiting for its repository, **when** the task is pinned to a
  mapped repository from any surface, **then** the same run leaves the waiting
  state and starts in that repository.
- **Given** a run waiting for its repository, **when** the user cancels it,
  **then** it ends as canceled and nothing was started.
- **Given** a task that is not ambiguous, **then** nobody is asked anything.

### US5 (P1): The agent knows every folder of the task

As an agent working on a task, I want to know where each repository of the
project lives, which one I work in and which ones I only read, so that I can
use sibling code without editing it by accident.

**Acceptance**

- **Given** a launch, **then** the agent receives the folder map: for each project
  repository and for the specifications repository, its remote, its role
  (primary, context, spec or changed), its local folder or that it is not mapped
  here, and its worktree when the task has one there.
- **Given** a provider whose additional-directory flag is attested (Claude Code),
  **then** every mapped context folder and the specifications folder are passed
  to it as additional directories, interactive and autonomous alike.
- **Given** another provider, **then** no flag is guessed and the folder map is
  still given.
- **Given** context folders, **then** the agent is told they are read-only and
  that changing one requires asking for a worktree in it first (US6).
- **Given** a mono-repo project with a single repository and no specifications
  repository, **then** the launch is unchanged apart from the folder map.

### US6 (P1): A task changes a second repository

As an agent whose task must change a context repository, I want a worktree on
the task branch in that repository, so that the change can become a pull request
like any other.

**Acceptance**

- **Given** a running task and a mapped project repository, **when** the skill
  asks for a worktree in it, **then** the local agent of that workstation creates
  or reuses the task's worktree there on the task's branch, answers its path, and
  that repository becomes a changed repository of the task.
- **Given** the task branch already exists in that repository, locally or on its
  remote, **then** it is reused, not recreated.
- **Given** a repository that is not in the project's repositories, **then** the
  request is refused.
- **Given** a repository that is not mapped on that workstation, **then** the
  request is refused with the message of US3.
- **Given** a mono-repo project, **then** the request is refused.
- **Given** the same request twice, **then** the second answers the same
  worktree.

### US7 (P1): Every changed repository has its pull request

As the owner, I want the stages that require pull request evidence to check the
pull request of every repository the task changed, so that no change is left
without review.

**Acceptance**

- **Given** a task with two changed repositories, **when** `implemented` is
  recorded with a pull request for each, **then** each is checked with today's
  evidence rules against its own repository and worktree head, and both are
  recorded on the task.
- **Given** a changed repository without a pull request, **when** a transition
  requiring pull request evidence is recorded, **then** it is refused and names
  that repository.
- **Given** a pull request whose head differs from its worktree head, **then**
  the transition is refused and names that repository.
- **Given** a pull request from a repository that is not a changed repository of
  the task, **then** it is refused.
- **Given** `adjust`, **then** the launch prerequisite checks every changed
  repository, each on a clean worktree.
- **Given** a task with a single changed repository, **then** the transitions
  behave as today.

### US8 (P2): Handoff cleans every repository

**Acceptance**

- **Given** a task with worktrees in several repositories, **when** its worktree
  is removed at handoff, **then** the worktree is removed in every changed
  repository mapped on that workstation, and a repository that could not be
  cleaned is reported by name.

## Functional requirements

- **FR1** Repository identity is the normalised `host/path` of Terms; every
  comparison of repositories uses it.
- **FR2** A project stores its repositories as an ordered list of remotes; the
  code remote, when set, is first. Duplicates by identity are refused (US1).
- **FR3** A task stores at most one pinned repository, which must be one of the
  project's repositories, and the list of its changed repositories.
- **FR4** Mappings are workstation settings keyed by repository identity,
  validated against the checkout's `origin`, never uploaded (US1).
- **FR5** The conversion of US2 runs once per project, on the first local agent
  that sees the project after the upgrade, and reports what it dropped.
- **FR6** The primary repository is resolved before the agent CLI starts: the
  pinned repository, else the single project repository, else the project's
  code remote on a mono-repo project; an ambiguous task waits (US3, US4).
- **FR7** A launch waiting for its repository is a run in the waiting-for-input
  state: it starts nothing, resumes on the same run when the task is pinned, and
  ends as canceled when canceled (US4).
- **FR8** The folder map is built by the local agent and never returned by the
  server (US5).
- **FR9** Additional directories are passed only with a flag the repository
  attests for the provider; for others, none (US5).
- **FR10** A worktree in a secondary repository is created only on request, by
  the local agent, on the task branch, for a project repository mapped on the
  requesting workstation of a multi-repo project (US6).
- **FR11** The transitions requiring pull request evidence, the agent post-back
  and the adjust launch prerequisite require one accepted pull request per
  changed repository, each checked with the rules of #392 against that
  repository (US7).
- **FR12** Worktree removal covers every changed repository (US8).
- **FR13** A user-visible `Added` entry is written under `[Unreleased]` in
  `CHANGELOG.md`.

## Success criteria

- A multi-repo project with two repositories runs a task end to end in the
  second one, with the first available as a context folder, in automated tests.
- A task that changes both repositories passes `implemented` only with both pull
  requests, in automated tests.
- An autonomous launch of an ambiguous task waits, and resumes when pinned, in
  automated tests.
- The existing launch, worktree and stage evidence tests pass unchanged.

## Open requirements

None. Every product question was settled in the clarification.
