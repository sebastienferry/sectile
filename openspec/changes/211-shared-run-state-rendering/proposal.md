# Read the run-state glyph and label from the shared definition on every surface

## Why
`shared/runStates.ts` was introduced by #174 as the one definition of what a run state looks like,
and its own header states the intent: every surface that shows a run state reads it there. Four
surfaces show one, and only two of them do.

The drift is already observable. The activities view offers a `waiting` filter whose predicate is
correct, but feeds `getStatusBadge` the raw activity status, and `waiting` is not one: filtering on
"waiting" lists the blocked runs under a blue "Running" pill. The desktop sidebar writes
`run.status` verbatim, so a session blocked on a permission prompt reads "running" in the list while
the banner it just raised says "is waiting for you". The mapping that resolves the difference,
`stateOf()`, is private to `desktop/src/notifications.mjs` and serves notifications alone.

## What Changes
- Promote `stateOf()` into `shared/runStates.ts` as `runStateOf()`, so the mapping from a run or an
  activity to its state id has one implementation. `notifications.mjs` re-exports it under its
  former name for its callers.
- Fold `pending` in with `queued` and `preparing` in that mapping: it is the activity status for the
  same thing, and the web filter already treats the two alike.
- Render `ActivitiesView.getStatusBadge` from the shared definition, keyed by the derived state,
  including a real `waiting` badge, so the filter and the badge finally agree. The separate amber
  "waiting" pill beside the badge goes: it said the same thing twice.
- Extract the inline glyph renderer of `RemoteRunBadge` into a component both web surfaces use, so
  the shared icon nodes are drawn by one piece of code.
- Show the derived state in the desktop sidebar — the shared glyph plus its label, on the task row
  and in the execution queue — instead of the raw status string.

## Impact
- New: `runStateOf()` in `shared/runStates.ts`, `web/src/components/RunStateGlyph.tsx`, a
  `run-state` indicator in the desktop sidebar.
- Changed: `desktop/src/notifications.mjs`, `desktop/src/main.js`, `desktop/src/style.css`,
  `web/src/components/ActivitiesView.tsx`, `web/src/components/RemoteRunBadge.tsx`.
- Unchanged: the run lifecycle, the waiting report, the activity statuses the server stores, and the
  `data-status` attribute the desktop UI tests select on.

## Non-goals
- Replacing the duplicated Tailwind colour classes with the hex values from `runStates.ts`. Worth
  doing, tracked separately so this change stays reviewable.
- Localising `shared/runStates.ts`. The web keeps reading its labels from the i18n table, and falls
  back to the shared label for a state the table does not know.
