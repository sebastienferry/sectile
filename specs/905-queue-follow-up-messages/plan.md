# Plan #905 - Tauri: queue follow-up messages while an agent is responding

## Stack

- React 19, TypeScript, Vite in `desktop-tauri/src/`.
- Tests: Vitest + Testing Library (`tests/*.ui.test.tsx`, jsdom), Node test
  runner (`tests/*.test.ts`) for pure modules.
- No Go, Rust or API change. The app keeps calling the existing
  `control_conversation` `message` and `interrupt` actions.

## Architecture

The queue lives in `ConversationController`
(`desktop-tauri/src/lib/conversation-controller.ts`), which already exists once
per conversation ID in a module-level map and survives view switches. Putting
the queue there gives isolation per conversation (US4) and background dispatch
(US3.5) by construction. The React composer only renders the controller's state
and calls its methods.

### 1. Pure queue module

New file `desktop-tauri/src/lib/conversation-queue.ts`, free of Tauri imports so
it is testable with the Node runner:

```ts
export const QUEUE_LIMIT = 20;
export const MESSAGE_LIMIT = 32000;
export type QueueItemStatus = "queued" | "sending" | "failed";
export interface QueuedMessage {
  id: string;            // local, crypto.randomUUID()
  text: string;
  options: TurnOptions;  // captured at enqueue (US1.7)
  status: QueueItemStatus;
  error?: string;        // set when status === "failed"
}
export type PauseReason = "interrupt" | "turnError" | "sendFailed";
export interface MessageQueue { items: QueuedMessage[]; paused?: PauseReason }

export function enqueue(q, text, options): MessageQueue | "full" | "empty";
export function remove(q, id): MessageQueue;           // refuses "sending"
export function turnOutcome(events: ConversationEvent[]): "ok" | "error" | "interrupted";
```

`turnOutcome` reads the transcript tail after the last `user` event: an event of
kind `error`, or a `notice` whose text is `Interrupted`, ends the turn badly.
These are the end-of-turn events the Go agent writes in
`internal/agent/agent_conversation.go` (`conversationWrite(run.trace, "error", …)`
and the `"Interrupted"` notice). This couples the app to those two strings; a
test pins them so a wording change in Go fails loudly. Move `ConversationEvent`
and `decodeEvents` imports from `conversation.ts` as type-only to keep the module
Tauri-free (or duplicate the minimal event type).

### 2. Controller changes

`ConversationState` gains `queue: MessageQueue` (initially `{ items: [] }`).

New members:

- `private dispatching = false`
- `private lastBusy = false`: the `busy` seen in the previous applied snapshot.
- `enqueue(text, options): "queued" | "full" | "empty" | "readOnly"`
- `removeQueued(id)`
- `resumeQueue()`: clears `paused`, resets a `failed` head to `queued`, calls
  `maybeDispatch()`.
- `submit(text, options)`: the single entry point for the composer:
  `if (busy || queue.items.length) return enqueue(...)` else `send(...)`.
- `private maybeDispatch()`: sends the head when all hold: `loaded`, not
  `pending`, no `error`, `!snapshot.busy`, `!snapshot.readOnly`,
  `!snapshot.approvals?.length`, `!sending`, `!dispatching`, `!queue.paused`,
  head exists with status `queued`.

Dispatch sequence (`private async dispatchHead()`):

1. `dispatching = true`; mark the head `sending`; `revision++`.
2. `await controlConversation(id, { action: "message", message, ...options })`.
3. Success: drop the head from `queue.items`; `refreshQueued = true`.
   Failure: head becomes `failed` with `String(error)`, `queue.paused =
   "sendFailed"`. Never retried (same rule as the existing `send`).
4. `dispatching = false`; refresh as `send` does today.

Hooks into the existing flow:

- In `refresh`, after a successful read whose revision is current: if
  `lastBusy && !snapshot.busy` and `turnOutcome(decodeEvents(snapshot)) !== "ok"`,
  set `queue.paused` to `turnError` (or `interrupt`). Then update `lastBusy` and
  call `maybeDispatch()`.
- In `control`, when `input.action === "interrupt"` and the queue is not empty,
  set `queue.paused = "interrupt"` before calling the agent.
- `retained` also returns true when `queue.items.length > 0` (US4.3). The
  eviction loop in `conversationFor` already skips retained controllers.
- Polling: keep the 750 ms poll while busy. Additionally, when a read fails and
  the queue holds dispatchable items (not paused), schedule a retry read every
  5 s so reconnection resumes dispatch for a conversation not on screen (US7.2).
  When the view is on screen, the existing reconnection refresh does the same.
- `send` stays as is for the immediate path and for `ProjectPrompt` (new prompt,
  `newlyCreated = true`).

Every queue mutation goes through `update({ queue: … })` with a new object so
`useSyncExternalStore` re-renders.

### 3. Composer changes (`ProjectPrompt.tsx`, `Conversation`)

- Submit handler: replace the `!busy` guard with `controller.submit(draft,
  options)`. Clear the draft on `"queued"` or immediate send; keep it on
  `"full"`, showing the US1.5 message in `composer-feedback`.
- Submit allowed when `loaded && !snapshot.readOnly && !attaching &&
  draft.trim()` and either the immediate-send conditions hold (unchanged:
  `agentAvailable && !busy && !error`) or the submission is going to the queue.
