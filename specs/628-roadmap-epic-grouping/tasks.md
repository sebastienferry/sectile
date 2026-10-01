# Tasks #628 - Roadmap: group and reorder epics by priority or quarter

Ordered; each group leaves the tree building and the tests green.

## 1. Section builder (FR3, FR4)

- [x] T1.1 `web/src/lib/epicGrouping.ts`: axis type, `isGroupableTab`,
  `quarterOf`, `comingQuarters`, `axisValueOf`, `groupEpics`, `planAxisDrop`.
- [x] T1.2 `web/tests/epicGrouping.test.mjs`.

## 2. Selection helper (FR8)

- [x] T2.1 `rangeSelection` in `web/src/lib/boardSelection.ts`.
- [x] T2.2 Unit tests.

## 3. Preferences (FR6)

- [x] T3.1 Axis and folded sections in `web/src/lib/roadmapViewPrefs.ts`.
- [x] T3.2 Tests in `web/tests/roadmapViewPrefs.test.mjs`.

## 4. View (FR1, FR2, FR5, FR7, FR8, FR9)

- [x] T4.1 Strings fr and en.
- [x] T4.2 Grouping control, sections, folding in `RoadmapView.tsx`.
- [x] T4.3 Selection: Ctrl/Cmd, Shift, counter, clear, Escape, pruning.
- [x] T4.4 Drag sources and drop targets, `dropOnSection`, report.

## 5. Checks and docs (FR10)

- [x] T5.1 Browser test `web/tests/roadmap-grouping.browser.mjs`.
- [x] T5.2 `CHANGELOG.md` `Added` line.
- [x] T5.3 `tsc -b`, `oxlint`, `npm test`, roadmap browser tests.

## Test plan

- `node --test web/tests/*.test.mjs`, the web typecheck and lint.
- Browser tests `roadmap-grouping`, `roadmap-view`, `roadmap-epic-axes` with
  the main checkout's Playwright module.
- Manual on a copy of the dev database, without a tracker token: group NOW by
  quarter, drag three epics onto the next quarter, reload, fold a section and
  come back from another view.
