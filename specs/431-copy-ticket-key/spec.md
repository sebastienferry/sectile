# Copy a ticket's key from its page

Scope restated from the clarification (`docs/clarifications/431.md`, two
rounds, confirmed by the owner). Implementation choices are in `plan.md`.

## User stories

### P1: Copy the ticket's key in one click

As someone reading a ticket in the web app, I want a button next to its key
that puts that key on my clipboard, so that I can paste `#431` or `SFE-123`
into a commit message, a pull request or a chat without selecting text by hand
or opening the tracker.

### P2: Copy the parent's key the same way

As someone reading a ticket that belongs to a macro or an epic, I want the
parent's key to have its own copy button, so that I can reference `M-7` or the
Jira epic just as easily.

### P3: Know whether the copy worked

As someone who just clicked a copy button, I want to see that the key is on my
clipboard, or to be told it is not, so that I never paste something stale
believing it is the key.

## Functional requirements

1. **Where.** The web ticket page, `TaskDetailModal`, in both of its layouts
   (side panel and modal). The desktop app is not changed.
2. **Task key button.** A copy button is always shown right after the task's
   key in the reference badge. It copies the key exactly as the badge displays
   it: `#431` for GitHub, `SFE-123` for Jira, the local key for a local board.
   Nothing else is added: no title, no prefix, no internal id, no URL, no
   trailing whitespace.
3. **Parent key button.** When the ticket has a parent key, a second copy
   button is shown right after the parent's key, before the `/` separator. It
   copies the parent key alone (`M-7`, `SFE-12`). When the ticket has no
   parent, only the task key button is shown.
4. **Links unchanged.** The task key and the parent key keep opening the
   tracker as today, in a new tab. Clicking a copy button never opens a tab or
   navigates. When a key has no tracker URL (local board, parent without URL),
   its button still works.
5. **Labels.** Each button has a tooltip and an accessible name that name the
   key it copies: `Copier #431`, `Copier M-7`.
6. **Success feedback.** After a successful copy, that button's icon changes
   from a copy icon to a check mark for 2 seconds, then reverts. The other
   button is unaffected. A success toast is shown whose title is
   `Identifiant copié` and whose description names the copied key
   (`#431 a été copié dans le presse-papiers.`).
7. **Failure feedback.** When the clipboard is unavailable or refuses the
   write (insecure context, denied permission), no check mark is shown and an
   error toast is shown, titled `Copie impossible`, with the description
   `Le presse-papiers n'est pas accessible depuis ce navigateur.`
8. **Repeated clicks.** Clicking again while the check mark is shown copies
   again and restarts the 2-second delay from that click.
9. **Nothing else changes.** The board card's "Copier la référence" menu
   entry, the specification copy button and every other part of the page are
   unchanged. No server, API or database change.
10. `CHANGELOG.md` carries one `Added` line under `[Unreleased]`.

## Acceptance scenarios

- **Given** GitHub ticket `#431` with parent macro `M-7`, **when** the page
  opens in panel layout, **then** the badge reads `M-7 [copy] / #431 [copy]`,
  with one copy button right after each key.
- **Given** the same ticket, **when** the user clicks the button after `#431`,
  **then** the clipboard holds exactly `#431`, that button shows a check mark,
  the `M-7` button still shows the copy icon, a success toast names `#431`,
  and no new tab is opened.
- **Given** the same ticket, **when** the user clicks the button after `M-7`,
  **then** the clipboard holds exactly `M-7` and the toast names `M-7`.
- **Given** a check mark shown after a copy, **when** 2 seconds pass, **then**
  the copy icon is back.
- **Given** Jira ticket `SFE-123` without a parent, **when** the page opens,
  **then** exactly one copy button is shown, and clicking it puts `SFE-123`
  on the clipboard.
- **Given** a local-board ticket with no tracker URL, **when** the user clicks
  its copy button, **then** its local key is on the clipboard.
- **Given** the modal layout, **when** the page opens, **then** the same
  buttons are present and behave the same.
- **Given** a browser whose clipboard write fails, **when** the user clicks a
  copy button, **then** an error toast is shown, no check mark appears, and no
  success toast is shown.
- **Given** the page opens, **when** the user hovers the button after `#431`,
  **then** its tooltip reads `Copier #431`; a screen reader announces the
  same name.
- **Given** the user clicks the `#431` link itself, **then** the tracker opens
  in a new tab as before and nothing is copied.

## Out of scope

- The desktop app (its execution header may get a copy button in a separate
  ticket).
- Copying the internal id, the title, the branch or the URL.
- The board card's "Copier la référence" menu entry.
- Keyboard shortcuts for copying.

## Open requirements

None. The clarification settled every product question.
