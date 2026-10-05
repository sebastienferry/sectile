# Specification #737 - Agent: option to work in any repository, not only the project's

- Ticket: https://github.com/sebastienferry/sectile/issues/737
- Branch: `feat/737`
- Clarification: `docs/clarifications/737.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-05)
- Framework: Spec Kit

## Summary

A project's workstation settings gain an option, **Any repository**, off by
default. When it is on, a ticket of that project may change a repository the
project neither declares, nor maps, nor attaches: the AI session names a local
checkout it found, or Sectile's agent clones the repository, and the ticket's
worktree, branch and pull request then work as for a declared repository. The
same option lets a ticket be pinned to such a repository, and lets a ticket
whose only change is in a secondary repository be launched without a worktree
in the code repository and finish without a pull request there. Independently
of the option, a new branch in any secondary repository now starts from the
up-to-date remote default branch.

## Scope

In scope:

- the per-project workstation option and its clones folder, in the Desktop
  project settings;
- an optional `path` on `prepare_repository_worktree`, checked by the agent;
- cloning by the agent when no folder answers;
- remembering every folder found or cloned in the workstation mapping;
- fetching before a new branch is created in a secondary repository;
- launching without a code worktree, and the pull request rule that follows;
- pinning a ticket to a repository its project does not declare;
- showing a worktree created mid-run to the running Claude Code session;
- the skill fragments, user guide, agent contract, ADR and changelog.

Out of scope:

- Sectile searching the disk for checkouts: the AI session does that.
- Storing git credentials: a clone uses the workstation's own git setup.
- Any server-side copy of the option: it stays a workstation setting.
- Changing what happens when the option is off, except the fetch fix (US5).
- The hand-maintained marketplace plugin.

## Vocabulary

- **Option**: the per-project workstation setting "Any repository".
- **Code repository**: the project's own repository, whose checkout is the
  project checkout on this workstation.
- **Declared repository**: one of the project's repositories on the server.
- **Known folder**: a folder the agent already finds for a repository: its
  workstation mapping, the project checkout for the code repository, or an
  attached folder whose `origin` is the repository.
- **Undeclared repository**: a repository that is neither declared nor known.
- **Clones folder**: where the agent clones an undeclared repository; set next
  to the option, defaulting to the parent folder of the project checkout.
- **Specifications away from the code**: the project drops its specification
  artefacts on this workstation (`specArtifacts` = `drop`), or its Issue
  specifications folder is distinct from the code checkout (#736).
- **Lazy code worktree**: the code repository's worktree of a ticket created
  only when the agent asks for it, rather than at launch.

## User stories

### US1 (P1) - Keep every workstation as it is

As an owner who never turns the option on, I see no change, except that new
secondary branches start from the remote default branch (US5).

1. Given the option is off, when `prepare_repository_worktree` is called for a
   repository that has no known folder, then it is refused with today's
   message, which also names the option.
2. Given the option is off, when `prepare_repository_worktree` is called with a
   `path`, then it is refused, saying the option is off, and nothing is
   written.
3. Given the option is off, when a ticket is launched, then its code worktree
   is created at launch as today, and its pull request rules are today's.
4. Given the option is off and a ticket is pinned to an undeclared repository,
   when it is launched on this workstation, then the launch fails, naming the
   repository and the option.

### US2 (P1) - Work in a repository the session found

As an owner whose ticket needs a change in a repository the project does not
list, I let the AI session find its checkout and the ticket proceeds.

1. Given the option is on and the session calls `prepare_repository_worktree`
   with a repository and a `path` that is the top level of a Git checkout
   whose `origin` is that repository, then the ticket's worktree is created or
   reused there on the ticket's branch, the repository is recorded as changed
   on the ticket, and its pull request is accepted at the transition as for a
   declared repository.
2. Given a `path` that is not absolute, not a directory, not a Git checkout,
   not its top level, or whose `origin` is another repository, then the call is
   refused with the reason, and nothing is created or remembered.
3. Given the repository already has a known folder, when a `path` is also
   given, then the known folder is used, `path` is ignored, and the answer says
   so.
4. Given a `path` was accepted, when the next ticket of any project on this
   workstation names the same repository, then it is found without `path`.
5. Given a ticket that changed a repository through `path`, when
   `handoff-issue` removes the ticket's worktrees, then that repository's
   worktree is removed as for a declared one.

### US3 (P1) - Let the agent clone a repository the workstation lacks

As an owner, I let Sectile clone a repository no folder on my workstation
holds, so the ticket is not blocked.

1. Given the option is on and the repository has no known folder and no `path`
   is given, when `prepare_repository_worktree` is called, then the agent
   clones it into `<clones folder>/<repository name>` with the workstation's
   git configuration, remembers the clone, and creates the ticket's worktree
   in it.
2. Given the caller gave a full URL, then that URL is cloned. Given it gave
   only `host/path`, then the URL is built with the scheme of the project's
   code remote: SSH when the code remote is SSH, HTTPS otherwise.
3. Given `<clones folder>/<repository name>` already exists, when it is a
   checkout of the same repository, then it is used and remembered without
   cloning; otherwise the call is refused, naming the folder, and nothing is
   overwritten.
4. Given the clone fails (network, authentication, unknown repository), then
   the call is refused with git's message, and no partial folder is left
   behind.
5. Given the clones folder setting is empty, then the parent folder of the
   project checkout is used; given it names a folder that does not exist, then
   it is created.

### US4 (P1) - The running session can write in the new worktree

1. Given a Claude Code session launched by the agent for the ticket, when a
   worktree is created during the run in a folder the session was not
   launched with, then the agent adds it to that session as it does for a
   folder attached from a run (#676), and the tool's answer says it was added.
2. Given the session was not launched by the agent (a session typed by hand),
   when the worktree is created, then the answer says the folder must be added
   to the session (`/add-dir <path>`) before writing there.

### US5 (P1) - New secondary branches start from the remote default branch

This story applies whether the option is on or off.

1. Given a secondary repository where the ticket's branch exists neither
   locally nor on `origin`, when its worktree is prepared, then the agent
   fetches `origin` and creates the branch from the remote default branch,
   whatever branch the checkout is on and however old it is.
2. Given the branch exists on `origin` and not locally, then it is fetched and
   the worktree is created from it.
3. Given the branch exists locally, then it is reused as today, without being
   moved.
4. Given the fetch fails, then the worktree is created from what is known
   locally, and the answer carries a warning saying so.

### US6 (P1) - Launch without a code worktree when nothing needs one

As an owner whose specifications are away from the code, I do not want an
empty worktree in the code repository for a ticket that only changes another
repository.

1. Given the option is on and specifications are away from the code, when a
   ticket is launched, then no worktree is created in the code repository; the
   session starts in the Issue specifications worktree when that folder is
   distinct, else in the project checkout, and the code repository is
   described to the session as a read-only folder that
   `prepare_repository_worktree` makes writable.
2. Given the same launch, when the session calls `prepare_repository_worktree`
   on the code repository, then the ticket's code worktree is created on the
   ticket's branch, and the code repository is recorded as changed.
3. Given the ticket has no branch name yet, when a worktree is prepared in any
   repository, then the branch is named with the project's branch format, as
   the code worktree's would have been.
4. Given the option is on and specifications are committed in the code
   repository, when a ticket is launched, then the code worktree is created at
   launch as today.
5. Given the code worktree already exists for the ticket, when the ticket is
   launched again, then it is reused, whatever the settings.

### US7 (P1) - Finish with pull requests only where something changed

1. Given a ticket launched without a code worktree, whose secondary
   repositories each have a pull request, when it is transitioned to
   `implemented`, then the transition is accepted without a pull request in the
   code repository, and the task's current pull request is the first changed
   repository's.
2. Given the same ticket later changed the code repository, then the code
   repository's pull request is required and becomes the task's current one.
3. Given a ticket launched with its code worktree, when it is transitioned
   without a code pull request, then it is refused as today.
4. Given a ticket launched without a code worktree that changed no repository,
   then the transition behaves as today for a ticket without a pull request
   (`noRepositoryChange`).

### US8 (P2) - Pin a ticket to an undeclared repository

1. Given the task detail of a project with several repositories, when I set
   its repository, then besides the project's repositories I can pick
   **Other repository…** and type a repository URL or `host/path`, which is
   stored on the task.
5. Given a project that stops declaring a repository, when it is saved, then
   the tasks pinned to that repository are unpinned.
2. Given a task pinned to an undeclared repository and the option on, when it
   is launched, then its primary worktree is created in that repository, found
   through its known folder or cloned (US3), never through `path`.
3. Given the same task, then its pull request in that repository is its primary
   one, and the code repository needs none unless the ticket changed it.
4. Given the option off on the launching workstation, then the launch fails as
   in US1.4.

### US9 (P1) - Set the option in Desktop

1. Given the Desktop project settings, when I open them, then I see "Any
   repository", off, and a "Clones folder" row, empty, whose effective value
   is shown as the parent folder of the project checkout.
2. Given I turn the option on and save, then the workstation settings hold it
   for that project only, and the next `prepare_repository_worktree` call
   honours it without restarting the agent.
3. Given the clones folder is relative, then it is refused, naming the
   setting.

## Functional requirements

- FR1. The option is a per-project workstation setting, off by default, never
  sent to the server.
- FR2. `prepare_repository_worktree` takes an optional `path`, accepted only
  with the option on and only after the checks of US2.2.
- FR3. The lookup order for a repository's folder is: workstation mapping,
  project checkout for the code repository, attached folder, `path`, clone.
- FR4. A folder accepted through `path` or a clone is written to the
  workstation mapping unless the repository is already mapped.
- FR5. Every repository whose worktree is prepared is recorded as changed on
  the ticket, the code repository included when its worktree is lazy.
- FR6. A new branch in a secondary repository is created from the remote
  default branch after a fetch; a fetch failure is a warning, not a refusal.
- FR7. With the option on and specifications away from the code, no code
  worktree is created at launch.
- FR8. A pull request is required for each changed repository; the code
  repository is exempt only when its worktree was never created for the
  ticket, and only under FR7.
- FR9. A ticket can be pinned to any repository named by a remote URL or
  `host/path`; a launch resolves it only with the option on.
- FR10. A worktree created mid-run is added to the agent's running Claude Code
  sessions of the ticket.
- FR11. Every new child process is built through `agentexec.Hidden`.
- FR12. Runtime messages shown to the user follow the project's rule for
  user-facing strings; existing French messages are not translated in passing.

## Acceptance criteria

- AC1. All user stories pass as automated tests, on temporary Git repositories
  for the agent side, and on a test database for the server side.
- AC2. With the option off, the existing suites of `internal/agent` and
  `internal/db` pass unchanged, except the tests updated for US5.
- AC3. The MCP tool `prepare_repository_worktree` exposes the new `path`
  input; the bridge relays it without change (it whitelists tool names only).
- AC4. `CHANGELOG.md` has one line under `Added` for the option and one under
  `Fixed` for US5.
- AC5. `docs/USER_GUIDE.md` and `docs/CAPABILITIES.md` describe the option, the
  clones folder and the lazy code worktree; an ADR records the decisions.
- AC6. A Windows test asserts `CREATE_NO_WINDOW` on the clone and fetch
  commands.

## Open points

None from the clarification. The technical choices that the clarification
left to the specification are listed in `plan.md`, section "Decisions".
