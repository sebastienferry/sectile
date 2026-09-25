# #474: Add Cmd (Ctrl) + B to toggle the sidebar

Ticket: https://github.com/sebastienferry/sectile/issues/474
Branch: `feat/474`.
Clarification: [`docs/clarifications/474.md`](../../docs/clarifications/474.md).

## Context

Both clients have a main sidebar that a button collapses and expands: the web
app's navigation sidebar, and the desktop app's projects sidebar beside the
execution console. Neither can be toggled from the keyboard. The web app also
forgets the collapsed state on every reload, while the desktop app remembers it.

Out of scope: resizing the sidebar, what it contains, any other shortcut, a
configurable keymap, and an entry in the desktop application menu.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

Settled by the owner during the clarification (round 2).

- **D1: One chord per platform.** Cmd+B on macOS, Ctrl+B on Windows and Linux.
  On macOS, Ctrl+B is never captured.
- **D2: The terminal keeps Ctrl+B.** On Windows and Linux, Ctrl+B pressed while
  a terminal has focus goes to the terminal. On macOS, Cmd+B toggles even from
  the terminal.
- **D3: Both clients.** Web and desktop, in this ticket.
- **D4: The web remembers.** The web app keeps the collapsed state across
  reloads, as the desktop app already does.

Settled from the code during the clarification (round 1).

- **D5: Exact chord.** Shift or Alt held, or a key held down (auto-repeat), does
  not toggle.
- **D6: Not behind a modal.** While a modal dialog is open, the shortcut does
  nothing.
- **D7: Bold wins in the editor.** In the Markdown editor, Cmd/Ctrl+B keeps
  inserting bold.
- **D8: Discoverable.** The toggle buttons' tooltips name the shortcut, and the
  web command palette offers the toggle.

---

## User stories

### US1: Toggle the sidebar from the keyboard (P1)

As someone working in Sectile, I want Cmd+B (macOS) or Ctrl+B (Windows, Linux)
to collapse and expand the sidebar, so that I can make room without reaching
for the mouse.

**Acceptance**

1. **Given** the web app on macOS with the sidebar expanded, **when** I press
   Cmd+B, **then** the sidebar collapses; **when** I press Cmd+B again, **then**
   it expands.
2. **Given** the web app on Windows or Linux, **when** I press Ctrl+B, **then**
   the sidebar toggles.
3. **Given** the desktop app on macOS, **when** I press Cmd+B, **then** the
   projects sidebar toggles, and the terminal re-fits the freed width exactly as
   after a click on the toggle button.
4. **Given** the desktop app on Windows or Linux with the focus outside the
   terminal, **when** I press Ctrl+B, **then** the projects sidebar toggles.
5. **Given** a toggle by keyboard, **then** the result is the same as a click on
   the toggle button: same collapsed layout, same button icon and label, same
   remembered state.
6. **Given** the focus in a plain text field (search, title...), **when** I
   press the chord, **then** the sidebar toggles and nothing is typed.
7. **Given** the chord, **then** the browser's own action for it (for example
   Firefox's bookmarks sidebar on Ctrl+B) does not happen.

### US2: Keep the keys that already mean something (P1)

As someone typing in a terminal or an editor, I want the keys I already use
there to keep working, so that the shortcut never costs me a command.

**Acceptance**

1. **Given** the desktop app on Windows or Linux with the focus in the
   terminal, **when** I press Ctrl+B, **then** the sidebar does not move and the
   terminal receives Ctrl+B.
2. **Given** the desktop app on macOS with the focus in the terminal, **when** I
   press Cmd+B, **then** the sidebar toggles and the terminal receives nothing.
3. **Given** macOS, in either client, **when** I press Ctrl+B, **then** the
   sidebar does not move, and the key reaches whatever has focus.
4. **Given** the web Markdown editor with the focus in it, **when** I press
   Cmd/Ctrl+B, **then** bold is inserted and the sidebar does not move.
5. **Given** a bare B with no modifier outside a field, **then** the web app
   still switches to the Board view.
6. **Given** Cmd/Ctrl+Shift+B or Cmd/Ctrl+Alt+B, **then** the sidebar does not
   move.
7. **Given** the chord held down, **then** the sidebar toggles once, not once
   per auto-repeat.

### US3: Nothing happens behind a dialog (P2)

**Acceptance**

1. **Given** a web modal (task detail, quick add, command palette, clone,
   profile, project settings, changelog, board view, batch pickup), **when** I
   press the chord, **then** the sidebar does not move.
2. **Given** a desktop dialog (settings, quick add, command palette...),
   **when** I press the chord, **then** the sidebar does not move.

### US4: The web app remembers the sidebar (P1)

**Acceptance**

1. **Given** I collapsed the web sidebar, by button or by keyboard, **when** I
   reload the page, **then** it is still collapsed.
2. **Given** I expanded it again, **when** I reload, **then** it is expanded.
3. **Given** a browser that never stored the state, or where storage is
   unavailable, **then** the sidebar starts expanded and the app works as
   before.

### US5: Find the shortcut (P2)

**Acceptance**

1. **Given** the web sidebar's collapse or expand button, **when** I hover it,
   **then** its tooltip names the shortcut of my platform (`⌘B` on macOS,
   `Ctrl+B` elsewhere).
2. **Given** the desktop toggle button, **when** I hover it, **then** its
   tooltip names the shortcut of my platform.
3. **Given** the web command palette, **when** I search for the sidebar,
   **then** a command toggles it, showing the shortcut of my platform, and the
   palette closes.

---

## Functional requirements

- **FR1** The chord is Meta+B on macOS and Control+B elsewhere, with no Shift
  and no Alt, not auto-repeated.
- **FR2** The chord toggles only when no modal dialog is open and no earlier
  handler already consumed the key.
- **FR3** On Windows and Linux, the chord pressed with the focus in a terminal
  is left to the terminal untouched.
- **FR4** A handled chord cancels the key's default action and does not reach
  the terminal.
- **FR5** A keyboard toggle goes through the same path as the toggle button.
- **FR6** The web collapsed state persists per browser; an absent, unreadable
  or invalid value means expanded.
- **FR7** Tooltips and the palette command name the platform's chord.
- **FR8** The web strings the user reads stay in French and English through
  `translations.ts`; the desktop strings stay in English, as the rest of that
  UI.
- **FR9** `CHANGELOG.md` gets one line under `[Unreleased]` / `Added`.

## Success criteria

- Unit tests cover the chord rule for every row of US1, US2 and US3 on both
  platforms.
- A web browser regression covers US1.1, US1.6, US2.4, US2.5, US3.1, US4 and
  US5.1, US5.3.
- A desktop UI regression covers US1.3-US1.5, US2.1, US2.2, US3.2 and US5.2.
- `npm run build`, `npm run lint` and `npm test` pass in `web/`; `npm test` and
  the new UI test pass in `desktop/`.
