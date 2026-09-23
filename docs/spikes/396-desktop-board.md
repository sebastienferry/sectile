# Spike #396: the board inside Sectile Desktop

Issue: https://github.com/sebastienferry/sectile/issues/396.
Clarification: [`docs/clarifications/396.md`](../clarifications/396.md).
Specification: [`specs/396-desktop-embedded-board/`](../../specs/396-desktop-embedded-board/spec.md).
Decision proposed: [ADR 0025](../adrs/0025-the-board-is-shown-inside-the-desktop.md).

## Question

How should Sectile Desktop show the connected server's board without sending the
user to the browser? The board is the server's existing web board. A board drawn by
the desktop itself and a board that works without a server were ruled out during
clarification.

## Recommendation

Show the server's board in a **dedicated desktop window**, signed in by the
**workstation API key, which the main process adds to the requests bound for the
server**, in a **session of its own held in memory**, with no preload and navigation
limited to the server. Keep the browser as the fallback when the desktop holds no key
for that server. The prototype on `feat/396` does exactly this and passes every check
below. No server or web change is needed to ship it; two small web follow-ups would
polish it.

## Options compared

### Where the board is shown

| Option | For | Against |
| --- | --- | --- |
| **Dedicated `BrowserWindow`** (recommended) | Own `webContents`, own session, own lifecycle; nothing of the main window's rules has to change; the user can put it on another screen next to the consoles. | A second window to manage; closing the main window has to close it too (done). |
| `WebContentsView` inside the main window | One window; board and consoles side by side. | Shares the main window's lifecycle, layout and title-bar overlay; the renderer would have to lay out a native view it does not own; the main window's `will-navigate` block and the board's navigation rules would live on one window. More code for a layout gain the separate window already offers. |

### How the board is signed in

| Option | For | Against |
| --- | --- | --- |
| **Key injected by the main process** (recommended) | Nothing to type; the identity is the paired user by construction (ADR 0011); the key never enters page script; works for `EventSource`, which cannot set a header; a revoked or expired key signs the board out at once. | The board has no session of its own, so the web "Sign out" cannot work (see below). |
| Web sign-in inside the window (OIDC redirect or local e-mail) | The web works exactly as in a browser, sign-out included. | A second sign-in on a workstation that is already paired; OIDC redirects leave the server origin, so the navigation rules would have to allow the identity provider; the board could be signed in as somebody other than the workstation's user; cookies would have to be persisted to avoid signing in at every launch. |
| Key handed to the page (query string, `localStorage`) | Trivial. | The key becomes readable by page script and by anything the page loads; rejected. |

### How it is isolated

- **No preload**: the page reaches none of the desktop's IPC (`window.localAgent`
  is `undefined` in the board, checked by `board-window.ui.cjs`).
- **Session partition per server origin, not persisted**: nothing the page stores
  survives the window, and it never meets the desktop renderer's storage. Another key
  or another server replaces the window and clears the partition.
- **Key scoping**: the header is added only when the request's origin is the server's,
  the request comes from the board's `webContents`, and the frame issuing it (when
  known) is the server's own. Another origin fetched by the page gets no key.
- **Navigation**: server pages stay; any other `http(s)` link, including
  `target="_blank"`, goes to the default browser; every other scheme is refused.
  Redirects follow the same rule.
- **Permissions**: all refused except `clipboard-sanitized-write`, which the board's
  copy buttons use.
- **`/auth/*` refused from the window**: the key is the only identity the board may
  have. A key that stops working leaves the board signed out, never signed in as
  somebody else.

## What was verified

### Server side, without a change (T1)

Read in `internal/handlers/pairing.go`, `auth.go`, `agent_api.go`,
`usercredentials.go`, `handlers.go`, and pinned by
`internal/handlers/boardkey_test.go`:

- `webSessionUser` resolves `Authorization: Bearer <workstation key>` exactly like a
  session cookie, so `RequireSession` lets a key-signed request through every
  `/api/` route. `/api/me` answers `signedIn: true` with the key's owner.
