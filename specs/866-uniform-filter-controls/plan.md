# Plan #866 - Uniform filter controls across all views

## Stack

React web client only. Sectile Desktop renders the same components and needs
no change of its own. No server change, no API change, no migration: every
view keeps reading and writing the state it uses today (`AppContext` for the
global filters, `useState` for the local ones).

Files touched:

- New: `web/src/components/filters/` (the shared controls),
  `web/src/lib/activeFilters.ts` (the chip model).
- Changed: `web/src/hooks/useClickOutside.ts`, `TaskFilters.tsx`,
  `RoadmapView.tsx`, `EpicLabelFilter.tsx`, `EpicOriginFilter.tsx`,
  `TriageView.tsx`, `SprintTimelineView.tsx`, `TeamView.tsx`,
  `ActivitiesView.tsx`, `BoardView.tsx`, `ListView.tsx` (filter bar only),
  `web/src/locales/shell.ts`, `web/src/locales/planning.ts`,
  `web/src/locales/operations.ts`, `CHANGELOG.md`.
- Not touched: the non-filter selects (`ListView.tsx:615`,
  `SprintTimelineView.tsx:1211`, `TrackerBacklogView.tsx:137`), the roadmap's
  sort selector, `AppContext.tsx` state and its request building.

## Shared controls (`web/src/components/filters/`)

| Control | Built from | Notes |
|---------|------------|-------|
| `FilterBar` | the `TaskFilters` toolbar layout | slots: `inline`, `panel`; renders the "Filters" button with its count badge only when `panel` has content; renders `ActiveFilterChips` under itself |
| `FilterPanel` | the `TaskFilters` panel (`w-[300px]`, `max-h-[70vh]`, `rounded-xl`) | header with a "Reset" link for the panel's pickers; closes through `useClickOutside` with Escape |
| `SinglePicker` | `LookupField` | thin wrapper fixing the label, icon, "none" option and clear entry; `LookupField` already portals its list and marks it with `PORTALED_PANEL_ATTRIBUTE`, so a pick does not close the panel |
| `MultiPicker` | the checklist of `EpicLabelFilter` / `EpicOriginFilter` | search box, one checked row per value with its count, list stays open between picks |
| `FilterToggle` | the pinned / active buttons of `TaskFilters` | pressed look from those buttons |
| `SegmentedFilter` | the Activities status buttons, restyled as toggles | single choice among a few values |
| `FilterSearch` | the Triage search field (icon, clear button) | one size and colour set |
| `ActiveFilterChips` | the roadmap chip row (`RoadmapView.tsx:1880`) | one chip per `ActiveFilter`, plus "Clear all" |

One visual set for all of them: the `TaskFilters` sizes (`text-[11px]`,
`rounded-md`, `px-2 py-1`), `bg-[var(--bg-secondary)]` and
`border-[var(--border-color)]`, `--accent-color` for pressed and focus.

## Chip model (`web/src/lib/activeFilters.ts`)

Pure, so it is tested with `node --test`:

```ts
export interface ActiveFilter {
  id: string          // stable key: dimension, plus value for multi pickers
  label: string       // already localised
  clear: () => void
}

/** Clears every listed filter once, in order. */
export function clearAll(filters: ActiveFilter[]): void

/** One entry per chosen value of a multi picker. */
export function multiValueFilters(
  dimension: string, values: string[], label: (v: string) => string,
  remove: (v: string) => void,
): ActiveFilter[]

/** Number of set pickers, for the "Filters" button badge. */
export function countSetPickers(values: Array<string | string[] | null | undefined>): number
```

Each view builds its own `ActiveFilter[]` from the state it already has, the
way `RoadmapView` builds `activeFilterChips` today, and passes it to
`FilterBar`. Keys use `id`, not `label`: two chips with the same label (a
sprint and a label of the same name) must not collide, which the roadmap's
`key={chip.label}` allows today.

## `useClickOutside`

Gains an optional `{ onEscape?: () => void }` argument registering a
`keydown` listener for Escape under the same `enabled` flag. `TaskFilters`
drops its own Escape effect; `EpicLabelFilter` and `EpicOriginFilter` go away
with their own listeners. `TaskCard` keeps its listener, as the hook's comment
already says.

## Per view

