# Specification #913 - Tauri: auto-expanding composer and Up/Down prompt history

Ticket: https://github.com/sebastienferry/sectile/issues/913
Clarification: `docs/clarifications/913.md` (rounds 1 and 2).

## Scope

In: the two composers of Sectile Desktop (Tauri) — the "New prompt" view and
the active conversation composer. Auto-grow applies to both; prompt history
applies to the conversation composer only (Q1 -> A).

Out: history persisted across conversations or restarts, history search, the
Electron desktop, the web client, the read-only history footer, queued
messages not sent yet.

## User stories

### US1 - The composer grows with its content (P1)

As a user writing a long prompt, I see all of it without scrolling a 2-row box,
and the transcript stays visible.

1. **Given** an empty composer, **when** it is displayed or focused, **then**
   it rests at about two lines (`min-height: 48px`, `line-height: 1.4`) and
   does not jump on focus.
2. **Given** the user types or pastes several lines, **when** the content
   grows, **then** the composer grows with it line by line.
3. **Given** the content is taller than the maximum height, **when** it grows
   further, **then** the composer stops at the maximum and scrolls internally.
4. **Given** the user clears or sends the draft, **when** the value becomes
   short again, **then** the composer shrinks back.
5. Both the "New prompt" view and the conversation composer behave this way.

### US2 - The maximum height follows the view (P1)

1. The maximum height is `max(150px, 40% of the view height)`, the view being
   the conversation main column (the side pane does not count) or the
   "New prompt" view.
2. **Given** the window or a pane is resized, **when** the view height changes,
   **then** a `ResizeObserver` updates the maximum height.

### US3 - Recall previous prompts of the conversation (P1)

1. **Given** an empty conversation composer and a conversation with sent
   prompts, **when** the user presses `ArrowUp`, **then** the most recent
   prompt is shown, caret at its end.
2. **Given** a recalled prompt, **when** the user presses `ArrowUp` with the
   caret on its first row, **then** the previous prompt is shown; on the
   oldest prompt nothing changes.
3. **Given** a recalled prompt, **when** the user presses `ArrowDown` with the
   caret on its last row, **then** the next prompt is shown; on the newest
   one the composer is cleared to an empty draft.
4. **Given** a recalled prompt, **when** the user presses `Escape`, **then**
   the composer is cleared to an empty draft and a running turn is **not**
   interrupted (Q2 -> A).
5. **Given** an empty or ordinary draft while a turn runs, **when** the user
   presses `Escape`, **then** the turn is interrupted as today.
6. Recalled text is the text the transcript shows for each user message
   (injected trailers stripped); empty prompts are skipped; queued messages
   are not part of the history.

### US4 - Normal editing is never hijacked (P1)

1. **Given** a non-empty draft the user typed, **when** they press
   `ArrowUp`/`ArrowDown`, **then** the caret moves normally and the draft is
   never replaced.
2. **Given** a recalled multi-line prompt, **when** the caret is not on the
   first (for `ArrowUp`) or last (for `ArrowDown`) row, **then** the arrow
   moves the caret.
3. **Given** the user edits a recalled prompt, **then** it becomes an ordinary
   draft and arrows stop recalling.
4. Arrows with Shift, Ctrl, Alt or Meta, and arrows during IME composition,
   never recall.
5. In the "New prompt" view, arrows keep their native behaviour.
6. The recall state is reset when the conversation changes and after a send.

## Functional requirements

- FR1. Auto-grow on both composers, bounded by a dynamic maximum height
  `max(150, 0.4 * viewHeight)` kept in sync by a `ResizeObserver`.
- FR2. Internal vertical scroll only once the maximum is reached.
- FR3. Prompt history navigation on the conversation composer, per US3/US4.
- FR4. `Escape` clears a recalled prompt before it can interrupt a turn.
- FR5. `CHANGELOG.md` gets one `Added` line under `[Unreleased]`.

## Success criteria

- Unit tests cover the history navigation rules; UI tests cover recall, step,
  clear, `Escape` precedence, multi-line caret, IME, modifiers, the new-prompt
  view keeping native arrows, and the maximum height update from a stubbed
  `ResizeObserver`.
