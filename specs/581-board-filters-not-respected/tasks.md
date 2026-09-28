# Tasks #581 - Board filters are respected and kept

Ordered. Each task names the requirements it covers and its tests.

## Phase 1 - Pure helpers

- [x] **T1** `web/src/lib/latestRequest.ts`: `createLatestRequest()` with
  `begin()` and `isLatest(ticket)`. (FR1, FR2, FR3)
  - Test `web/tests/latestRequest.test.mjs`: a first ticket is latest; a
    second `begin` makes the first stale and the second latest; two instances
    do not share tickets.
- [x] **T2** `web/src/lib/filterPruning.ts`: `staleFilters(filters, facets,
  scope, unassignedValue)`. (FR4, FR5, FR6)
  - Test `web/tests/filterPruning.test.mjs`: facets of another scope or with
    a `null` scope drop nothing; same scope drops a sprint, team or assignee
    absent from a non-empty list; an empty list drops nothing; a present value
    is kept; the unassigned value is never dropped.

## Phase 2 - Wiring in `AppContext.tsx`

- [x] **T3** Gate `fetchTasks` with `tasksRequestRef`: ignore a stale answer
  before any state write, toast or view fallback; clear the loading state only
  for the latest request. (FR1, FR2)
- [x] **T4** Gate the activity poll refresh with the same `tasksRequestRef`
  before `setTasks` / `setSelectedTask`, joining the newest read with
  `current()` rather than starting one. (FR1)
- [x] **T5** Gate `fetchTaskFacets` with `facetsRequestRef`, and record
  `scope: filterScopeKey(selectedProjectId, selectedViewId)` on the facets;
  add `scope` to the `taskFacets` state, its initial value (`null`) and the
  `AppContextType` type. (FR3, FR4)
- [x] **T6** Pruning effect: call `staleFilters` with `filterScope` and clear
  only what it returns; add `filterScope` to the dependencies. Rewrite the
  touched French comments in English. (FR4, FR5, FR6)
- [x] **T7** Check that the mutation paths using `setTasks(prev => ...)` are
  untouched. (FR7)

## Phase 3 - Regression and documentation

- [x] **T8** `web/tests/board-filters.browser.mjs` (US1, US2):
  - opening a project with a remembered sprint while the unfiltered answer is
    delayed shows only that sprint's cards (US1-1);
  - a filter change during a delayed older answer shows the newer filters'
    cards (US1-2, US1-3);
  - switching from A to B with A's answer delayed shows B (US1-4): same gate
    as US1-1 and US1-2, not a browser scenario of its own;
  - switching B to A keeps A's remembered sprint, team and assignee on screen
    and in `localStorage`, also after a reload (US2-1, US2-3);
  - a saved view keeps its remembered team when opened from a project without
    it (US2-2): not a browser scenario of its own, the view scope goes through
    the same `staleFilters` path and is covered by the unit test "a view and a
    project are different scopes";
  - a remembered sprint absent from its own project's facets is dropped and
    forgotten (US2-4); "Unassigned" is kept (US2-5).
- [x] **T9** `CHANGELOG.md`: under `## [Unreleased]` / `### Fixed`, one line:
  the board again respects its Sprint, Team and Assignee filters, and keeps
  them when switching projects or views (#581). (FR8)
- [x] **T10** Run in `web/`: `npm test`, `npx tsc --noEmit`, the new and the
  existing `board-views.browser.mjs` browser tests.