| View | Inline | Panel | Chips from |
|------|--------|-------|------------|
| Board, List | pinned, active toggles; priority dots (unchanged) | sprint, team, assignee, parent (single); tracker status, issue type (multi) | `AppContext` global filters |
| Roadmap | none of its own: search and pinned are set from the header (global) and only show as chips, as today | epic priority (single), epic labels, epic origins (multi) | today's `filterChips`, keyed by `id` |
| Triage | search | sprint, parent (single) | local sprint and search, global parent |
| Sprint timeline | backlog search | parent (single) | global parent, local search |
| Team | team picker (`SinglePicker`, not cleared, not counted) | none | none |
| Activities | status (`SegmentedFilter`), search | skill (single) | local status, skill, search |

- `TaskFilters` becomes the Board / List composition of the shared controls;
  its public shape (`<TaskFilters />`, no props) is kept so `BoardView` and
  `ListView` change only by the chip row it now renders.
- The Roadmap's epic priority keeps its `PriorityFilter` values (`null`,
  priority, `'none'`); its sort selector stays a native `<select>`, out of
  scope.
- Triage's "all" / "none" sentinel values map to the picker's clear entry and
  "none" option; its parent picker keeps writing `__no_macro__` to the global
  `parentFilter`.
- The timeline parent drop-down (`SprintTimelineView.tsx:160-265`) is replaced
  by `SinglePicker` with the same options and "No macro" entry.

## Strings

- `t.shell.filters` gains `clearAll`, `activeFilterTitle` (moved from the
  roadmap strings), `filtersButton`, `noResult`, `searchPlaceholder`.
- The "none" entries are aligned on one wording per dimension in both
  languages (`noSprint`, `noMacro`, `unassigned`, `noPriority`).
- Strings left unused by the removed components are deleted from
  `planning.ts` and `operations.ts`; the locale catalog test keeps both
  languages in step.

## Tests

- `web/tests/activeFilters.test.mjs`: `clearAll` calls each `clear` once;
  `multiValueFilters` gives one entry per value with distinct ids;
  `countSetPickers` ignores `null`, `''` and `[]`.
- `web/tests/filter-panel.browser.mjs` (existing): kept passing, extended with
  the chip row, a chip clear, "Clear all", Escape and a press outside.
- New `web/tests/view-filters.browser.mjs`: for Roadmap, Triage, Timeline,
  Team and Activities, with mocked context: the filter bar renders, no native
  `<select>` is a filter, setting a picker shows its chip and narrows the
  list as before, clearing restores it.
- Existing browser tests referencing the replaced controls (listed by
  `grep -l` on `filterSprint`, `skillFilter`, `EpicLabelFilter`,
  `EpicOriginFilter`) are updated to the new selectors, never weakened.
- `make test` (Go, `npm test`, `tsc --noEmit`, `oxlint`).

## Rejected alternatives

- **Every picker inline, roadmap style.** Rejected by round 2, decision 2.
- **A new state store for filters.** Out of scope by decision 1; it would also
  change what the server receives.
- **Restyling the native `<select>` only.** Leaves single vs multi, search,
  chips and reset divergent, which is the ticket's point.

## As implemented

Departures from the plan above, found while implementing; none changes an
acceptance criterion except where `spec.md` was amended with it.

- **Flat files.** The controls live in `web/src/components/` (`FilterToggle`,
  `FilterPanel`, `FilterPicker` with `FilterRow` and `SinglePicker`,
  `MultiPicker`, `FilterSearch`, `ActiveFilterChips`, `useMacroFilterSearch`),
  not in a `filters/` subfolder: every browser fixture mocks the context by
  rewriting the exact import `'../context/AppContext'`, which a subfolder
  would escape.
- **No `FilterBar`, no `SegmentedFilter`.** Each view lays out its
  `FilterPanel` and `ActiveFilterChips` in its existing toolbar row, which
  wraps. The Activities status buttons and tiles are kept (spec US2.5).
- **Escape** uses the existing `useEscapeKey`; `useClickOutside` is unchanged.
  Escape closes the whole panel, a lookup list open inside it included.
- **Chips** only for what the toolbar does not show pressed (spec US3.1);
  "Clear all" from two chips (spec US3.3).
- **Macro lookup** is one hook, `useMacroFilterSearch`, for the board, the
  triage and the timeline; the triage and the timeline always offer "No
  macro", as their drop-downs did.
- **Triage** takes the board's wording for its pickers ("Tous sprints",
  "Toutes macros…"); its own strings are removed. The timeline's macro picker
  loses the epic colour dot of the old drop-down.
- **Tests.** `board-filters.browser.mjs` was failing on `origin/main` because it
  predated the Filters panel; it now opens the panel first. Four roadmap
  browser tests open the panel instead of the removed buttons and select.

