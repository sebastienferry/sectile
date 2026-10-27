# Spec #875 - Offer the next step in a task conversation

Ticket: https://github.com/sebastienferry/sectile/issues/875
Clarification: rounds 1 and 2, settled and confirmed by the owner (ticket
comment "Clarification Report", 2026-10-27). The report file
`docs/clarifications/875.md` it names is not on the work branch; this
specification applies the decisions as the ticket comment records them.
Branch: `feat/875`

## Problem

When a workflow skill ends in a Sectile Desktop conversation, the owner reads
the result in the conversation, then has to look elsewhere to go on: the
toolbar's `Next:` button stays disabled while the conversation is open, so
moving on means stopping the conversation first, then launching the next step
from the toolbar or the board. The conversation is where the owner is looking
when the step ends, and it offers nothing.

## Scope

In:

- Sectile Desktop, conversation view, task conversations run by Claude Code or
  Codex.
- Every next step the workflow already defines: Clarify, Specify, Implement,
  Create PR, Adjust, Handoff, with the conditions it already applies to each.

Out:

- Project prompts (free consoles), macro runs, discussions, and the terminal
  view.
- Finished tasks.
- Skills with no expected workflow stage (`pickup`, `pickup_issues`,
  `create_pr`, `rewrite_story`): their completion cannot be validated against
  a stage, so they never lead to the offer.
- The toolbar's `Next:` and `Pickup (full chain)` buttons, the board and the
  tickets list: unchanged.
- Antigravity conversations: the clarification covers Claude and Codex only
  (see Open points).
- The web client.

## Vocabulary

- **Task conversation**: a conversation-view run launched on a task, by Claude
  Code or Codex.
- **Predecessor**: the task conversation in which the step just ended.
- **Successor**: the conversation launched by the offer for the next step.
- **Next step**: the step the workflow names for the task's current stage, as
  the toolbar shows it (`Next: Implement`, `Next: Create PR`, ...).
- **Offer**: the action shown at the end of the predecessor's messages that
  launches the next step.

## User stories

### US1 - The conversation offers the next step once the step is done (P1)

- **US1.1** Given a task conversation whose skill has ended as completed, and
  whose task has reached the stage that skill is expected to reach, when the
  conversation is idle, then the offer appears after its last message, naming
  the next step (`Next: Implement`).
- **US1.2** Given the skill has completed but the task has not reached the
  expected stage yet ("Awaiting stage validation"), then no offer shows; it
  appears as soon as the stage is reached, without the owner refreshing.
- **US1.3** Given the skill failed, was stopped, or its completion is
  unconfirmed, then no offer shows.
- **US1.4** Given the conversation is answering, has a message queued, waits
  for a tool approval, or asks the owner a question (an `AskUserQuestion`, or
  a wait the skill declared), then no offer shows; it appears once none of
  these holds.
