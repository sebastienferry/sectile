# Tasks #905 - Tauri: queue follow-up messages while an agent is responding

## Phase 1: Pure queue logic

- [x] 1.1 Create `desktop-tauri/src/lib/conversation-queue.ts` with `QueuedMessage`, `MessageQueue`, `enqueue`, `remove`, limits (20 items, 32,000 characters) and `turnOutcome` (plan §1).
- [x] 1.2 Write `desktop-tauri/tests/conversation-queue.test.ts` covering order, limits, removal and turn outcomes, including the Go end-of-turn strings `Interrupted` and the `error` event kind.

## Phase 2: Controller

- [x] 2.1 Add `queue` to `ConversationState` and `submit`, `enqueue`, `removeQueued`, `resumeQueue` to `ConversationController`.
- [x] 2.2 Implement `maybeDispatch` / `dispatchHead`: one item in flight, removed only on acknowledgement, failure marks it `failed` and pauses (US3, US6).
- [x] 2.3 Detect busy → idle transitions in `refresh`; pause on a turn ending with an error or an interruption; call `maybeDispatch` after each applied snapshot (US5.2).
- [x] 2.4 Pause on a local `interrupt` in `control` (US5.1).
- [x] 2.5 Count a non-empty queue as `retained`; retry reads every 5 s after a read error while a dispatchable queue exists (US4.3, US7.2).
- [x] 2.6 Never dispatch while read-only, with a pending approval or question, or with a controller error (US3.4, US7.3).

## Phase 3: Composer UI

- [x] 3.1 Create `desktop-tauri/src/components/QueuedMessages.tsx` (list, statuses, Copy, Remove, paused banner, Resume) (US2, US5.3).
- [x] 3.2 Route the `Conversation` submit through `controller.submit`; keep immediate send when idle with an empty queue (US1.3, US1.4).
- [x] 3.3 Add the Queue button beside Interrupt while busy; busy placeholder and shortcut line; full-queue feedback keeping the draft (US1.2, US1.5).
- [x] 3.4 Render `QueuedMessages` in the read-only branch without Resume (US7.3).
- [x] 3.5 Add queue styles to `desktop-tauri/src/App.css`, distinct from sent messages (US2.2).

## Phase 4: Tests

- [x] 4.1 Write `desktop-tauri/tests/conversation-queue.ui.test.tsx` covering the controller and composer cases listed in the plan's test plan (order, one per turn, removal, isolation, retained, failure and resume, no double send, pause on interrupt and turn error, approval, read-only, UI).
- [x] 4.2 Check existing `tests/prompt.ui.test.tsx` and `tests/conversation-mcp.ui.test.tsx` still pass; adjust only assertions tied to the removed busy guard.
- [x] 4.3 Run `npm run check`, `npm test` and `npm run test:ui` in `desktop-tauri`.

## Phase 5: Documentation

- [x] 5.1 Describe queueing, removal, pause/resume and loss on quit in the conversation section of `desktop-tauri/README.md`.
- [x] 5.2 Add one `Added` line under `[Unreleased]` in `desktop-tauri/CHANGELOG.md`.

## Phase 6: Manual verification

- [ ] 6.1 With `npx tauri dev`, queue two messages during a Claude Code answer and a Codex answer; check they run as two separate turns in order, that Remove works, that Esc pauses the queue and Resume continues it.
