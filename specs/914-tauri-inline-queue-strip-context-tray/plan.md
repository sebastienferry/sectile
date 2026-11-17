# Plan #914 - inline queue strip and context tray

Stack: React + TypeScript in `desktop-tauri/`. No agent, server or Rust change.

## Queue

- `src/lib/conversation-queue.ts`: `withdraw(queue, id)` returns
  `{ queue, item }` or `undefined` (missing or `sending`); `mergeDraft`.
- `src/lib/conversation-controller.ts`: `withdrawQueued(id)` updates state and
  returns the withdrawn `QueuedMessage`.
- `src/components/QueuedMessages.tsx`: keeps the read-only block
  (`QueuedMessages`, unchanged labels) and adds `QueueStrip` (writable),
  sharing the pause-reason and copy helpers. Accessible names reuse
  `Copy/Edit/Remove queued message N`; collapsed, the head's buttons are N=1,
  expanded they move into the list `Queued messages`.
- `ProjectPrompt.tsx` (Conversation): render `QueueStrip` as the first child
  of `.composer-box`; `QueuedMessages` only when read-only. Edit handler:
  `withdrawQueued`, `setDraft(mergeDraft(draft, item.text))`,
  `setOptions(item.options)`, focus the textarea.

## Context tray

- `src/components/ContextTray.tsx`: `ContextTray({ items })` with
  `ContextTrayItem { key, label, title, icon, onClick? }`. Pills live in a
  wrapping row clipped to one line (`max-height:24px; overflow:hidden`); the
  toggle sits beside the row. A layout effect plus a `ResizeObserver` on the
  row count pills whose `offsetTop` is below the first pill's; those are
  `aria-hidden` and untabbable while collapsed. Collapses when the item keys
  change.
- `FolderContext.tsx`: `PromptFolders` gains `onEntriesChange(entries)`
  (entries already computed for its popover, so no second fetch); the composer
  keeps them in state and builds tray items (copy path on click).
- `App.css`: `.queue-strip*`, `.context-tray*` styles.

## Test plan

- `tests/conversation-queue.test.ts`: `withdraw` (queued, failed, sending,
  missing, pause settled), `mergeDraft`.
- `tests/conversation-queue.ui.test.tsx`: strip inside the composer box,
  Edit merges and restores options and focus, expand/collapse with pause,
  Resume and failure.
- `tests/context-tray.ui.test.tsx`: hidden with one folder, pills and copy,
  `+N more` / `Show less` with stubbed `offsetTop`, re-collapse on change.
