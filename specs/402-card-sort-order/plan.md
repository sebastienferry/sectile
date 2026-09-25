# #402: Implementation plan

References: [`spec.md`](./spec.md), [`tasks.md`](./tasks.md).

## Stack and scope

Web client only (`web/`, React 19 + TypeScript). No Go change, no migration, no
endpoint, no tracker field: `priority`, `key`, `parentKey`, `trackerUpdatedAt`,
`updatedAt` and `id` are already on every `Task`. The desktop app embeds the web
UI and inherits the change without code of its own.

## Target files

| File | Change |
| --- | --- |
| `web/src/lib/boardSort.ts` (new) | Pure module: sort type, options, comparators, `sortTasks`, `pickBoardSortField`, load / save of the preference. |
| `web/src/context/AppContext.tsx` | `boardSort` state and `setBoardSort`, loaded and saved through `boardSort.ts`; exposed on the context. |
| `web/src/components/BoardSortSelect.tsx` (new) | The shared selector: criterion `<select>` plus direction button, `sm` / `md` sizes. |
| `web/src/components/BoardView.tsx` | Replace `PRIORITY_RANK` / `byPriorityDesc` with `sortTasks(list, boardSort)` in its three places; selector in the toolbar. |
| `web/src/components/ListView.tsx` | Replace the "Priorité" button with the selector; split the local sort into a nullable header override; grouped view and flat table read their own ordered lists. |
| `web/src/locales/translations.ts` | `board.sort` strings, French and English, and the interface entry. |
| `web/tests/boardSort.test.mjs` (new) | Unit tests for the module. |
| `web/tests/board-sort.browser.mjs` (new) | Browser regression on the real App. |
| `CHANGELOG.md` | One `Added` line. |

## Decisions

### One pure module

`web/src/lib/boardSort.ts`, shaped like `boardDisplayMode.ts` (injectable
`StorageLike`, every storage access inside `try`) so that `node --test` imports
it directly, as `boardDisplayMode.test.mjs` does.

```ts
export type BoardSortField = 'priority' | 'epic' | 'key' | 'updated'
export interface BoardSort { field: BoardSortField; asc: boolean }

export const BOARD_SORT_STORAGE_KEY = 'sectile_board_sort'
export const DEFAULT_BOARD_SORT: BoardSort = { field: 'priority', asc: false }

/** Order of the selector; `naturalAsc` is the direction a pick starts in. */
export const BOARD_SORT_OPTIONS: readonly {
  id: BoardSortField
  labelKey: BoardSortField          // translations.board.sort.fields[...]
  naturalAsc: boolean
}[]
// priority: false, epic: false, key: true, updated: false

export function pickBoardSortField(field: BoardSortField): BoardSort
export function sortTasks<T extends SortableTask>(tasks: readonly T[], sort: BoardSort): T[]
export function loadBoardSort(storage?: StorageLike): BoardSort
export function saveBoardSort(sort: BoardSort, storage?: StorageLike): void
```

`SortableTask` is `Pick<Task, 'id' | 'key' | 'priority' | 'parentKey' |
'trackerUpdatedAt' | 'updatedAt'>`, so the tests build plain objects.

`sortTasks` never mutates its input and returns a new array.

**Comparators.** `rank(p)` is `{ urgent: 4, high: 3, medium: 2, low: 1 }[p] ?? 0`,
the table `BoardView` and `ListView` each hold today. `byKey` is
`a.key.localeCompare(b.key, undefined, { numeric: true })`, the Backlog's and the
roadmap's comparator. The shared tail, never reversed, is:

```
tail(a, b) = rank(b) - rank(a) || byKey(a, b) || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
```

The `id` step is the "order that does not depend on how the list arrived" of
FR4: a multi-project view can hold `#42` twice (`board-views.browser.mjs` seeds
exactly that), and `Array.prototype.sort` being stable would otherwise keep the
arrival order, which a refresh can change (US6.2).

Per field, with `dir = asc ? 1 : -1`:

- `priority`: `dir * (rank(a) - rank(b)) || tail`.
- `key`: `dir * byKey(a, b) || tail`.
- `updated`: `time(t) = Date.parse(t.trackerUpdatedAt || t.updatedAt)`; a task
  whose `time` is `NaN` (no date, or an unparsable one) sorts after every dated
  task in both directions; otherwise `dir * (time(a) - time(b)) || tail`.
