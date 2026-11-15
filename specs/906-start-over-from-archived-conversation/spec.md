# Spec #906 - Start a new conversation or skill from an archived conversation

Ticket: https://github.com/sebastienferry/sectile/issues/906
Clarification: rounds 1 and 2, settled and confirmed by the owner on
2026-11-15, recorded in `docs/clarifications/906.md`.
Branch: `feat/906`

## Problem

An archived conversation in Sectile Desktop holds work the owner may want to
build on. Today the only way back to it is to restore it, which brings the old
session back into the active list and continues it as it was. There is no way
to start something new that knows what the archived conversation said, while
leaving that conversation archived and untouched.

## Scope

In:

- Sectile Desktop (Tauri), conversations shown in the archived list.
- Starting a fresh conversation from an archived conversation.
- Starting a skill from an archived conversation linked to a ticket, on that
  ticket, in the conversation view.

Out:

- Non-archived conversations, finished or not.
- A ticket picker: a free conversation offers no skill.
- Restoring or continuing the archived conversation itself.
- The legacy Electron client (`desktop/`) and the web client.
- An archived conversation whose history is held by another workstation's
  agent: it is reported as unavailable, never fetched.
- Skills launched autonomously (`pickup`, `pickup_issues`) or in the terminal
  view.

## Vocabulary

- **Source**: the archived conversation the owner starts over from.
- **Former context**: the text built from the source's history and given to
  the new session before the owner's new instructions.
- **New session**: the conversation or skill execution created by Start over.
- **Context budget**: about 100,000 characters of former context.

## User stories

### US1 - Start a fresh conversation from an archived one (P1)

- **US1.1** Given an archived conversation is open, then its header shows a
  **Start over** action. Given a non-archived conversation is open, then it
  does not.
- **US1.2** Given the owner presses Start over, then a dialog opens with a
  choice between **New conversation** and **Skill**, an engine or profile
  picker defaulting to the source's engine, and a field for new instructions.
- **US1.3** Given the owner chooses New conversation, types instructions and
  confirms, then a new conversation is created on the chosen engine, its first
  message carries the former context followed by the new instructions, and
  Desktop opens it.
- **US1.4** Given the new conversation runs, then the agent treats the former
  context as background, not as instructions to carry out: the block says so
  explicitly.
- **US1.5** Given the new instructions are empty, then the dialog does not
  start anything.

### US2 - Start a skill from an archived conversation of a ticket (P1)

- **US2.1** Given the source is linked to a ticket, then the dialog offers
  Skill, with the project's skills that can be launched interactively on that
  ticket now.
- **US2.2** Given the source is not linked to a ticket, then the dialog offers
  only New conversation, and says that skills need a ticket.
- **US2.3** Given the owner chooses a skill, types instructions and confirms,
  then the skill starts on the source's ticket in the conversation view, its
  first message carries the skill launch with the new instructions, then the
  former context, and Desktop opens it.
- **US2.4** Given the ticket is finished, is not in the project, already has
  an active run, or the skill is not launchable, or the provider is not
  ready, then the launch is refused with the same message as a launch from the
  ticket, and nothing starts.

### US3 - Keep the source unchanged (P1)

- **US3.1** Given a new session was started from a source, then the source is
  still in the archived list, and its messages, title, engine and status are
  the same as before.
- **US3.2** Given the new session runs and ends, then nothing is ever written
  to the source.

### US4 - Show what was kept (P2)

- **US4.1** Given the dialog is open, then it shows how many messages of the
  source will be given as former context.
- **US4.2** Given the source's user and assistant text exceeds the context
  budget, then the most recent messages that fit are kept, and the dialog
  says how many earlier messages are left out, before the owner confirms.
- **US4.3** Given messages were left out, then the new session shows the same
  count at the top.
- **US4.4** Given the source has tool calls, tool results, notices or
  approvals, then they are not part of the former context, and the former
  context says in one line that they were omitted.

### US5 - Link the new session to its source (P2)

- **US5.1** Given a new session started from a source, then its header shows
  "Started from <source title>".
- **US5.2** Given the owner clicks it, then the source opens read-only, still
  archived.
- **US5.3** Given the source was later deleted, then the line still shows,
  without a link, as "Started from an archived conversation".

### US6 - Explain what cannot be done (P1)

- **US6.1** Given the source's history is still loading, then Start over is
  disabled and says so.
- **US6.2** Given the agent is disconnected, then Start over is disabled and
  says so; nothing is queued for later.
- **US6.3** Given the source has no user or assistant message, then the
  dialog says there is no history to start from, and does not start anything.
- **US6.4** Given the source's history is not held by this workstation's
  agent, then the dialog says the history is unavailable here, and does not
  start anything.
- **US6.5** Given the launch fails, then the dialog stays open with the
  error, the instructions the owner typed are kept, and no session without the
  former context is started.
- **US6.6** Given the source cannot be read when the session actually starts
  (removed in between), then the new session fails with that reason instead of
  running without its former context.

## Functional requirements

- **FR-001** Start over appears only on conversations in the archived list.
- **FR-002** The new session is a new run with its own ID; the source run, its
  history and the archived list are never modified by Start over.
- **FR-003** The former context contains only the source's user and assistant
  text messages, in order, each labelled with its author.
- **FR-004** The former context is delimited, states that it comes from an
  earlier conversation and is background only, and states that tool activity
  was omitted when the source had any.
- **FR-005** When the source's messages exceed the context budget, the most
  recent messages that fit are kept whole, and the count of earlier messages
  left out is part of the former context and of the dialog.
- **FR-006** The owner's new instructions follow the former context and are
  required.
- **FR-007** The new conversation path offers the same engine and profile
  choices as the new-prompt composer, defaulting to the source's engine.
- **FR-008** The skill path is offered only for a source linked to a ticket,
  launches on that ticket only, in the conversation view, with the interactive
  skills only.
- **FR-009** The skill path applies every existing launch check unchanged.
- **FR-010** Every failure listed in US6 is shown to the owner, and none of
  them leads to a session started without its former context.
- **FR-011** The new session records which source it started from, and shows
  it.
- **FR-012** User documentation and `CHANGELOG.md` (`Added`) describe Start
  over.

## Success criteria

- Starting over from an archived conversation, on either path, takes one
  dialog.
- The archived source compares equal, history and metadata, before and after.
- No path starts a session whose former context was requested but missing.
