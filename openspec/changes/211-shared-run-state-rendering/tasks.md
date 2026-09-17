# Tasks

## 1. The shared mapping
- [x] 1.1 Add `runStateOf(run)` to `shared/runStates.ts`: terminal statuses pass through, a wait mark
      on a still-running run yields `waiting`, `queued`/`preparing`/`pending` yield `queued`, the rest
      yields `running`. Accept a `{ status, waitingSince }` shape so an activity and a desktop run
      both fit (D3).
- [x] 1.2 Replace `stateOf()` in `desktop/src/notifications.mjs` with a re-export of `runStateOf`
      under its former name, leaving `transitions()` and its callers untouched (D1).
- [x] 1.3 Extend `desktop/tests/notifications.test.mjs` for `pending`, for a terminal status carrying
      a stale wait mark, and for `preparing`.

## 2. The web badge
- [x] 2.1 Extract `StateGlyph` from `RemoteRunBadge.tsx` into `web/src/components/RunStateGlyph.tsx`,
      taking a state id, a size and a spin flag; import it back in `RemoteRunBadge` (D5).
- [x] 2.2 Rewrite `ActivitiesView.getStatusBadge` to take the activity, derive its state through
      `runStateOf`, and render the shared glyph plus a label looked up by state id in
      `t.activities.stats`, falling back to the shared definition's label (D6).
- [x] 2.3 Key the existing Tailwind class strings by state id with a neutral slate fallback, and drop
      the `switch` (D7).
- [x] 2.4 Remove the duplicate amber waiting pill at the call site, and the `Hand` import it leaves
      behind (D8). `Loader2`, `Clock`, `CheckCircle2`, `AlertTriangle` and `XCircle` stay: the stats
      tiles and the detail panel still use them.
- [x] 2.5 Add a web test over `runStateOf`, the mapping the badge derives from: waiting, pending and
      preparing, a terminal status carrying a stale wait mark, an absent or unknown status, and the
      label fallback for a state the definition does not know.

## 3. The desktop sidebar
- [x] 3.1 Add a helper in `desktop/src/main.js` that builds the derived-state indicator for a run:
      the shared glyph as inline SVG, `data-run-state`, and the label on `aria-label`/`title`.
- [x] 3.2 Show it on the task row (`~:298`), keeping `data-status` on the raw status (D4).
- [x] 3.3 Show it in the execution queue entry (`~:189`), replacing the raw status in the context
      line while keeping the cancel-requested wording.
- [x] 3.4 Style `.run-state` in `desktop/src/style.css`, colouring it from the shared definition and
      keeping the waiting glyph legible against the row background.
- [x] 3.5 Extend `desktop/tests/waiting-notification.ui.cjs` to assert that the row of the session
      the banner was raised for reads as waiting, then follows it to its outcome. The existing
      `[data-status=...]` selectors are covered by `console.ui.cjs` and `task-order-hold.ui.cjs`,
      both re-run unchanged.

## 4. Gates
- [x] 4.1 `make test` (go, web tests, tsc, oxlint).
- [x] 4.2 `npm test` in `desktop/`.
- [x] 4.3 Re-read the diff as a reviewer.