- Writes are attributed to the key's owner (`actingContext` reads the same
  `webSessionUser`).
- The event stream `/api/events` opens with the key and refuses without it.
- Personal tracker credentials (`/api/me/tracker-credentials`, `unlock`) are keyed by
  user, not by session: the board lists and unlocks them like a browser.
- Only the agent APIs (`AgentAPIAuth`, pairing) refuse a request carrying an
  `Origin`; web routes do not, so the board's own-origin requests are accepted. There
  is no CSRF token to satisfy: the web relies on the cookie being `SameSite`, and a
  key-signed request carries no cookie at all.
- `POST /auth/logout` answers `signedOut: true` but only revokes a cookie session:
  the key keeps answering for its owner. An expired key, by contrast, is refused on
  every guarded route and `/api/me` reads signed out.

### Prototype, automated

- `desktop/tests/board-window.test.cjs` (8 tests): origin validation, board URL and
  `?task=`, navigation verdicts, key scoping by origin, window and frame, refused
  `/auth/` routes, permissions, single window reused, replacement on another key or
  server with the partition cleared, a window closed by the user not reused.
- `desktop/tests/board-window.ui.cjs` (real Electron): the **Connected** link opens a
  second app window and no browser; the page has no `localAgent`; `/`, `/api/me` and
  the `EventSource` stream all carry the key; another origin reached by the page
  receives no key; `/auth/logout` is refused before reaching the server; a PR link
  goes to the browser; the board is reused; `openTask('#7')` loads `?task=%237` in the
  same window; a key saved for another server makes the link fall back to the browser.
- `desktop/tests/board-link.ui.cjs` and `contract-mismatch.ui.cjs` still pass with no
  stored key: the link opens the browser exactly as before.

### Prototype, against a real server (manual, 2026-09-23)

A throwaway `bin/server` built from this branch (SQLite in a scratch directory,
local e-mail sign-in), a key issued from `/api/devices`, and the desktop's board
window pointed at it:

| Check | Result |
| --- | --- |
| Board opens signed in, no sign-in screen | yes: `spike@example.com`, role admin, sidebar and Kanban rendered |
| A card created with the key is on the board | yes |
| A write from the board page (`PUT /api/tasks/<id>`) | 200, seen by a separate browser session |
| A stage transition made by another session reaches the open board | yes: `task_updated` received over the key-signed stream |
| Sign-out from the board | refused by the window; the web shows its "could not sign out" message |

### Prototype, on a real deployment (owner, 2026-09-23)

The branch's desktop, run on the owner's paired workstation against the team's
deployed server, with the real local agent:

| Check | Result |
| --- | --- |
| **Connected** opens the board in a desktop window | yes, signed in, no browser launched |
| **Discuter** from a card's **…** menu | the session's console appears in the desktop's execution list |

## What remains unproven

- **OIDC deployments.** Not needed by the recommended approach (the key bypasses
  sign-in), but not exercised against a real identity provider.
- **A stage skill launched by hand from the board.** The owner launched *Discuter*,
  which takes the same dispatch path to the agent; an *Avancer* launch was not tried.
- **Drag-and-drop by hand.** Card moves were exercised as the API writes they are,
  not by dragging in the window.
- **Packaging.** Only the unpackaged app was run; nothing in the approach depends on
  packaging.

## Follow-ups (not in this spike)

1. **Web: know when the caller is a key.** `/api/me` could say the caller is a
   workstation key, so the web hides **Sign out** (and the sign-in screen offers no
   form) in the desktop board instead of showing "could not sign out".
2. **Desktop: a board button of its own.** Today the **Connected** link and the
   open-task actions reach the board; a sidebar entry would make it discoverable.
3. **Desktop: "Open in browser"** from the board window, for people who want both.
4. Decide, in the ADR review, whether the browser fallback stays once every desktop
   holds a key.
