# Design

## Context
`shared/runStates.ts` holds the six run states and, for each, a label, an announcement, a colour and
a lucide glyph. Two of the four surfaces that show a run state read it. The two that do not each
re-derive the state their own way: `ActivitiesView` switches on the raw activity status, and the
desktop sidebar prints `run.status` verbatim. The mapping that reconciles a raw status and a
`waitingSince` mark into a displayable state exists once already, as `stateOf()` in
`desktop/src/notifications.mjs`, but it is private to the notification layer.

## Goals
- One implementation of "which state is this run in", reachable from every surface.
- A `waiting` badge in the activities view, so its existing `waiting` filter stops lying.
- The derived state visible in the desktop sidebar, matching the banner raised for the same run.
- A seventh state added to `shared/runStates.ts` renders on both surfaces with no edit to either.

## Decisions

### D1. `runStateOf(run)` lands in `shared/runStates.ts`; `notifications.mjs` re-exports it
The mapping is display logic about the shared vocabulary, so it belongs beside the vocabulary. The
desktop keeps the name its callers and its tests already use by re-exporting it:

```js
export { runStateOf as stateOf } from '../../shared/runStates.ts'
```

One implementation, no call-site churn, and `desktop/tests/notifications.test.mjs` keeps testing the
real thing.

*Rejected:* leaving `stateOf` in the desktop and importing it from the web. The web would then
depend on an Electron-side module for a decision about a shared vocabulary, and the dependency runs
the wrong way.

### D2. `pending` folds in with `queued` and `preparing`
`stateOf()` never saw `pending`: it is an activity status, not a run status, and today it falls
through to `running`. The web filter already treats `pending` and `queued` alike. Folding it in makes
the shared mapping agree with what both surfaces already mean, and drops a case from
`getStatusBadge`.

### D3. The input is a shape, not a type
`runStateOf` accepts `{ status, waitingSince }`. A `TaskActivity` and a desktop run both satisfy it,
and neither module has to import the other's type. `waitingSince` is only read while the status is
still `running`, as `remoteRunIndicator.isWaiting` already does, so a stale timestamp on a finished
run cannot make it look alive.

### D4. The desktop keeps `data-status` as the raw status
`console.ui.cjs`, `agent-logs.ui.cjs` and `task-order-hold.ui.cjs` select on
`.run[data-status=running|failed|canceled|completed]`. That attribute is a test selector, not display
text; rewriting it with the derived id would silently break those selectors for `waiting`,
`preparing` and `pending`. The derived state is published separately, as `data-run-state` plus a
visible glyph and an accessible label. The acceptance criterion asks that the sidebar *read* as
waiting, which the visible indicator satisfies.

### D5. The glyph renderer becomes a shared component
`RemoteRunBadge` already turns a shared `IconNode[]` into an inline SVG. That function moves to
`web/src/components/RunStateGlyph.tsx` and both web surfaces use it, so the shared icon nodes are
drawn by exactly one piece of code, at a caller-chosen size.

### D6. Labels come from i18n, keyed by state id, falling back to the shared label
`t.activities.stats.*` is localised FR/EN; `runStates.ts` is English-only and stays that way —
localising it would drag an i18n table into a module the Electron main process imports. The badge
looks the label up by state id in the i18n table and falls back to `definition.label`, so a seventh
state renders in English rather than not at all.

### D7. Tailwind classes stay, keyed by state id, with a neutral fallback
Replacing them with the hex values from `runStates.ts` is explicitly out of scope. Keying the
existing class strings by state id instead of by a `switch` case is what makes D6's "no edit for a
seventh state" true: an unknown state gets the neutral slate classes rather than `null`.

### D8. The duplicate waiting pill goes
`ActivitiesView` renders an amber "waiting" pill beside the badge when `status === 'running' &&
waitingSince`. Once the badge itself reads "waiting", the pill states it twice. It is removed, and
`Hand` along with it.

## Risks
- **The desktop sidebar row gains an element.** The UI tests assert on `.run` text and on
  `data-status`; the glyph is `aria-hidden` with the label carried in `aria-label` and `title`, so
  `textContent` assertions are unaffected.
- **`shared/runStates.ts` is imported by the Electron main process.** `runStateOf` stays pure and
  dependency-free, as the rest of the module already is.
