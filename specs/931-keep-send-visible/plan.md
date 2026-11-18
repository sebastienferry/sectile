# Plan #931 - Keep Send visible while queueing

Stack: React, TypeScript, CodeMirror, existing ConversationController.
Work branch: `feat/931`; PR base: `feat/tauri-monorepo`.

## Design

- `desktop-tauri/src/components/ProjectPrompt.tsx`: use a single Send button
  with the send icon in Conversation. Keep the existing send-or-enqueue
  disabled rules and use a contextual busy tooltip. Keep Interrupt separate.
- Mirror the current conversation draft in a ref updated whenever the draft
  changes (typing, queue Edit, successful submission). Submission reads this
  ref and consumes it synchronously only after `controller.submit` returns
  sent or queued. React state still drives the editor. A refused submission
  leaves both intact. This prevents stale event handlers from reusing the
  accepted draft before React renders.
- Keep controller and queue algorithms unchanged: they already provide ordered
  dispatch, per-turn options, pause/retry and pending-send serialization.
- Reuse the existing Queued strip and keyboard hint for waiting feedback.
- Update `desktop-tauri/README.md`, root and Tauri `CHANGELOG.md`.

## Rejected alternatives

- Deduplicating by message text would reject intentional repeated messages.
- Disabling Send for an entire active turn would prevent follow-up queueing.
- Changing queue dispatch or backend admission is unnecessary for this UI fix.

## Validation

Extend `desktop-tauri/tests/conversation-queue.ui.test.tsx` with real composer
interactions and batched events before React renders. Update obsolete Queue
button expectations in prompt and command-menu UI tests. Reuse controller
tests for ordering and delivery. Run `npm run check`, `npm test`,
`npm run test:ui`, `npm run build`, `npm run test:rust` in `desktop-tauri/`.
