# Specification #689 - Add a folder from a free console or a native-terminal discussion

- Ticket: https://github.com/sebastienferry/sectile/issues/689
- Branch: `feat/689`
- Clarification: `docs/clarifications/689.md` (rounds 1 and 2; the owner
  answered the only product question with option (a))
- Follows: #676 (`specs/676-add-folder-from-discussion/`)
- Framework: Spec Kit

## Summary

The **Add folder…** action of #676 is offered in the two places it is still
missing: a free "Project prompt" console running in a Sectile terminal, and a
ticket discussion or free console detached to the native terminal. The outcome
is the one of a running ticket discussion: the folder joins the project, and a
Claude Code session has `/add-dir <path>` typed into it so it sees the folder
at once. A detached run is still a terminal session the agent owns; the native
terminal only attaches to it, so the line typed by Sectile shows there at once.

## Scope

In scope:

- a running free console ("Project prompt") in a Sectile terminal;
- a running ticket discussion or free console detached to the native terminal;
- the agent capability that tells the desktop it serves both;
- the status line shown after the action;
- the agent contract, the user guide and the changelog.

Out of scope:

- the web app, detaching a folder, a folder visible to one session only
  (as in #676);
- the bare-shell native terminal (`/desktop/tasks/terminal/external`), which
  starts no engine and is not reachable from the desktop.

## User stories

### US1 - Add a folder from a free console (P1)

As a user talking to an engine in a "Project prompt" console, I attach a
folder of the workstation without leaving the console.

- **Given** a running free console of Claude Code in a Sectile terminal,
  **when** I choose **Add folder…** and pick a folder, **then** the folder is
  attached to the project, `/add-dir <path>` is typed into the session once
  its output settled, and the status line says it was typed into the session.
- **Given** a running free console of another engine (Codex, Antigravity, a
  custom template), **when** I add a folder, **then** it is attached, nothing
  is typed, and the status line says a new Project prompt or a relaunch sees
  it.
- **Given** a free console that ended, **then** the action is not offered,
  and the agent refuses it with 409 if called.

### US2 - Add a folder from a run detached to the native terminal (P1)

As a user who moved a discussion or a console to the native terminal, I keep
the **Add folder…** action in the run's Sectile toolbar.

- **Given** a running Claude Code ticket discussion or free console detached
  to Ghostty, **when** I add a folder, **then** `/add-dir <path>` is typed into
  the session, which Ghostty shows at once, and the status line says it was
  typed into the session in Ghostty.
- **Given** a detached run of another engine, **when** I add a folder,
  **then** it is attached only, and the status line says when the run sees it,
  as for the same run in Sectile.

### US3 - A new desktop against an older agent (P2)

- **Given** an agent that serves `run-folders` but not this change, **then**
  the desktop does not offer the action on a free console nor on a detached
  run, and keeps offering it where #676 did.

## Functional requirements

- **FR1** `POST /desktop/run-folder` accepts a running free console that has a
  terminal session, with the checks, answer shape and refusals of a ticket
  discussion.
- **FR2** A free console records the engine it opened, so that a Claude Code
  console, including an engine whose provider is Claude, is typed into, and
  any other engine is not.
- **FR3** A run detached to the native terminal is typed into like the same
  run in Sectile: the detach no longer turns `now` into `next-launch`.
- **FR4** A refusal for any other run names the runs the action applies to.
- **FR5** The agent reports a new capability on `/desktop/status`; the desktop
  offers the action on a free console or a detached run only when it is
  reported. `run-folders` keeps being reported.
- **FR6** The desktop offers the action on a running free console with a
  session and on a detached run; never on a headless, ended or read-only run.
- **FR7** The status line:
  - names the native terminal when the line was typed into a detached run;
  - says "a new Project prompt or a relaunch sees it" for a free console that
    was not typed into;
  - is unchanged for every case #676 covers.
- **FR8** The contract (`docs/contracts/server-agent-v1.md`), the user guide
  and `CHANGELOG.md` (`## [Unreleased]`) describe the widened scope.

## Open points

None. Known, accepted risk (clarification, Dependencies): the quiet wait
before typing watches the session's output only, so a draft the user left
untouched in the prompt for a second is submitted together with the typed
line. The in-app discussion of #676 shares it.