- `epic`: done in three passes rather than as a pairwise comparator, because a
  card's place depends on its whole group:
  1. partition by `parentKey` (empty string and `undefined` both mean "no
     parent");
  2. for each group, `top = max(rank)`; order the groups by
     `dir * (top(a) - top(b))`, then by `byKey` on the parent key, never
     reversed;
  3. concatenate each group sorted by `tail`, then the no-parent tickets sorted
     by `tail`.

Rejected: a composite comparator `parentKey || priority`. It orders groups by
parent key, not by their highest priority, so an urgent epic could sit under a
low one (US2.2).

Rejected: reversing the whole Epic output on ascending. It would put the lowest
card of each group first and the no-parent tickets first (US2.5, D12).

**Preference.** `sectile_board_sort` holds JSON, `{"field":"epic","asc":false}`.
`loadBoardSort` returns `DEFAULT_BOARD_SORT` when storage is missing, the value is
absent, not JSON, not an object, `field` is not one of the four, or `asc` is not a
boolean. No legacy `taskacao_*` key: the preference is new.

### Context

`AppContext.tsx` keeps `boardSort` beside `boardGrouping` and
`boardCardDisplayMode` (around line 520): `useState(loadBoardSort)` and a
`setBoardSort` that sets the state and calls `saveBoardSort`. Both are added to
the context interface (line ~122) and to the provider value (line ~3817). One
state in the context is what makes D5 hold within a session; the stored value
makes it hold across reloads.

The selector writes the whole `BoardSort`: picking a criterion calls
`setBoardSort(pickBoardSortField(field))`, even when it is the current one, which
resets it to its natural direction (FR5). The direction button calls
`setBoardSort({ ...boardSort, asc: !boardSort.asc })`.

### Selector component

`BoardSortSelect.tsx` next to `BoardGroupingToggle.tsx`, with the same
`size?: 'sm' | 'md'` prop (Backlog `sm`, Board `md`) and the same border and
radius tokens, so the two controls read as a pair.

- A native `<select>` for the criterion, `aria-label` and `title` from
  `t.board.sort.label` ("Trier par" / "Sort by"), options from
  `BOARD_SORT_OPTIONS`. A native select keeps keyboard and screen-reader
  behaviour for free and needs no outside-click handling.
- A button for the direction: `ArrowUp` / `ArrowDown` from `lucide-react`, and
  `aria-label` / `title` from `t.board.sort.ascending` / `descending`
  ("Croissant" / "Décroissant", "Ascending" / "Descending") naming the current
  direction (US3.4). `ArrowUpDown` stays the flat-table header glyph.
- `data-testid="board-sort-field"` and `"board-sort-direction"` for the browser
  test.

Labels (French / English): Priorité / Priority, Epic / Epic, Clé / Key,
Dernière mise à jour / Last updated.

### Board

In `BoardView.tsx`, delete `PRIORITY_RANK` and `byPriorityDesc` (lines 98-100)
and call `sortTasks(list, boardSort)` in `tasksForColumn` (line 372),
`unassignedTasks` (line 386) and `workflowColumnTasks` (line 401). The selector
goes in the left toolbar group, after `<BoardGroupingToggle size="md" />`
(line 557).

`boardCards`, `boardOrder` and `orderSelection` (lines 413-425) already derive
from `workflowColumnTasks` / `statusColumnTasks` / `unassignedTasks`, so card
selection and batch order follow the new sort with no change (FR7, US7.1).

`PRIORITY_RANK` is used by nothing else in the file; check with a grep before
deleting it.

### Backlog

`ListView.tsx` today holds `sortField` / `sortAsc` (lines 79-80) and one
`sortedTasks` (line 122) that both views read.

- Replace them with `headerSort: { field: HeaderSortField; asc: boolean } | null`,
  `HeaderSortField` being today's union (`key | title | status | priority |
  dueDate | createdAt`; `createdAt` has no header and can go, D13).
- Split the list in two:
  - the grouped view (lines 874 and 945) and `batchRows` in grouped mode
    (lines 206-208) sort each group on its own, `sortTasks(group, boardSort)`,
    as a Board column is. *Changed while implementing:* sorting the whole list
    once and filtering it per group, as first planned, keeps an epic
    contiguous but orders the epics of a group by priorities found in other
    groups, which breaks US2.2 inside a Backlog group;
  - `tableTasks = headerSort ? headerSorted(listed, headerSort) :
    selectorSortedTasks`: `visibleTasks` for the flat table reads it.
  `headerSorted` is today's comparator body, unchanged, with the same
  `tail` appended so header ties are deterministic too.
- `handleSort(field)`: the column the table is currently sorted by is
  `headerSort?.field ?? (boardSort.field === 'priority' || boardSort.field ===
  'key' ? boardSort.field : null)`. Same column: flip (starting from
  `headerSort?.asc ?? boardSort.asc`). Other column: `{ field, asc: field !==
  'priority' }`, as today (US5.3).
- Clear the override when the selector changes. *Changed while
  implementing:* instead of a `useEffect` resetting it (which the linter flags
  as `set-state-in-effect`), the override records the `boardSort` value it was
  made against and only applies while `boardSort` is still that value. The
  selector is the only writer of `boardSort` and writes a new object on every
  change, so any change retires the override, and `BoardSortSelect` stays free
  of a Backlog-specific callback.
- The override is component state: it is lost on reload and when the Backlog
  unmounts (US5.6), and the grouped view never reads it (US5.7). It survives a
  switch from the flat table to the grouped view and back within the page.
- Remove the "Priorité" toolbar button (lines 830-843) and put
  `<BoardSortSelect size="sm" />` after `<TaskFilters />`, where it was.

### Translations

Add under `board` in the interface (line ~108) and in both dictionaries (lines
~903 and ~1694):

```ts
sort: {
  label: string
  ascending: string
  descending: string
  fields: { priority: string; epic: string; key: string; updated: string }
}
```

The Backlog's existing French literals are left as they are; only the strings
this ticket adds go through `t`.

### Changelog

Under `[Unreleased]` / `### Added`, one line naming the selector on the Board
and the Backlog, the four criteria, the Epic grouping, the direction toggle and
the remembered choice, with `(#402)`.

## Risks

- **`ListView` size.** 1 374 lines; the sort split touches the grouped view, the
  table and `batchRows`. Keep the change to the list derivation and the toolbar
  and do not restructure the rendering.
- **Existing browser tests.** `backlog-batch.browser.mjs` asserts a batch order
  from the Backlog; with no stored sort it stays Priority, highest first, so it
  should pass unchanged, and the tie-break on key may reorder equal-priority
  fixtures. Run it and adjust only fixtures whose expected order relied on
  arrival order.
- **Worktree tooling.** `node --test` runs bare; `tsc` / `oxlint` and Playwright
  need the main checkout's `node_modules` (project memory: worktrees have no
  `node_modules`, browser tests need a `#`-free `PLAYWRIGHT_MODULE`).
