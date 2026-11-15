# Spec #905 - Tauri: queue follow-up messages while an agent is responding

Ticket: https://github.com/sebastienferry/sectile/issues/905
Clarification: `docs/clarifications/905.md` (rounds 1 and 2, confirmed by the owner)
Branch: `feat/905`

## Problem

In the Sectile Desktop (Tauri) conversation view, the composer refuses to submit
while the agent is responding: the Send button is replaced by Interrupt and Enter
does nothing. A user who already knows the next instruction has to watch the
answer finish, remember the instruction and come back. Interrupting is not an
option, since it throws away the answer in progress.

## Scope

### In

- The conversation composer of an existing conversation in the Tauri desktop app
  (`Conversation` in `desktop-tauri/src/components/ProjectPrompt.tsx`).
- A per-conversation queue of follow-up messages, held by the desktop app in
  memory, dispatched one message per turn, in insertion order.
- Visible queued messages, removable before dispatch, and copyable.
- Pausing and resuming the queue on interrupt, stop, turn error and send failure.
- User documentation in `desktop-tauri/README.md` and an entry in
  `desktop-tauri/CHANGELOG.md`.

### Out

- Scheduled or offline delivery.
- The "new prompt" composer (`ProjectPrompt`), which has no active turn.
- The Electron desktop app and the web client.
- Steering: sending a message into the turn in progress (possible follow-up ticket).
- Persisting the queue across a restart of Sectile Desktop.
- Any change to the Go conversation engine, the Tauri Rust commands or the API.
- Editing a queued message in place (remove it and type it again).
- Translating existing UI strings.

## Vocabulary

- **Turn**: one message from the user and the agent's full response to it.
- **Busy**: the conversation snapshot reports `busy: true`, or a send or read
  started by this app has not returned yet.
- **Idle**: a freshly read snapshot reports `busy: false`, the conversation is not
  read-only, it has no pending approval or question, and the last read succeeded.
- **Queue**: the ordered list of messages waiting to be sent in one conversation.
- **Paused queue**: a queue that does not dispatch until the user resumes it.
- **Dispatch**: sending the head of the queue as a new turn.

## User stories and acceptance criteria

### US1 - Queue a message while the agent responds (P1)

- **US1.1** Given a conversation that is busy, when the user types a message and
  presses Enter (or clicks **Queue**), then the message is added at the end of the
  conversation's queue, the draft is cleared, and the response in progress
  continues: no interrupt is sent and no message reaches the agent.
- **US1.2** Given a conversation that is busy, the composer shows both the
  existing **Interrupt** button and a **Queue** button. Esc keeps interrupting.
- **US1.3** Given a conversation that is idle with an empty queue, when the user
  submits a message, then it is sent immediately, exactly as today. No queue is
  shown.
- **US1.4** Given a queue that is not empty, when the user submits a message,
  then it is appended to the queue even if the conversation is idle, so it is
  never sent ahead of messages queued before it.
- **US1.5** Given a queue holding 20 messages, the **Queue** button is disabled,
  Enter does not add the draft, the draft is kept, and the composer says
  `Queue full (20 messages). Remove one to add another.`
- **US1.6** A queued message obeys the same 32,000-character limit as a sent one.
  An empty or whitespace-only draft is never queued.
- **US1.7** The model and mode selected in the composer when a message is queued
  are the ones used when it is dispatched.

### US2 - See and manage the queue (P1)

- **US2.1** Given a non-empty queue, the queued messages are listed between the
  transcript and the composer, in dispatch order, each with its position, its
  text, a **Remove** button and a **Copy** button, under a heading
  `Queued (N)`.
- **US2.2** Queued messages are visibly distinct from sent messages: they are not
  part of the transcript and carry a "queued" style and label. A queued message
  joins the transcript only once the agent has received it.
- **US2.3** When the user clicks **Remove** on a queued message, it disappears
  from the queue and is never sent. The order of the other messages is kept.
- **US2.4** A message being dispatched (sent, not yet acknowledged) is labelled
  `Sending…` and cannot be removed.
- **US2.5** **Copy** puts the message text on the clipboard.

### US3 - Dispatch in order, one per turn (P1)

- **US3.1** Given a queue of messages A, B, C and a busy conversation, when the
  turn in progress completes and a fresh snapshot shows the conversation idle,
  then A alone is sent as a new turn. B is sent only after A's turn completes,
  then C after B's.
- **US3.2** Queued messages are never merged into one turn and never sent into
  the turn in progress.
- **US3.3** Each queued message is sent at most once by the app: a message is
  never dispatched twice, including when several reads or snapshot updates
  arrive at the same time.
