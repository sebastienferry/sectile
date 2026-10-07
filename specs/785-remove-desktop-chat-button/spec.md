# Spec #785 - Desktop: remove the Claude chat (test) button

Ticket: https://github.com/sebastienferry/sectile/issues/785
Clarification: `docs/clarifications/785.md` (rounds 1 and 2, settled)
Branch: `feat/785`

## Problem

In Conversation mode, the Desktop execution toolbar offers **Claude chat
(test)**, or **Codex chat (test)** on a Codex run, on any terminal execution
with a local directory, running ones included. It starts a second, independent
agent in the directory where the terminal session may still be working. #771
removed this button and said so in `[Unreleased]`, but #783 brought it back,
relabelled per engine, before any release shipped the removal.

## Scope

In: the toolbar button under both labels, the Desktop IPC plumbing only it
used, the documentation that describes it, and the tests that guard its
absence.

Out: the conversation view itself, the **AI consoles** setting, interactive
ticket launches and **Project prompt** opening as conversations, Codex
conversations, the other conversation IPC calls, and the agent endpoint
`POST /desktop/conversation` with `sourceRunId`, which stays for older Desktop
builds.

## User stories

### US1 - The execution toolbar offers no independent chat (P1)

As a Desktop user, I am not offered a second agent in an execution's
directory from the execution toolbar.

- **US1.1** Given **AI consoles** is **Conversation** and a completed terminal
  execution with a local directory is selected, when I look at the execution
  toolbar, then neither **Claude chat (test)** nor **Codex chat (test)** is
  shown.
- **US1.2** Given the same setting and a running terminal execution, Claude or
  Codex, then neither button is shown.
- **US1.3** Given **AI consoles** is **Terminal**, then neither button is
  shown, as before.
- **US1.4** Given **AI consoles** is **Conversation**, when I launch a ticket
  skill interactively or open a **Project prompt** with Claude or Codex, then
  it still opens in the conversation view.

### US2 - The documentation no longer describes the button (P2)

- **US2.1** Given the root README, the Desktop README and the conversation
  prototype notes, then none of them mentions **Claude chat (test)**, **Codex
  chat (test)** or a "Claude chat" as a way to start a conversation.
- **US2.2** Given `CHANGELOG.md` `[Unreleased]`, then a `Removed` line says the
  toolbar button is gone under both labels, and no other unreleased line
  presents it as available.

## Functional requirements

- **FR1** The renderer creates no chat button: its creation, placement, click
  handler, label and visibility rule are removed from `desktop/src/main.js`.
- **FR2** The `create-conversation` IPC handler (`desktop/electron/main.cjs`)
  and the `createConversation` preload entry (`desktop/electron/preload.cjs`)
  are removed; no other caller uses them.
- **FR3** The agent and its `POST /desktop/conversation` endpoint are
  unchanged.
- **FR4** UI tests assert that no button whose name ends in `chat (test)` is
  shown in Conversation mode on a terminal execution with a directory, the
  case where it showed before, and in Terminal mode.
- **FR5** `CHANGELOG.md`: the removal sentence #771 appended to its `Changed`
  line moves to a `Removed` line naming both labels, and the "first message of
  a Claude chat" clause of the conversation permission mode line goes.

## Open points

None.
