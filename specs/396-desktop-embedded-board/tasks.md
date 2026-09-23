# #396: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Verify the server side (plan: points to verify)

- [ ] T1.1 Against a local server, call the board's routes with `Authorization: Bearer <key>` and no cookie: `/api/me`, `/api/tasks`, `/api/events`, `/api/events/sse`, a stage move, a skill launch, `/api/me/tracker-credentials` and `unlock`. Record each answer.
- [ ] T1.2 Check mutating routes for an Origin or CSRF check that would refuse a keyed request from a server-served page; record the result.
- [ ] T1.3 Record what `POST /auth/logout` does without a cookie.

## 2. Board window (FR1, FR2, FR5, FR6)

- [ ] T2.1 `desktop/electron/board-window.cjs`: create or focus one `BrowserWindow` with a partition per server origin, no preload, sandbox and context isolation.
- [ ] T2.2 `onBeforeSendHeaders` on that partition adds the stored key for the server origin only.
- [ ] T2.3 Deny permission requests on the partition.

## 3. Navigation (FR4, FR6)

- [ ] T3.1 `will-navigate` and `setWindowOpenHandler`: server origin stays, other `http(s)` URLs go to `shell.openExternal`, anything else is denied.

## 4. Desktop actions (FR1, US3)

- [ ] T4.1 `open-board` and `open-task` in `main.cjs` route to the board window; `open-task` loads `?task=<id>`; the existing URL validation is kept.
- [ ] T4.2 Disconnection, re-pairing or key change closes or reloads the window and clears its partition storage (FR7).

## 5. Tests

- [ ] T5.1 `desktop/tests/board-window.test.cjs`: origin matching, header scoping, external-link routing, `?task=` URL.
- [ ] T5.2 Adapt `desktop/tests/board-link.ui.cjs` to the embedded window (opened, reused, disconnected error).
- [ ] T5.3 Run `npm --prefix desktop test`, `npx vite build` then `npm --prefix desktop run test:ui`, and `make test`.
- [ ] T5.4 Manual scenarios of the plan's test plan, results recorded in the finding.

## 6. Decision (US5, FR8)

- [ ] T6.1 `docs/spikes/396-desktop-board.md`: options compared (window vs view, injected key vs in-app sign-in, isolation), T1 results, manual results, recommendation, what remains unproven.
- [ ] T6.2 `docs/adrs/0024-the-board-is-shown-inside-the-desktop.md`, status *Proposed*, amending ADR 0003.
- [ ] T6.3 `desktop/README.md` and `CHANGELOG.md` updated if the prototype stays as user-visible behaviour on the branch.
