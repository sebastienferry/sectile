# Specification #589 - An attached repository is seen by a running session

- Ticket: https://github.com/sebastienferry/sectile/issues/589
- Branch: `feat/589`
- Clarification: `docs/clarifications/589.md` (rounds 1 and 2, confirmed by
  the owner on 2026-09-28)
- Framework: Spec Kit

## Summary

An owner attached a Git folder to a project in the desktop settings while a
session was running on one of the project's tasks. From that session,
`prepare_repository_worktree` kept refusing the repository as "neither
associated nor attached", and `transition_stage` then refused its pull request
in `prUrls`, because the repository never joined the task's changed
repositories. Only a desktop restart or a new run got out of it.

An attachment must apply to the calls that follow it, from any session,
running or new. When a refusal is legitimate, it must name its real reason,
so that the owner is never told to attach a folder that is already attached.

The clarification established that the cause written in the ticket (a launch
snapshot or a cache of attachments) is contradicted by the code: the agent
reads the workstation settings and runs `git` on each attached folder at every
call. The cause is therefore not known yet. This specification makes the
reproduction the first deliverable, and states the expected behaviour whatever
the cause turns out to be.

## Scope

In scope:

- Reproducing the defect, automatically and on the owner's workstation, and
  fixing the confirmed cause.
- The refusals of `prepare_repository_worktree` for a repository resolved
  through attached folders: each one names its actual reason.
- The refusal of `transition_stage` for a pull request outside the task's
  changed repositories: it says how a repository becomes changed.
- The description of the `prepare_repository_worktree` tool: what it reads,
  and what `SECTILE_REPOSITORIES` is.

Out of scope:

- Refreshing the environment of a running agent process: `SECTILE_REPOSITORIES`
  and the folder list of the launch prompt stay a launch snapshot.
- Accepting in `prUrls` a pull request of a repository the task never prepared
  through `prepare_repository_worktree` (settled "no" in round 2).
- Linking argocd-pl-tooling !844 to SFE-376: the owner does it once the fix is
  in, or from the task detail view.
- The attached-folders panel of the desktop app, its layout and its wording.

## User stories

### US1 - An attachment applies to the next call of a running session (P1)

As an owner whose agent is working on a task, I attach a folder to the project
in the desktop settings, and the running session can prepare a worktree in that
repository right away, without restarting anything.

Acceptance:

1. **Given** a session running on a task of project P and a Git folder F whose
   origin is repository R, not attached to P, **when** the owner attaches F to
   P in the desktop settings and the session then calls
   `prepare_repository_worktree` for R (by remote URL or host/path), **then**
   the call returns the task's worktree in R, on the task's branch.
2. **Given** acceptance 1 succeeded, **when** the session calls
   `transition_stage` with R's pull request in `prUrls`, **then** the pull
   request is accepted like any other changed repository's.
3. **Given** the owner detaches F from P, **when** the session calls
   `prepare_repository_worktree` for R again, **then** it is refused as not
   attached (US2-1), with no restart either.
4. **Given** a session started before the attachment, **then**
   `SECTILE_REPOSITORIES` in its environment still lists the repositories of
   its launch; this is expected and does not affect acceptances 1 to 3.

### US2 - A refusal names its real reason (P1)

As an owner or an agent reading a refusal of `prepare_repository_worktree`, I
learn what actually stands in the way, and I am told to attach a folder only
when no folder of that repository is attached.

Acceptance, for a call naming repository R on project P:

1. **Given** no mapping and no folder attached to P has R as origin, and every
   attached folder was read without error, **then** the refusal says R is
   neither associated nor attached to P on this workstation, names the
   workstation that answered, and says to attach R's folder in the project
   settings of the desktop app.
2. **Given** a folder attached to P no longer exists or is not a directory,
   and no other folder answers for R, **then** the refusal names that folder
   and says it is missing.
3. **Given** a folder attached to P is a directory but not a Git checkout, and
   no other folder answers for R, **then** the refusal names that folder and
   says it is not a Git checkout.
4. **Given** a Git folder attached to P has no `origin` remote, and no other
   folder answers for R, **then** the refusal names that folder and says it
   has no origin (and that a folder without a remote is changed in place, as
   today when it is named by its path).
5. **Given** a Git folder attached to P has an origin other than R, and no
   other folder answers for R, **then** the refusal names the folders it
   checked with the origin each one has.
6. **Given** a `git` command fails on an attached folder for another reason
   than "not a checkout" or "no origin" (permissions, `safe.directory`,
   missing `git`, timeout), **then** the refusal names that folder and carries
   the `git` error message; it never reads as "not attached".
7. When several attached folders fail for different reasons, the refusal
   lists each folder with its own reason.
8. When a folder attached to P answers for R, a failure on another attached
   folder does not prevent the call from succeeding.
