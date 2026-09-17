# Tasks

## 1. The predicate
- [ ] 1.1 Add `activeTaskIds(activities, now?)` to `web/src/lib/remoteRunIndicator.ts`: one pass over
      the activities, keeping the `remote_run` ones whose state is waiting, running or queued, and
      returning their task ids as a `Set`. Reuse the module's existing constants and `isWaiting`, and
      exclude the recently-canceled window `deriveRunIndicator` honours (D1, D3, D4).
- [ ] 1.2 Extend `web/tests/remoteRunIndicator.test.mjs`: a running run, a queued run, a waiting run,
      a just-canceled run, a finished run, a task with no run, an activity that is not a remote run,
      and two runs on the same task yielding one entry.

## 2. The filter in the context
- [ ] 2.1 Add `activeOnly: boolean` and `setActiveOnly` to `AppContextValue` and the provider, beside
      `pinnedOnly` (`web/src/context/AppContext.tsx:146`, `:500`, `:3120`).
- [ ] 2.2 Persist it through `persistFilter({ activeOnly: value ? '1' : null })` and restore it with
      `setActiveOnlyState(stored.activeOnly === '1')` in the per-project restore effect (`:636`,
      `:1000`).
- [ ] 2.3 Expose `activeTaskIds` as a memo over `activities`, so each view tests membership rather
      than rescanning (D3). Do **not** add a parameter to `buildTaskQuery`, and do not filter the
      context's `tasks` (D2, D5).

## 3. The toggle
- [ ] 3.1 Add the toggle to `web/src/components/TaskFilters.tsx`, next to the pin (`:160`): same
      button shape, a `Loader2` or `Activity` glyph, the count of active tasks, and
      `disabled={!activeOnly && activeCount === 0}` (D7). The component is already shared by the
      board and the list, so one edit serves both.
- [ ] 3.2 Add the clearable chip to `web/src/components/Header.tsx` beside the `pinnedOnly` one
      (`:122`), and include `activeOnly` in `hasActiveFilters` (`:42`).

## 4. The two views
- [ ] 4.1 In `web/src/components/BoardView.tsx`, narrow the collections that feed the columns
      (`:333`, `:343`, `:357`, `:506`) by the active set when the filter is on. Leave the `tasks.find`
      lookups at `:251`, `:376` and `:392` untouched (D5), and keep every column rendered (D6).
- [ ] 4.2 In `web/src/components/ListView.tsx`, narrow `sortedTasks` (`:98`) so the groups (`:795`,
      `:866`) and the counts follow. Leave the selection lookup at `:178` untouched (D5).
- [ ] 4.3 Give each view an empty state that says no task is running, distinct from "no task matches
      the filters".

## 5. Gates
- [ ] 5.1 `make test` (go, web tests, tsc, oxlint).
- [ ] 5.2 Check the filter live: start a run, see the task appear; let it end, see it leave.
- [ ] 5.3 Re-read the diff as a reviewer.
