# Plan #581 - Board filters are respected and kept

Implements `specs/581-board-filters-not-respected/spec.md`. Behaviour lives in
the spec; this file says where and how.

## Stack and constraints

- Web only: React + TypeScript, `web/src`. Unit tests `web/tests/*.test.mjs`
  (`node --test`, importing `../src/lib/*.ts` directly), browser tests
  `web/tests/*.browser.mjs` (Playwright, see README "Browser tests").
- No server, database, desktop or tracker change. The requests sent to
  `GET /api/tasks` and `GET /api/tasks/facets` keep their parameters.
- New comments are written in English; a French comment that is touched is
  rewritten in English (AGENTS.md). User-facing strings are untouched.

## What the code does today

All in `web/src/context/AppContext.tsx`.

- `buildTaskQuery` (l. 1197) builds the query from the scope and every filter.
- `fetchTasks` (l. 1245) awaits `readJson` then calls `setTasks`,
  `trackRead`, `setError` and, in `finally`, `setIsLoading(false)`, with no
  guard: every answer is applied in arrival order.
- Callers of `fetchTasks`: the "reload on filter / project change" effect
  (l. 1764), the SSE `task_updated` listener (l. 1776), bookmark toggle
  (l. 1375), view edit (l. 1733) and about twenty mutation paths.
- The activity poll (l. 1809) runs its own `fetch('/tasks?' + buildTaskQuery())`
  and `setTasks(freshTasks)` when a run finished (l. 1879-1891), outside
  `fetchTasks`.
- On mount the filters start `null`; the restore effect (l. 1277) sets the
  remembered ones from `localStorage` for `filterScope`. The first fetch
  therefore goes out unfiltered and the second filtered: the race of FR1.
- `fetchTaskFacets` (l. 1307) fetches `/tasks/facets` for the project or view
  and calls `setTaskFacets` unguarded; it reruns when the scope or
  `tasks.length` changes.
- The pruning effect (l. 1652) clears `sprintFilter`, `teamFilter` or
  `assigneeFilter` when non-empty facets do not include the value, through
  the persisting setters. On a scope switch, the render after the restore
  holds the new scope's filters and the old scope's facets: the restored value
  is cleared, and `persistFilter` stores the null under the new scope.

## Design

### 1. Latest-request gate (FR1, FR2, FR3)

New module `web/src/lib/latestRequest.ts`:

```ts
/** A counter that tells whether an answer belongs to the newest request. */
export interface LatestRequest {
  /** Starts a request and returns its ticket. */
  begin(): number
  /** True while no request was started after the one holding `ticket`. */
  isLatest(ticket: number): boolean
}
export const createLatestRequest = (): LatestRequest
```

Pure, no React, unit-tested. In `AppContext.tsx`, two instances held in refs:
`tasksRequestRef` and `facetsRequestRef` (`useRef(createLatestRequest())`).

- `fetchTasks`: `const ticket = tasksRequestRef.current.begin()` before the
  request; after `await`, return early unless `isLatest(ticket)`, before any
  `leaveUnavailableView`, toast, `trackRead`, `setError` or `setTasks`; in
  `finally`, `setIsLoading(false)` only when `isLatest(ticket)`.
- Activity poll refresh: take a ticket from the same `tasksRequestRef` before
  its `fetch`, and apply `setTasks` / `setSelectedTask` only when it is still
  the latest. Sharing the counter is what makes a poll answer lose against a
  later filter change, and a filter change answer win against an earlier poll.
- `fetchTaskFacets`: same with `facetsRequestRef` around `setTaskFacets`.
- The functional updates `setTasks(prev => ...)` of the mutation paths are
  untouched (FR7). They apply to whatever list is current, which is the point.

Rejected: an `AbortController` per request. It also works for `fetch` but
`readJson` would need a signal parameter, every caller of `fetchTasks` would
share one controller, and an abort surfaces as an error each path must
recognise. A ticket check is a single comparison at the point of writing.

Rejected: debouncing the reload effect. It narrows the window without closing
it (the SSE and poll paths stay unordered) and delays every filter change.

### 2. Facets that know their scope (FR4, FR5, FR6)

- `taskFacets` gains `scope: string | null`: the `filterScopeKey` of the
  project or view the facets were fetched for, `null` in the initial state.
  `fetchTaskFacets` computes it from the same `selectedProjectId` /
  `selectedViewId` it sends. The `AppContextType.taskFacets` type gains the
  field; consumers that read other fields are unaffected.
- New pure helper in `web/src/lib/filterPruning.ts`:

  ```ts
  export interface PrunableFilters { sprint: string | null; team: string | null; assignee: string | null }
  export interface ScopedFacets { scope: string | null; sprints: string[]; teams: string[]; assignees: string[] }
  /** The filters to drop because their value left the board of their own scope. */
  export const staleFilters = (
    filters: PrunableFilters,
    facets: ScopedFacets,
    scope: string,
    unassignedValue: string,
  ): Array<keyof PrunableFilters>
  ```

  It returns `[]` when `facets.scope !== scope`. Otherwise it keeps today's
  rules: a value is stale when the matching facet list is non-empty and does
  not include it; the assignee `unassignedValue` is never stale.
- The pruning effect calls `staleFilters(..., filterScope, UNASSIGNED_FILTER_VALUE)`
  and clears only the returned filters through the existing persisting
  setters, which now write under the right scope because facets and scope
  agree. `filterScope` joins the effect's dependencies.

Rejected: clearing `taskFacets` on scope switch. It would make the selectors
disappear (`TaskFilters` hides a selector with no values) and flicker on every
switch, and a late answer from the previous scope could still land.

## Target files

| File | Change |
| --- | --- |
| `web/src/lib/latestRequest.ts` | new, gate |
| `web/src/lib/filterPruning.ts` | new, `staleFilters` |
| `web/src/context/AppContext.tsx` | gate in `fetchTasks`, poll refresh, `fetchTaskFacets`; `taskFacets.scope`; pruning effect |
| `web/tests/latestRequest.test.mjs` | new unit tests |
| `web/tests/filterPruning.test.mjs` | new unit tests |
| `web/tests/board-filters.browser.mjs` | new browser regression (out-of-order answers, scope switch) |
| `CHANGELOG.md` | `Fixed` line under `[Unreleased]` |

## Data contracts

No change. `GET /api/tasks?...` and `GET /api/tasks/facets?projectId|viewId`
are called with the same parameters; `scope` on `taskFacets` is client state
only.

## Test strategy

- Unit: the gate (a later `begin` makes the earlier ticket stale, the latest
  stays latest, tickets are independent between instances); `staleFilters`
  (foreign scope returns nothing, `null` scope returns nothing, same scope
  keeps today's rules, empty facet lists drop nothing, unassigned kept).
- Browser (`board-filters.browser.mjs`, harness in the style of
  `board-views.browser.mjs` / `batch-pickup-context.browser.mjs` with
  `page.route`): the fake API delays the answer of an unfiltered
  `/api/tasks` and serves the filtered one at once, then asserts the cards;
  it seeds per-project filters in `localStorage`, switches projects with
  facets that do not contain the other project's values, and asserts the
  selectors and the stored filters, also after a reload.
- Regression: `npm test` and `npx tsc --noEmit` in `web/`; the existing
  `board-views.browser.mjs` still passes.
