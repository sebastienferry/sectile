# Tasks #866 - Uniform filter controls across all views

Order matters: each step leaves the tree buildable and `make test` green.
Tests go with the step they cover. One commit per step.

## 1. Foundations

- [x] T1.1 Escape through the existing `useEscapeKey` (US1.5); see plan, As implemented.
- [x] T1.2 `web/src/lib/activeFilters.ts`: `ActiveFilter`, `clearAll`,
      `multiValueFilters`, `countSetPickers` (US1.4, US3).
- [x] T1.3 Shared strings in `web/src/locales/shell.ts`, French and English
      (FR4).
- Tests: `web/tests/activeFilters.test.mjs`; locale catalog test.

## 2. Shared controls

- [x] T2.1 `FilterToggle`, `FilterSearch` (US2.4, US2.5); no `SegmentedFilter`.
- [x] T2.2 `SinglePicker` over `LookupField`, `MultiPicker`
      from the epic filters' checklist (US2.1, US2.2, FR5).
- [x] T2.3 `FilterPanel`, `ActiveFilterChips` (US1.1, US1.4 to US1.6, US3.1 to
      US3.4); no `FilterBar`.

## 3. Board and List

- [x] T3.1 Rebuild `TaskFilters` on the shared controls, same props, same
      state; add the chip row (US1.2, US3, US4).
- Tests: `filter-panel.browser.mjs` extended (chips, clear, "Clear all",
  Escape, press outside); `progressive-board`, `i18n-shell` still pass.

## 4. Roadmap

- [x] T4.1 Epic priority, epic labels, epic origins move into the panel as
      `SinglePicker` / `MultiPicker`; delete `EpicLabelFilter.tsx` and
      `EpicOriginFilter.tsx` (FR3).
- [x] T4.2 `filterChips` becomes `ActiveFilter[]` keyed by `id`; the origin
      chip keeps its reset (US3.5).
- Tests: the existing roadmap browser tests (`roadmap-epic-axes`,
  `roadmap-epic-labels`, `roadmap-epic-navigation`, `roadmap-origins`) open the
  panel instead of the removed controls.

## 5. Triage, Sprint timeline, Team, Activities

- [x] T5.1 Triage: `FilterSearch` inline, sprint and parent pickers in the
      panel, chips (US2.3).
- [x] T5.2 Sprint timeline: parent drop-down replaced by `SinglePicker`,
      backlog search by `FilterSearch`, chips.
- [x] T5.3 Team: team `SinglePicker` inline, not counted, no chip (US1.3).
- [x] T5.4 Activities: `FilterSearch`, skill picker in the panel, chip; the
      status buttons and tiles are kept (spec US2.5).
- [x] T5.5 Delete the strings the removed controls used (FR4).
- Tests: `view-filters.browser.mjs` cases for each view, including "no native
  `<select>` as a filter" (FR2) and "same items before and after" (US4.1);
  existing browser tests updated.

## 6. Finish

- [x] T6.1 Check the out-of-scope selects are untouched (`git diff` on
      `ListView.tsx:615`, `SprintTimelineView.tsx:1211`,
      `TrackerBacklogView.tsx`).
- [x] T6.2 `CHANGELOG.md`: one `Changed` line under `## [Unreleased]` (FR6).
- [x] T6.3 `make test`: web green; one Go test, `TestTryPullLocalAgentTasks`, fails on a TempDir cleanup race with no Go change on the branch.
