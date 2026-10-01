# Plan #628 - Roadmap: group and reorder epics by priority or quarter

Web only: no migration, no API, no server change. Everything is pure logic in
two library modules, tested without a DOM, and the wiring in
`web/src/components/RoadmapView.tsx`.

## 1. Section builder: `web/src/lib/epicGrouping.ts` (new)

```ts
export type EpicGroupAxis = 'none' | 'priority' | 'quarter'
export const EPIC_GROUP_AXES: EpicGroupAxis[]

export interface EpicSection<R> {
  id: string        // folding key: 'priority:p1', 'quarter:2026-Q4', 'priority:none'
  axis: 'priority' | 'quarter'
  value: string     // 'p1', '2026-Q4', '' for the no-value section
  rows: R[]
}

export const isGroupableTab = (tab: HorizonTab): boolean   // not 'hidden'
export const quarterOf = (date: Date): string              // local date -> 'YYYY-Qn'
export const comingQuarters = (date: Date, count = 4): string[]
export const groupEpics = <R extends { priority?: EpicPriority | ''; quarter?: string }>(
  rows: R[], axis: 'priority' | 'quarter', today: Date
): EpicSection<R>[]
export const axisValueOf = (row, axis): string             // '' when none
export const planAxisDrop = (rows, keys, axis, target) => ({ toSave: string[], skipped: string[] })
```

- Rows keep their incoming order inside a section, so the #627 sort applied
  before grouping is the order inside each section.
- Quarters go through `normalizeQuarter`; an unreadable one counts as none.
- `YYYY-Qn` sorts chronologically as a string.

## 2. Selection: `web/src/lib/boardSelection.ts` (extended)

Reuse `isSelectionClick`, `toggleSelected`, `pruneSelection` and
`shouldEscapeClearSelection`. Add one pure helper shared with #631 later:

```ts
/** The ids from the anchor to the target in display order, both included. */
export const rangeSelection = (order: readonly string[], anchor: string | null, target: string): string[]
```

With no anchor, or an anchor no longer in the order, the range is the target
alone.

## 3. Preferences: `web/src/lib/roadmapViewPrefs.ts` (extended)

- `ROADMAP_GROUP_AXIS_STORAGE_KEY = 'sectile_roadmap_group_axis'`,
  `loadRoadmapGroupAxis()` (unknown -> `'none'`), `saveRoadmapGroupAxis()`.
- `ROADMAP_FOLDED_SECTIONS_STORAGE_KEY = 'sectile_roadmap_folded_sections'`, a
  JSON array of section ids; `loadRoadmapFoldedSections()` answers `[]` on
  anything but an array of strings; `saveRoadmapFoldedSections()`.

## 4. View: `RoadmapView.tsx`

- State: `groupAxis` (persisted), `foldedSections: Set<string>` (persisted),
  `pickedKeys: Set<string>`, `pickAnchor: string | null`, `dropTarget: string | null`
  (section id under the pointer).
- `grouped = groupAxis !== 'none' && isGroupableTab(tab)`.
- `sections = grouped ? groupEpics(visibleRows, groupAxis, new Date()) : null`.
- `displayOrder`: the keys of the unfolded sections in order; the selection is
  pruned against it (`pruneSelection`) in an effect.
- Row click: when `grouped` and `isSelectionClick(e)`: toggle and set the
  anchor; when `grouped` and `e.shiftKey`: add `rangeSelection`; else
  `setSelectedKey` as today. Selected rows get an accent outline and
  `aria-selected`.
- Rows are `draggable={grouped}`; `onDragStart` writes the keys as JSON under
  `application/x-sectile-epic-keys` (the selection when the row is picked, the
  row alone otherwise).
- Section: a header button (chevron, value label, count) toggling the fold, and
  a body; header and body take `onDragOver` / `onDrop`. The drop target is
  highlighted.
- `dropOnSection(section, keys)`: `planAxisDrop`; when nothing is to save,
  return without a toast (a drop on the epic's own section is silent); else
  save each with `saveMacroMeta(projectId, key, patch, { quiet:
  true })`, reload with `fetchProjectMacros` once, remove saved keys from the
  selection, one toast (`success` when nothing failed, `error` naming keys
  otherwise).
- Toolbar: a select after the priority sort; a selection strip (count and
  clear button) while `pickedKeys.size > 0`.
- Escape: a `window` keydown listener while the selection is non-empty, gated
  by `shouldEscapeClearSelection` as `BoardView` does.

## 5. Strings (`web/src/locales/planning.ts`, fr and en)

Under `roadmap.grouping`: `label`, `none`, `priority`, `quarter`,
`noPriority`, `noQuarter`, `fold`, `unfold`, `dropHere`, `selected` (plural),
`clearSelection`, `dragTitle`, `doneTitle`, `done` (plural), `skipped`
(plural), `failedTitle`, `failed`.

## 6. Tests

- `web/tests/epicGrouping.test.mjs`: priority order and empty sections; quarter
  union, order, current and coming quarters from a fixed date, past quarters
  only when used, unreadable quarter to none; order kept inside sections;
  `planAxisDrop` skipping already-at-target and clearing on none;
  `quarterOf` / `comingQuarters` across a year boundary.
- `web/tests/boardSelection.test.mjs` (or the existing one): `rangeSelection`
  both directions, missing anchor.
- `web/tests/roadmapViewPrefs.test.mjs`: axis and folded sections round trip,
  foreign values.
- Browser `web/tests/roadmap-grouping.browser.mjs`: group by priority, fold and
  reload, drag one epic to P1 (request body carries `priority: "p1"` only),
  Ctrl-click two epics and drop on "No priority", the report toast.

## 7. Docs

- `CHANGELOG.md`: one `Added` line.
- `specs/633-epic-readiness/plan.md` Part B already points here; nothing to
  change until #633 Part B.
