# Specification #699 - Animate the conversation status indicator

- Ticket: https://github.com/sebastienferry/sectile/issues/699
- Branch: `feat/699`
- Clarification: `docs/clarifications/699.md` (rounds 1 and 2, every
  recommendation accepted by the owner)
- Framework: Spec Kit

## Summary

In a Sectile Desktop conversation, the status at the right of the composer
toolbar shows a small dot that pulses while Claude Code is working, so the
owner sees at a glance that the model is busy. When Claude waits for the
owner, on a question or an approval, the status turns to the waiting colour
with a still dot. Every other status stays as it is today.

## Scope

In scope: the status of the Desktop conversation composer, and the changelog.

Out of scope:

- The run-state icons of the sidebar, the tickets pane and the task status bar
  (the running state already pulses).
- The run console, the tool cards and their `running…` label.
- The web app.
- The wording of every status sentence.
- Any agent, server, API or database change.

## Vocabulary

- **Status**: the text at the right of the conversation composer toolbar,
  announced to assistive technologies as a live status.
- **Working status**: one of `Claude Code is working…`, `Running in the shell…`,
  `Checking MCP servers…`, `Stopping the answer…`, `Loading conversation…`.
- **Asking status**: `Claude is asking you a question` or
  `Waiting for your approval`.
- **Idle status**: any other text: `Ready`, `Read-only history`, the outcome of
  an added folder, an error message.
- **Indicator**: the dot shown before the status text.

## User stories

### P1 - See that Claude is working

As the owner of a conversation, I want a moving indicator while Claude Code
works, so that I know my message is being handled without reading the status.

- **Given** a conversation, **when** I send a message, **then** the status
  reads `Claude Code is working…` and the indicator pulses before it.
- **Given** Claude Code is answering (the conversation is busy), **when** the
  status shows `Claude Code is working…`, **then** the indicator pulses.
- **Given** I send a message starting with `!`, **when** the status reads
  `Running in the shell…`, **then** the indicator pulses.
- **Given** I send `/mcp`, **when** the status reads `Checking MCP servers…`,
  **then** the indicator pulses.
- **Given** Claude is answering, **when** I press **Stop answer**, **then**
  the status reads `Stopping the answer…` and the indicator pulses.
- **Given** I select a conversation, **when** it is loading, **then** the
  status reads `Loading conversation…` and the indicator pulses.
- **Given** the answer ends, **when** the status returns to `Ready`, **then**
  the indicator disappears.

### P1 - Tell "Claude needs me" from "Claude is busy"

As the owner, I want a pending question or approval to look different from
work in progress, so that I notice that Claude waits for me.

- **Given** Claude asks a question, **when** the status reads
  `Claude is asking you a question`, **then** the text and the indicator use
  the waiting colour and the indicator does not move.
- **Given** a tool call waits for approval, **when** the status reads
  `Waiting for your approval`, **then** it looks the same as a question.
- **Given** the question is answered and Claude resumes, **when** the status
  reads `Claude Code is working…` again, **then** the indicator pulses in the
  accent colour.

### P2 - Respect reduced motion

As an owner who asked the system for reduced motion, I want no animation.

- **Given** the system prefers reduced motion, **when** a working status is
  shown, **then** the indicator is shown still.

## Functional requirements

- **FR1** A working status shows the indicator before its text, in the accent
  colour, pulsing continuously.
- **FR2** An asking status shows its text and the indicator in the waiting
  colour, with no animation.
- **FR3** An idle status shows no indicator and keeps today's faint text.
- **FR4** The indicator follows the status itself: whichever code path writes
  the status also sets its kind, so the indicator never shows a stale state.
- **FR5** An asking status wins over the busy flag, as its text already does:
  while a question or approval is pending, the indicator is the still waiting
  one even if the conversation is busy.
- **FR6** A notice (the outcome of an added folder) and an error message are
  idle, even while the conversation is busy.
- **FR7** Under `prefers-reduced-motion: reduce`, the indicator never animates.
- **FR8** The status texts, their order of precedence and the live `status`
  role are unchanged; the indicator is not announced by assistive
  technologies.
- **FR9** The pulse uses the same rhythm as the running run-state icon
  (2 s, opacity and scale).
- **FR10** `CHANGELOG.md` gains one `Changed` line under `## [Unreleased]`.

## Acceptance criteria

- **AC1** In the conversation UI test, after a message is sent, the status
  is of kind working and its indicator has a running animation.
- **AC2** With a pending `AskUserQuestion`, then with a pending tool approval,
  the status is of kind asking, its colour is the waiting colour and its
  indicator has no animation.
- **AC3** `Ready`, `Read-only history` and a send error are of kind idle and
  show no indicator.
- **AC4** With reduced motion emulated, a working status has no animation.
- **AC5** The existing assertions on the status text pass unchanged.
- **AC6** The changelog line is present.

## Open points

None.
