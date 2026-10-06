# Specification #761 - Desktop: Open terminal

- Ticket: https://github.com/sebastienferry/sectile/issues/761
- Branch: `feat/761`
- Clarification: `docs/clarifications/761.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-06)
- Framework: Spec Kit

## Summary

A project's `…` menu in the Sectile Desktop sidebar gains **Open terminal**,
right after **Project prompt**. It opens a native terminal window, running the
user's own shell, on the project's local repository on this workstation, in
the terminal application the project is set to use.

## Scope

In scope:

- the menu item, its availability and its error reporting in Desktop;
- one agent endpoint, `POST /desktop/project-terminal`, and the capability that
  announces it;
- the agent contract, the Desktop README and the changelog.

Out of scope:

- a shell embedded in Desktop as a sidebar console (a possible follow-up);
- terminals on a task worktree or an execution's folder (the conversation's
  **Terminal** button already covers them);
- the web app, and any new terminal setting.

## User stories

### US1 - Open a terminal on the project (P1)

As a Desktop user, I open a terminal on a project's local repository from the
sidebar, so I can run commands there without looking for the folder.

- **Given** a project mapped to a local repository on this workstation and an
  agent that announces the capability, **when** I choose **Open terminal** in
  its `…` menu, **then** a native terminal window opens on that repository's
  folder, running my own shell, with no Sectile session attached.
- **Given** the project sets a terminal application in its workstation
  settings, **when** I choose **Open terminal**, **then** that application is
  the one that opens.
- **Given** the project sets none, **when** I choose **Open terminal**,
  **then** the agent's terminal applies, as for the conversation's **Terminal**
  button: the agent's explicit flag, then its default, then detection.

### US2 - The item is offered only when it can work (P1)

- **Given** a project with no local folder (not mapped, not cloned, or
  disconnected), **when** I open its `…` menu, **then** **Open terminal** is
  shown disabled, as **Project prompt** is.
- **Given** an agent that does not announce the capability, **when** I open a
  project's `…` menu, **then** **Open terminal** is not shown.

### US3 - A failure is reported (P2)

- **Given** the terminal cannot be started, or the folder can no longer be
  resolved, **when** I choose **Open terminal**, **then** Desktop shows the
  agent's reason in its error banner, and nothing is retried.

## Functional requirements

- **FR1** The item is labelled `Open terminal` and sits directly after
  `Project prompt` in the project's `…` menu.
- **FR2** Desktop sends the project ID only. The agent resolves the folder
  itself, the way **Project prompt** does; a path in the request is ignored.
- **FR3** The folder is the project's local root as **Project prompt**
  resolves it: the workstation mapping, or the agent's own checkout for the
  project it is linked to.
- **FR4** The terminal application is chosen as for the conversation's
  **Terminal** button.
- **FR5** The window opened is visible on purpose on every platform: it is a
  launch the user asked to see, and the code says so.
- **FR6** `GET /desktop/status` announces the capability `project-terminal`;
  Desktop hides the item without it.
- **FR7** The endpoint answers 405 to anything but `POST`, 400 without a
  project ID, 502 when the project's configuration cannot be read from the
  server, 409 when the project has no local folder on this workstation, and
  500 with the launch error. On success it answers `{opened: true, terminal,
  directory}`.
- **FR8** `CHANGELOG.md` gains one `Added` line under `[Unreleased]`.

## Success criteria

- Choosing **Open terminal** on a mapped project opens one terminal window on
  its folder.
- The item never appears against an agent that cannot serve it, and is never
  enabled on a project with no local folder.
