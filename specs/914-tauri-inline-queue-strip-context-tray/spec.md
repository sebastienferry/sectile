# Specification #914 - Tauri: inline composer queue strip and single-line context tray

Ticket: https://github.com/sebastienferry/sectile/issues/914
Clarification: `docs/clarifications/914.md` (rounds 1 and 2).

## Scope

In: the conversation composer's queue presentation and an Edit action; a
folder context tray in both composers of Sectile Desktop (Tauri).

Out: Electron desktop and web client, "steer now", any change to how the
queue is dispatched, paused or persisted, adding or detaching folders from
the tray (the footer folder popover keeps that role), per-turn file, image or
selection attachments (Q2).

## User stories

### US1 - The queue lives inside the composer box (P1)

1. **Given** a writable conversation with queued messages, **then** a strip at
   the top of the bordered composer box shows a `Queued` tag, a single-line
   truncated preview of the next message to be sent, its status when it is
   not plainly queued (`Sending…`, `Failed: <reason>`), and Copy, Edit and
   Remove buttons acting on it.
2. **Given** an empty queue, **then** no strip is shown.
3. The standalone queue block above the status strip is no longer shown for a
   writable conversation.

### US2 - Edit takes a queued message back into the composer (P1)

1. **Given** a queued or failed message, **when** the user clicks Edit,
   **then** it leaves the queue, its text goes into the composer and the
   composer is focused.
2. **Given** a non-empty draft, **when** the user clicks Edit, **then** the
   queued text is appended after the draft, separated by a blank line (Q1).
3. **When** the user clicks Edit, **then** the message's turn options (model,
   mode) replace the composer's turn settings (Q3).
4. **Given** a message being sent, **then** Edit (like Remove) is disabled.
5. **Given** a paused queue whose last message is edited out, **then** the
   pause is dropped, as for Remove.

### US3 - Several queued messages expand into a list (P1)

1. **Given** more than one queued message, **then** the strip shows the count
   and an expand/collapse toggle.
2. **When** expanded, **then** every message is listed with its position,
   status, Copy, Edit and Remove.
3. **Given** a paused queue, **then** the pause reason and Resume stay in the
   strip header, collapsed or expanded; a failed message shows its error.

### US4 - Stopped conversations keep their queue block (P2)

1. **Given** a read-only conversation holding queued messages, **then** the
   standalone block stays as today ("Not sent: this conversation is stopped",
   Copy, Remove), with no Edit since there is no composer.
   Note: the clarification listed "Copy only"; Remove is kept because it is
   existing behaviour (#905) outside this ticket's scope, and removing it
   would leave unsendable messages stuck until the app quits.

### US5 - A single-line context tray (P1)

1. **Given** a composer (new prompt or conversation) whose context has more
   than the working folder, **then** the top of the composer box shows each
   folder (working first, then attached, context and specification folders)
   as a 24 px pill with its name, the full path and role in its tooltip;
   clicking a pill copies its path.
2. **Given** only the working folder, **then** no tray is shown.
3. **Given** more pills than fit on one row, **then** the overflowing ones are
   hidden behind a `+N more` button; expanded, the tray wraps and the button
   reads `Show less`. The count follows resizes.
4. **When** the folder list changes, **then** the tray collapses again.
5. The tray takes a generic list of items (label, icon, tooltip, action).

## Functional requirements

- FR1. `withdraw(queue, id)` / `controller.withdrawQueued(id)`: refuses a
  `sending` item, returns the item, settles the pause.
- FR2. `mergeDraft(draft, text)`: text alone for a blank draft, else draft
  (trailing whitespace trimmed) + blank line + text.
- FR3. Queue strip per US1-US3; read-only block per US4.
- FR4. Context tray per US5, overflow measured from pill positions on render
  and on `ResizeObserver` callbacks.
- FR5. One `CHANGELOG.md` line under `[Unreleased]`.

## Success criteria

Unit tests for `withdraw` and `mergeDraft`; UI tests for edit-back (merge,
options, focus), multi-item expand/collapse with pause/resume and a failure,
and tray overflow with stubbed geometry.
