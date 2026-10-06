# Tasks #671 - Roadmap: move the mode switcher into the side panel

Order matters: each step leaves the tree building and green.

## 1. The list follows the horizon tab (US2)

- [x] T1.1 `RoadmapView.tsx`: add `sprintCheckHere` and use it in
      `visibleRows`, the unfolded row badge, the "À corriger" toggle, the
      sprint strip and the empty-list text (plan D1).

## 2. The switcher moves into the panel (US1, US3)

- [x] T2.1 Add `modes.label` to both locales in `web/src/locales/planning.ts`
      (plan D3).
- [x] T2.2 Remove the toolbar switcher and render the `tablist` at the bottom
      of the panel header (plan D2).

## 3. Tests

- [x] T3.1 Switch the four existing tests to `getByRole('tab', …)`.
- [x] T3.2 Add `web/tests/roadmap-mode-tabs.browser.mjs` (plan, test
      strategy), covering acceptance criteria 1 to 5.

## 4. Documentation

- [x] T4.1 `CHANGELOG.md`: one `Changed` line under `[Unreleased]`.

## Test plan

- `cd web && npx tsc -b && npx oxlint`
- `cd web && node --test tests/*.test.mjs`
- `cd web && node tests/roadmap-mode-tabs.browser.mjs`, then the four updated
  browser tests and `roadmap-view.browser.mjs`.
- Manual: on the Roadmap, select a macro on Now, switch the panel to Framing:
  row badges, toggle and sprint strip stay; hide the panel: no switcher left.
