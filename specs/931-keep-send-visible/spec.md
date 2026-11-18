# Specification #931 - Keep Send visible while queueing

Ticket: https://github.com/sebastienferry/sectile/issues/931
Clarification: `docs/clarifications/931.md` (rounds 1 and 2).

## Scope

The Tauri conversation composer and its existing follow-up queue. Provider,
backend, queue persistence, interruption and delivery policies are unchanged.
The existing English runtime language is preserved.

## User stories

### US1 - Send keeps its identity (P1)

- Given a writable conversation, when it is idle, busy or holds a queue,
  then the primary submission control keeps its Send name, icon and appearance.
- Given a busy conversation, then the separate Interrupt action remains
  available and contextual feedback explains that a follow-up will wait.

### US2 - Submit once through either interaction (P1)

- Given an idle conversation with an empty queue, when Send or Enter submits
  a valid draft, then it is sent immediately and the composer is cleared.
- Given a busy conversation or existing queue, when Send or Enter submits a
  valid draft, then it joins the queue exactly once without interrupting.
  The existing Queued strip shows it waiting; messages are delivered in order,
  one per completed turn, according to the existing pause and retry rules.
- Given a draft accepted by submission, when another submission arrives before
  the composer updates, then the accepted draft is not sent or queued again.
- Given a newly entered draft identical to an earlier message, then it can be
  submitted normally.

### US3 - Preserve draft admission rules (P1)

- Given an empty draft, pending attachment, unloaded or read-only conversation,
  then submission does not admit a message.
- Given an unavailable agent or conversation error while idle with no queue,
  then the draft is retained and no message is sent.
- Given a busy conversation or existing queue, then existing local queue
  admission remains available, including during an outage.
- Given a refused enqueue, including a full queue, then the draft is retained;
  the existing queue-limit explanation stays visible.

## Functional requirements

- FR1: US1-US3 hold for button and keyboard submission.
- FR2: Update the Tauri README to describe Send while busy.
- FR3: Add a Fixed entry under Unreleased in both changelogs.

## Success criteria

Interaction tests cover idle Send and Enter, busy Send and Enter, synchronous
duplicate events, queue feedback and order, repeated intentional text, refusal
and availability. Existing attachment, history, completion and queue tests pass.
Tauri type checking, unit tests, UI tests, production build and Rust tests pass.
