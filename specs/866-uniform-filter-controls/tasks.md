# Tasks #866 - Uniform filter controls across all views

Order matters: each step leaves the tree buildable and `make test` green.
Tests go with the step they cover. One commit per step.

## 1. Foundations

- [ ] T1.1 `useClickOutside` gains the optional Escape handler (US1.5).
- [ ] T1.2 `web/src/lib/activeFilters.ts`: `ActiveFilter`, `clearAll`,
      `multiValueFilters`, `countSetPickers` (US1.4, US3).
- [ ] T1.3 Shared strings in `web/src/locales/shell.ts`, French and English
      (FR4).
- Tests: `web/tests/activeFilters.test.mjs`; locale catalog test.

## 2. Shared controls

- [ ] T2.1 `filters/FilterToggle`, `filters/SegmentedFilter`,
      `filters/FilterSearch` (US2.4, US2.5).
- [ ] T2.2 `filters/SinglePicker` over `LookupField`, `filters/MultiPicker`
      from the epic filters' checklist (US2.1, US2.2, FR5).
- [ ] T2.3 `filters/FilterPanel`, `filters/ActiveFilterChips`,
      `filters/FilterBar` (US1.1, US1.4 to US1.6, US3.1 to US3.4).

## 3. Board and List

- [ ] T3.1 Rebuild `TaskFilters` on the shared controls, same props, same
      state; add the chip row (US1.2, US3, US4).
- Tests: `filter-panel.browser.mjs` extended (chips, clear, "Clear all",
  Escape, press outside); `progressive-board`, `i18n-shell` still pass.

## 4. Roadmap

- [ ] T4.1 Epic priority, epic labels, epic origins move into the panel as
      `SinglePicker` / `MultiPicker`; delete `EpicLabelFilter.tsx` and
      `EpicOriginFilter.tsx` (FR3).
- [ ] T4.2 `filterChips` becomes `ActiveFilter[]` keyed by `id`; the origin
      chip keeps its reset (US3.5).
- Tests: `view-filters.browser.mjs` roadmap case; existing roadmap browser
  tests updated to the new selectors.

## 5. Triage, Sprint timeline, Team, Activities

- [ ] T5.1 Triage: `FilterSearch` inline, sprint and parent pickers in the
      panel, chips (US2.3).
- [ ] T5.2 Sprint timeline: parent drop-down replaced by `SinglePicker`,
      backlog search by `FilterSearch`, chips.
- [ ] T5.3 Team: team `SinglePicker` inline, not counted, no chip (US1.3).
- [ ] T5.4 Activities: `SegmentedFilter` status, `FilterSearch`, skill picker
      in the panel, chips.
- [ ] T5.5 Delete the strings the removed controls used (FR4).
- Tests: `view-filters.browser.mjs` cases for each view, including "no native
  `<select>` as a filter" (FR2) and "same items before and after" (US4.1);
  existing browser tests updated.

## 6. Finish

- [ ] T6.1 Check the out-of-scope selects are untouched (`git diff` on
      `ListView.tsx:615`, `SprintTimelineView.tsx:1211`,
      `TrackerBacklogView.tsx`).
- [ ] T6.2 `CHANGELOG.md`: one `Changed` line under `## [Unreleased]` (FR6).
- [ ] T6.3 `make test` green; screenshots of the seven filter bars for the
      pull request.