- **US1.5** Given the conversation later goes on to run another skill (#586),
  then the offer reflects that skill: it disappears while it runs, and shows
  again for the step after it once it completes and its stage is validated.
- **US1.6** Given the owner sends a new message to the predecessor, the offer
  disappears while the conversation answers, and shows again when it is idle
  and every condition above still holds.

### US2 - The offer follows the workflow's own rules (P1)

- **US2.1** The offer names exactly the step the toolbar would name for the
  task: Clarify after `new`, Specify after `clarified`, Implement after
  `specified`, Create PR or Adjust after `implemented` (Create PR while the
  task records no pull request, through the skill the project's pull request
  creation stage designates), Handoff after `reviewed`.
- **US2.2** Given the task is finished, no offer shows.
- **US2.3** Given the project is not configured locally, or does not offer the
  next skill, no offer shows; the toolbar's status line keeps explaining why.
- **US2.4** Given another run is active on the task, or a launch for it is
  being submitted or waits for its console, no offer shows.

### US3 - Launching the next step starts a fresh conversation (P1)

- **US3.1** When the owner activates the offer, Desktop re-reads the task and
  the runs before launching. If the next step has changed, another run has
  become active on the task, or the predecessor is no longer idle, nothing is
  launched, the offer is redrawn from what was read, and the owner sees why.
- **US3.2** Otherwise the next step is launched as a new task run in the
  conversation view, in the same task worktree, with a fresh context: it does
  not resume the predecessor's session and receives nothing from its
  transcript. Everything it needs comes from what the workflow persisted (the
  ticket, its comments, the committed artefacts).
- **US3.3** While the launch is in flight, the offer reads as in progress and
  cannot be activated twice.
- **US3.4** Once the launch is confirmed (the agent accepted it and the
  successor's run exists), Desktop selects the successor and only then closes
  the predecessor.
- **US3.5** The closed predecessor stays in the sidebar with its full
  transcript, read-only, as any stopped conversation.
- **US3.6** Given the launch fails or is refused, the predecessor stays open,
  selected and usable, the offer stays available, and the reason is shown
  beside it. A refusal because another run is active on the task is reported
  as such; forcing a launch stays a toolbar action.

### US4 - Nothing else changes (P2)

- **US4.1** The toolbar's next-step button, `Pickup (full chain)`, `Skip to
  Handoff`, `Retry` and `Launch anyway` behave as before.
- **US4.2** The server's one-active-run-per-task rule is unchanged: the
  predecessor's run is already over on the server when the offer is shown, so
  the successor is not refused because of it.
- **US4.3** A conversation that was not launched on a task, or that is opened
  read-only from history, never shows the offer.

## Functional requirements

- **FR1** The offer is shown only when all of these hold for the selected
  conversation: it is a task conversation by Claude Code or Codex, not
  read-only; the skill it speaks for (its own, or the one it went on to run)
  is a workflow skill with an expected stage (clarify, specify, implement,
  adjust) and its skill result is "Skill completed"; the conversation is idle,
  with no queued message, no pending approval or question and no declared
  wait; the task is not finished; the workflow names a next step whose skill
  the project offers; no other run is active or being submitted on the task.
- **FR2** The offer is rendered inside the conversation, after the last
  message, as one primary button labelled `Next: <step>` with a one-line
  explanation that it starts a new conversation and closes this one. It is
  keyboard reachable and announced to assistive technologies.
- **FR3** The launch reuses the toolbar's launch path and its checks: the
  freshness re-read, the local busy guard, the duplicate check on the server.
  The only exception is that the predecessor itself does not count as an
  active run on the task, and only while it satisfies FR1.
- **FR4** The predecessor is stopped only after the launch is confirmed, and
  never when the launch fails or is refused.
- **FR5** The successor runs in the conversation view, whatever the engine's
  fallback rules are for the terminal view, with the execution mode the
  toolbar's next step uses.
- **FR6** User-visible strings follow the language Desktop's conversation view
  already uses (English).
- **FR7** `CHANGELOG.md` gains one line under `## [Unreleased]` / `Added`.

## Acceptance criteria

- **AC1** Given a Specify conversation that completed and moved the task to
  `specified`, when it is idle, then `Next: Implement` shows at the end of the
  conversation; activating it opens a new Implement conversation in the same
  worktree, selects it, and the Specify conversation becomes read-only history
  in the sidebar.
- **AC2** Given an Implement conversation that completed on a task with no
  pull request, the offer reads `Next: Create PR`; with a recorded pull
  request it reads `Next: Adjust`.
- **AC3** Given a completed skill whose task is still at the previous stage,
  no offer shows until the stage moves.
- **AC4** Given a conversation waiting for a tool approval or asking a
  question, no offer shows.
- **AC5** Given a launch refused by the server, the predecessor is still open
  and selected, and the refusal is shown beside the offer.
- **AC6** Given a Handoff conversation that completed, the task is finished
  and no offer shows.
- **AC7** A project prompt, a macro run, a discussion and an Antigravity
  conversation never show the offer.
- **AC8** The toolbar's next-step controls behave exactly as before in every
  case above.

## Open points

- **Antigravity conversations.** The clarification names Claude and Codex
  task conversations only. Antigravity conversations are left out; including
  them is a product decision for the owner. Not blocking.
- **Clarification report file.** The committed report
  (`docs/clarifications/875.md`, commits `cface39d` and `5e995bab`) is not
  reachable from this repository; the decisions are taken from the ticket
  comment. Not blocking, as the owner confirmed them there.
