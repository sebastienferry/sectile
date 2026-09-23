# #396: Plan

Spec: [`spec.md`](spec.md). Stack: Electron main process (`desktop/electron/`),
vanilla-JS renderer (`desktop/src/`), the Go server's web routes
(`internal/handlers/`) read but not changed, the React web UI (`web/src/`) read but
not changed.

## What exists today

- `desktop/electron/main.cjs`: one `BrowserWindow`, sandboxed, `contextIsolation`,
  preload bridge, `setWindowOpenHandler(deny)` and `will-navigate` prevented, loading
  the local `dist/index.html`. `open-board` and `open-task` validate the server URL
  and call `shell.openExternal`.
- `internal/handlers/pairing.go` `webSessionUser`: a request is identified by the
  session cookie **or** by `Authorization: Bearer <workstation API key>`
  (`bearerToken` in `agent_api.go`). `RequireSession` therefore lets a keyed request
  through every `/api/` route the web board uses, and `/api/me` answers the paired
  user.
- The web board reads `?task=<id>` on load (`web/src/context/AppContext.tsx`) and
  subscribes with `EventSource` to `/api/events` and `/api/events/sse`. `EventSource`
  cannot set headers.
- The key the desktop earned by pairing lives in the main process's settings
  (`storedKey`), which is where it must stay.

## Prototype hypothesis (to be confirmed or refuted by the spike)

```
 desktop renderer ──ipc open-board / open-task──► main process
                                                   │
                                                   ▼
                           board BrowserWindow (one instance)
                           · session partition  persist:board-<serverOrigin hash>
                           · no preload, sandbox, contextIsolation
                           · loadURL(serverURL [?task=id])
                                                   │ every request to serverOrigin
                                                   ▼
             session.webRequest.onBeforeSendHeaders → Authorization: Bearer <key>
                                                   │
                                                   ▼
                                 sectile-server  (cookie-less, key-identified)
```

1. **Placement: a dedicated `BrowserWindow`.** Simplest isolation: its own
   `webContents`, no preload, its own lifecycle. The alternative, a
   `WebContentsView` inside the main window beside the consoles, is prototyped only
   if time allows and compared in the finding.
2. **Authentication: the key injected by the main process.** A dedicated session
   partition; `onBeforeSendHeaders` filtered on the server origin adds
   `Authorization: Bearer <key>` to every request, including `EventSource` and
   navigations, so the key never reaches page JavaScript. The alternative, running the
   web sign-in (OIDC redirect or local e-mail) inside the partition, is assessed in
   the finding, not built.
3. **Isolation.** `will-navigate` and `setWindowOpenHandler` allow the server origin
   only and hand every other `http(s)` URL to `shell.openExternal`; everything else is
   denied. Permission requests (`setPermissionRequestHandler`) are denied. The main
   window's rules are unchanged.
4. **Lifecycle.** `open-board` and `open-task` create or focus the window; `open-task`
   loads `?task=<id>`. On disconnection, key change or server change, the window is
   closed or reloaded and its partition storage cleared, so nothing from a previous
   identity survives. A 401 from the server shows the web board's own
   non-authenticated state.

## Points the spike must verify, not assume

- `RequireSession` and every route the board calls accept the key: in particular
  `/api/events*` (SSE), `/api/me/tracker-credentials` and its `unlock` (ADR 0014,
  sealed personal credentials), launches and stage moves.
- Whether an Origin or CSRF check on mutating routes refuses a keyed request coming
  from a page served by the server itself.
- What the web **Sign out** does with no cookie (`POST /auth/logout`), and how the
  board should present it.
- That attribution of writes (`actingContext`) is the paired user's, as for a browser
  session.
- That a skill launched from the embedded board reaches this workstation's agent and
  appears in the desktop's execution list.

## Rejected for this spike (per clarification)

- A native desktop board (duplicating `web/src/components/BoardView.tsx`).
- A serverless board with a workstation-side store.

## Target files

| File | Change |
| --- | --- |
| `desktop/electron/main.cjs` | board window, partition, header injection, navigation rules, `open-board` / `open-task` routed to it, teardown on disconnect/re-pair |
| `desktop/electron/board-window.cjs` (new) | the board window's creation and rules, kept out of `main.cjs` so it can be unit tested |
| `desktop/tests/board-window.test.cjs` (new) | origin filter, header injection only for the server origin, external-link routing |
| `desktop/tests/board-link.ui.cjs` | the existing board-link test now expects the embedded window, not `openExternal` |
| `docs/spikes/396-desktop-board.md` (new) | the finding |
| `docs/adrs/0024-the-board-is-shown-inside-the-desktop.md` (new) | ADR draft, *Proposed*, amending ADR 0003's "navigation is blocked" |
| `desktop/README.md`, `CHANGELOG.md` | only if the prototype is kept as user-visible behaviour on the branch |

No server, web UI, migration or dependency change is expected. If the verification
above shows a server change is needed (for example an SSE route refusing the key),
the spike records it in the finding and the ADR rather than making it.

## Test plan

- Unit (`npm --prefix desktop test`): origin matching (scheme, host, port, base path),
  header added only on the server origin, never on other origins; navigation to
  another origin handed to `openExternal`; `open-task` URL built with `?task=`.
- UI (`npx vite build` then `npm --prefix desktop run test:ui`): `board-link.ui.cjs`
  adapted to assert a board window opens and is reused, and that a disconnected
  desktop shows the existing error.
- Manual, recorded in the finding: against a local server, board signed in as the
  paired user, card move, task detail, skill launch reaching the desktop console, live
  update from another browser session, external PR link, key revoked while open.
- `make test` for non-regression of Go and web.
