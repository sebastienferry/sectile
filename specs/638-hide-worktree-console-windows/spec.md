# #638 — Creating a worktree on Windows opens a flickering console

Ticket: https://github.com/sebastienferry/sectile/issues/638
Type: Bug — "When creating worktree on windows it should not open a flickering console".
Branch: `feat/638`.
Clarification: [`docs/clarifications/638.md`](../../docs/clarifications/638.md).

## Context

Sectile Desktop starts the local agent detached, so on Windows the agent owns no console
and Windows opens a fresh, visible window for every console child that does not say it
needs none. Fix #301 routed the agent's own commands through a helper that says so, but
two launches still bypass it: the dependency install that follows the creation of a task
worktree, and the `git` calls made when the agent reads a macro's specification files.
The install keeps its window open for minutes.

This ticket hides those two launches. Out of scope: the windows a user asked to see (the
native terminal launcher, an interactive run, "Open in editor"), every non-Windows
platform, and when or how a worktree is created and provisioned.

This file states behaviour and acceptance criteria only. Implementation choices are in
[`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

## Decisions being specified

Settled by the owner in Round 2 of the clarification.

1. **Both launches are hidden** (option A): the worktree dependency install and the macro
   specification reader's `git` calls.
2. **The existing helper is reused** unchanged; nothing else about provisioning changes.

## User stories

### US1 (P1) — Preparing a task worktree opens no window

As a Windows user of Sectile Desktop, when the agent prepares a task worktree, no console
window appears, flickers or steals focus.

- **Given** the agent started by Sectile Desktop on Windows and a task with no worktree,
  **When** a skill is launched on that task,
  **Then** the worktree is created and its dependencies are installed with no console
  window opening at any point.
- **Given** the same agent and a task whose branch the board checks out,
  **When** the board prepares the workspace,
  **Then** the worktree and its background dependency install open no console window.

### US2 (P2) — Reading a macro's specification files opens no window

As a Windows user of Sectile Desktop, when the board shows a macro's specification files
read through the agent, no console window appears.

- **Given** the agent started by Sectile Desktop on Windows and a macro with a branch,
  **When** the board reads that macro's specification files,
  **Then** the files are returned and no console window opens.

### US3 (P2) — Nothing else changes

- **Given** a dependency install that succeeds, fails or times out,
  **When** it runs from a worktree preparation,
  **Then** it is logged and bounded exactly as before, and a failure still leaves the
  launch to proceed.
- **Given** the native terminal launcher, an interactive run or "Open in editor",
  **When** the user starts it,
  **Then** its window opens as before.

## Functional requirements

- **FR1** — On Windows, the `npm ci` the agent runs to provision a task worktree is started
  with no console window, and so is every process it starts.
- **FR2** — On Windows, every `git` command the agent runs to read a macro's specification
  files is started with no console window.
- **FR3** — The install's output capture, error reporting, timeouts and stamp handling are
  unchanged, and so is the specification reader's output.
- **FR4** — Launches meant to be visible keep their window.
- **FR5** — Behaviour on non-Windows platforms is unchanged.
- **FR6** — Each hidden launch is covered by a Windows test asserting that it asks for no
  console window.

## Open requirements

None.