- Buttons, while busy and not read-only: the existing Interrupt button plus a
  new `Queue` button (`aria-label="Queue message"`, `title="Queue · Enter"`)
  replacing the hidden Send button. The Esc handler is unchanged.
- New component `QueuedMessages` in
  `desktop-tauri/src/components/QueuedMessages.tsx`, rendered just above the
  composer form, and also in the read-only branch (US7.3) with `readOnly`:
  - heading `Queued (N)`; when paused, `Queue paused · <reason>` and a
    `Resume` button (hidden when read-only);
  - one `<li>` per item: position, text (clamped to 3 lines, full text in
    `title`), status label (`Queued`, `Sending…`, `Failed: <error>`), `Copy`
    (`navigator.clipboard.writeText`) and `Remove` (disabled while `sending`).
  - `role="list"`, `aria-label="Queued messages"` for tests and accessibility.
- Styles in `desktop-tauri/src/App.css`: `.queued-messages`, `.queued-message`,
  `.queued-message.failed`, muted/dashed border to distinguish from sent bubbles.
- Placeholder while busy: `Queue a follow-up…`. Shortcut line while busy:
  `Enter to queue · Esc to interrupt`.

### 4. Strings (English, new only)

`Queue message`, `Queue · Enter`, `Queued (N)`, `Queued`, `Sending…`,
`Failed: <error>`, `Queue paused`, `after an interrupt`, `after an error`,
`send failed`, `Resume`, `Copy`, `Remove`, `Queue full (20 messages). Remove one
to add another.`, `Queue a follow-up…`, `Enter to queue · Esc to interrupt`.

## Target files

| File | Change |
| --- | --- |
| `desktop-tauri/src/lib/conversation-queue.ts` | new, pure queue logic |
| `desktop-tauri/src/lib/conversation-controller.ts` | queue state, submit, dispatch, pause, retained |
| `desktop-tauri/src/components/QueuedMessages.tsx` | new, queue list |
| `desktop-tauri/src/components/ProjectPrompt.tsx` | composer submit and buttons, render list |
| `desktop-tauri/src/App.css` | queue styles |
| `desktop-tauri/tests/conversation-queue.test.ts` | new, pure module tests |
| `desktop-tauri/tests/conversation-queue.ui.test.tsx` | new, controller and composer tests |
| `desktop-tauri/README.md` | conversation section |
| `desktop-tauri/CHANGELOG.md` | `[Unreleased]` / `Added` |

## Test plan

Pure module (`node --test`):
- enqueue appends in order; refuses empty/whitespace; refuses the 21st item;
  refuses more than 32,000 characters.
- remove keeps order; refuses a `sending` item.
- `turnOutcome`: `ok` on `Ready for your next message`; `error` on an `error`
  event after the last user event; `interrupted` on the `Interrupted` notice;
  an `error` before the last user event does not count.

Controller and composer (Vitest, `vi.mock("../src/lib/conversation")` as in
`tests/prompt.ui.test.tsx`; fake timers for polling):
- **Order / one per turn**: busy snapshot, queue A, B; snapshot idle → only A
  sent; snapshot busy then idle → B sent; `controlConversation` called with A
  then B, never merged.
- **No interrupt on enqueue**: queueing while busy never calls
  `controlConversation` with `interrupt` or `message`.
- **Idle immediate send**: idle, empty queue → sent at once, no queue list.
- **Order kept when idle with a queue**: paused queue [A], submit B → B queued
  after A, not sent.
- **Removal**: queue A, B; remove A; idle → only B sent.
- **Isolation**: controllers X and Y, each with a queued item; X goes idle → only
  X's message sent, to X's ID; Y untouched.
- **Retained**: a controller with a queue survives 20 more `conversationFor`
  calls.
- **Failure**: `controlConversation` rejects → item stays, `failed` with the
  error, queue paused, no second call; Resume → second call; success → removed.
- **No double send**: two refreshes resolving idle at once → one call.
- **Pause on interrupt**: Interrupt with a queue → paused, nothing sent on idle.
- **Pause on turn error**: busy → idle with an `error` event → paused.
- **Approval**: idle snapshot with a pending approval → nothing sent.
- **Read-only**: read-only snapshot → nothing sent; list shows Copy and Remove,
  no Resume.
- **UI**: while busy, Enter adds to the list with `Queued` label and clears the
  draft; Queue button present beside Interrupt; full queue shows the message and
  keeps the draft.

Commands: `npm run check`, `npm test`, `npm run test:ui` in `desktop-tauri`.

## Rejected alternatives

- **Server-side queue (reuse Go `run.conversation.next`)**: merges buffered
  messages into one turn, joins the running Claude turn through stdin, writes
  messages to the transcript immediately and offers no removal. Contradicts
  US2.3, US3.1 and US3.2, and the ticket excludes Go changes.
- **Persisting the queue** (local storage or companion): rejected by the owner
  (Q1); a restart must never send messages behind the user's back.
- **Explicit turn-outcome field in the snapshot**: cleaner than reading the
  transcript tail, but needs a Go and API change out of scope. Reconsider if
  `turnOutcome` proves brittle.
- **Automatic retry after a send failure**: an ambiguous acknowledgement could
  duplicate a prompt; the controller already refuses to replay.