- **US3.4** The queue does not dispatch while the conversation has a pending
  approval or question. A queued message never answers an approval or question.
- **US3.5** Dispatch continues while the user looks at another conversation or
  another view of the app: the queue belongs to the conversation, not to the
  open view.

### US4 - Isolation between conversations (P1)

- **US4.1** Given queues in conversations X and Y, a message queued in X is only
  ever sent to X, whichever conversation is displayed when it is dispatched.
- **US4.2** When the user switches from X to Y and back, X's queue is shown
  unchanged (minus the messages sent meanwhile), and Y's composer shows only Y's
  queue.
- **US4.3** A conversation whose queue is not empty is never evicted from the
  app's conversation cache, whatever the number of conversations opened since.

### US5 - Pause and resume (P1)

- **US5.1** When the user interrupts the turn (Esc or **Interrupt**), the queue
  pauses before the next dispatch. Messages stay visible.
- **US5.2** When a turn ends with an error or an interruption (including one
  triggered from another client), the queue pauses.
- **US5.3** A paused queue shows `Queue paused` with the reason (`after an
  interrupt`, `after an error`, `send failed: <error>`) and a **Resume** button.
  **Resume** dispatches the head of the queue as soon as the conversation is idle
  (immediately if it already is) and normal dispatch continues.
- **US5.4** While a queue is paused, new submissions are appended to it and wait.
- **US5.5** When the queue becomes empty, its paused state is cleared.

### US6 - Send failure (P1)

- **US6.1** Given a dispatch whose send call fails, then the message stays at the
  head of the queue, marked failed with the error text, and the queue pauses.
- **US6.2** A failed message is never retried automatically. **Resume** retries it
  explicitly; **Remove** discards it.
- **US6.3** A failed message is never lost silently: it stays until the user
  removes it, retries it successfully, or quits the app.

### US7 - Disconnection, stop and restart (P2)

- **US7.1** While the agent is unavailable, the queue does not dispatch. Messages
  can still be queued in a conversation that was busy or already has a queue.
- **US7.2** When the agent becomes available again and a fresh snapshot shows the
  conversation idle, an unpaused queue resumes dispatch, whether or not the
  conversation is displayed.
- **US7.3** When the conversation is stopped (Stop discussion, from any client) and
  becomes read-only, its queue is kept visible and is never sent. Each message can
  still be copied or removed; **Resume** and **Queue** are not offered.
- **US7.4** Queued messages are held in memory only. Quitting or restarting
  Sectile Desktop discards them; they are never sent after a restart.

### US8 - Documentation (P2)

- **US8.1** `desktop-tauri/README.md` describes queueing, removal, pause and
  resume, and that the queue is lost when the app quits.
- **US8.2** `desktop-tauri/CHANGELOG.md` has one `Added` line under
  `[Unreleased]` describing the feature for users.

## Functional requirements

- **FR1** The queue is client-side, one per conversation ID, in memory.
- **FR2** Enqueue when the conversation is busy or its queue is not empty;
  otherwise send immediately. Enqueue requires a loaded, non read-only
  conversation.
- **FR3** Dispatch only the head, only when idle (see Vocabulary), not paused,
  and no dispatch is already in flight.
- **FR4** Remove the head from the queue only once the agent acknowledged it.
- **FR5** Pause on local interrupt, on a turn that ended with an error or an
  interruption, and on a send failure. Resume only on explicit user action.
- **FR6** Never dispatch on a read-only conversation.
- **FR7** At most 20 queued messages per conversation, 32,000 characters each.
- **FR8** New UI strings are in English. Existing strings are not changed.

## Edge cases

- The user presses Enter at the moment a turn ends: if the queue is empty and
  the app still sees the conversation busy, the message is queued and sent at the
  next idle snapshot; if it sees it idle, it is sent immediately. Either way it
  is sent once.
- Another client (web, CLI) starts a turn just as the app dispatches: the Go agent
  then joins or buffers the message server-side. Accepted; the app only dispatches
  after a fresh snapshot shows the conversation idle.
- A send whose acknowledgement is lost is reported as failed even if the agent
  received it. The user sees the message both in the transcript and as failed in
  the queue, and removes it; it is not replayed.
- The conversation ends in a read-only state while a dispatch is in flight: the
  in-flight result decides (acknowledged: removed; failed: kept, failed); the
  rest of the queue stays unsent.

## Success criteria

- Automated tests prove dispatch order, one message per turn, removal,
  isolation between two conversations, failure keeping the message and pausing
  the queue, no double send, pause after interrupt, and idle immediate send.
- `npm run check`, `npm test` and `npm run test:ui` pass in `desktop-tauri`.

## Open questions

None. All product questions were settled in clarification round 2.