9. The refusals stay in French, the language this surface already speaks.

### US3 - The prUrls refusal says how to make a repository changed (P2)

As an agent whose `transition_stage` is refused because a pull request is not
in a changed repository, I learn what to do next.

Acceptance:

1. **Given** a pull request in `prUrls` whose repository is not among the
   task's changed repositories nor its primary one, **when** `transition_stage`
   is called, **then** it is refused as today, with the list of changed
   repositories, **and** the message adds that a repository becomes changed
   by calling `prepare_repository_worktree` for it first.
2. The rule itself does not change: the pull request is not accepted.

### US4 - The tool description states its source of truth (P3)

As an agent reading the MCP tool list, I know that `SECTILE_REPOSITORIES` is a
snapshot and that `prepare_repository_worktree` reads the current settings.

Acceptance:

1. The description of `prepare_repository_worktree` says that the tool reads
   the workstation's current project settings at each call, so a folder
   attached after the session started is accepted, and that
   `SECTILE_REPOSITORIES` lists the folders known when the run was launched.

## Functional requirements

- **FR1 Reproduction first.** Before any fix, an automated test attaches a
  folder through the local agent's `POST /desktop/folders`, then runs the
  `repository_worktree` operation for its repository on the same agent, and
  records the outcome. A manual reproduction on the owner's workstation
  follows the steps of the ticket with the agent log open, and records how
  many agent connections the server holds for the user and project at the
  time, and which workstation answered the operation. The confirmed cause is
  written in the pull request description and in `docs/clarifications/589.md`.
- **FR2 Attachments are read per call.** The `repository_worktree` operation
  resolves attached folders from the workstation settings as they are at the
  time of the call. No state kept since the agent started, the desktop app
  started, or the run was launched decides whether a folder is attached. The
  automated test of FR1 guards this.
- **FR3 The confirmed cause is fixed** so that US1 holds. If the cause proves
  to be by design (for example, the operation was answered by an agent of
  another workstation, or of another user environment), the refusal says so
  instead: it names the workstation that answered (US2-1), which lets the
  owner see that it is not the one where the folder was attached.
- **FR4 Folder diagnosis.** Reading an attached folder distinguishes: missing
  or not a directory, not a Git checkout, Git checkout without an origin, Git
  checkout with an origin, and `git` failing for another reason, with its
  message. The desktop's attached-folders panel keeps showing the kinds it
  shows today.
- **FR5 Refusal wording.** A refusal of `repository_worktree` for a
  repository not found among mappings and attached folders follows US2,
  in French. The "not attached" advice appears only in the US2-1 case.
- **FR6 Success unchanged.** When a mapping or an attached folder answers for
  the repository, the operation behaves as today: same worktree, same branch,
  same exclusion of task worktrees, and the server records the repository as
  changed.
- **FR7 prUrls refusal.** The `transition_stage` refusal of a pull request
  outside the changed repositories keeps its current content and adds that
  the repository must first be prepared with `prepare_repository_worktree`
  (US3). This message stays in English, as it is today.
- **FR8 Tool description.** The MCP description of
  `prepare_repository_worktree` states US4-1.
- **FR9 Changelog.** `CHANGELOG.md` carries one `Fixed` line under
  `## [Unreleased]`: a folder attached while a session runs is usable by that
  session, and refusals of `prepare_repository_worktree` say why a repository
  cannot be prepared. The wording of the line follows the confirmed cause.

## Success criteria

- The acceptance scenarios of US1 pass in an automated test that attaches the
  folder through the agent's HTTP endpoint between two operations on the same
  agent, with no restart.
- Each refusal case of US2 is covered by an automated test on real temporary
  folders (missing, plain folder, checkout without origin, checkout with
  another origin, `git` failing).
- On the owner's workstation, the ticket's repro succeeds at step 5, or, if
  the cause is by design, the refusal at step 5 names the workstation that
  answered.
- No database migration, and no change to the server-agent operation
  contract beyond the error text.

## Assumptions

- The owner's workstation runs a single `HOME`, so every agent on it reads the
  same `~/.config/sectile/settings.json`.
- Naming the answering workstation in a refusal discloses nothing sensitive:
  it is the device name the agent already reports to the server and that the
  activity steps already show ("Déléguée à l'agent local (<device>)").

## Open points

- **The cause is not confirmed.** The clarification left three candidates:
  (1) the operation answered by another agent connection than the one the
  desktop wrote through (two agents for the same user and project, a stale
  route after a reconnection, or a session whose MCP bridge talks to another
  server than the desktop's); (2) the attachment first saved on another
  project; (3) a `git` call failing in the agent's environment, swallowed into
  "not attached". FR1 decides it. It blocks FR3 only: FR2, FR4 to FR8 do not
  depend on it, and FR9's wording does.
