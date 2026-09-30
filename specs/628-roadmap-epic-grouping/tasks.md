# Tasks #628 - Roadmap: group and reorder epics by priority or quarter

Ordered; each group leaves the tree building and the tests green.

## 1. Section builder (FR3, FR4)

- [ ] T1.1 `web/src/lib/epicGrouping.ts`: axis type, `isGroupableTab`,
  `quarterOf`, `comingQuarters`, `axisValueOf`, `groupEpics`, `planAxisDrop`.
- [ ] T1.2 `web/tests/epicGrouping.test.mjs`.

## 2. Selection helper (FR8)

- [ ] T2.1 `rangeSelection` in `web/src/lib/boardSelection.ts`.
- [ ] T2.2 Unit tests.

## 3. Preferences (FR6)

- [ ] T3.1 Axis and folded sections in `web/src/lib/roadmapViewPrefs.ts`.
- [ ] T3.2 Tests in `web/tests/roadmapViewPrefs.test.mjs`.

## 4. View (FR1, FR2, FR5, FR7, FR8, FR9)

- [ ] T4.1 Strings fr and en.
- [ ] T4.2 Grouping control, sections, folding in `RoadmapView.tsx`.
- [ ] T4.3 Selection: Ctrl/Cmd, Shift, counter, clear, Escape, pruning.
- [ ] T4.4 Drag sources and drop targets, `dropOnSection`, report.

## 5. Checks and docs (FR10)

- [ ] T5.1 Browser test `web/tests/roadmap-grouping.browser.mjs`.
- [ ] T5.2 `CHANGELOG.md` `Added` line.
- [ ] T5.3 `tsc -b`, `oxlint`, `npm test`, roadmap browser tests.

## Test plan

- `node --test web/tests/*.test.mjs`, the web typecheck and lint.
- Browser tests `roadmap-grouping`, `roadmap-view`, `roadmap-epic-axes` with
  the main checkout's Playwright module.
- Manual on a copy of the dev database, without a tracker token: group NOW by
  quarter, drag three epics onto the next quarter, reload, fold a section and
  come back from another view.
