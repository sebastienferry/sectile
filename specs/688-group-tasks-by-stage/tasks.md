# Tasks #688 - Group a project's tasks by stage

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. One stage list (FR3)

- [x] T1.1 Export `STAGES` from `desktop/src/workflow.mjs` and use it for the
  module's own `stages`.
- [x] T1.2 `desktop/src/task-list-order.mjs` imports `STAGES` instead of its
  private `STAGE_ORDER`.
- [x] T1.3 `desktop/tests/workflow.test.mjs`: `STAGES` is the six stages in
  workflow order; `task-list-order.test.mjs` passes unchanged.

## 2. Ordering by stage (FR3, FR4)

- [x] T2.1 `orderedTaskGroups(groups,{stageOf}={})` in
  `desktop/src/task-order.mjs`: stage rank first when `stageOf` is given,
  unknown last, then the existing comparison.
- [x] T2.2 `desktop/tests/task-order.ui.cjs`: existing cases unchanged without
  `stageOf`; new cases for the stage order, ties in current order, unstaged
  last, input not mutated.

## 3. Stage cache (FR8, FR9)

- [x] T3.1 `taskStages` map beside `taskTitles` in `desktop/src/main.js`.
- [x] T3.2 `refreshPRs` stores `taskStage(task)` when it is one of `STAGES`,
  deletes the entry otherwise; a failed request leaves it.

## 4. The option (FR1, FR2)

- [x] T4.1 `stageGroupedProjects` set read from `localStorage`
  (`stageGroupedProjects`), `toggleStageGrouping(projectID)`.
- [x] T4.2 "Group by stage" item in `projectMenu`, after "Show execution
  queue", as `menuitemcheckbox` with `aria-checked` and a check glyph.
- [x] T4.3 Widen the menu's focus and arrow selectors to `[role^=menuitem]`.
- [x] T4.4 Pass `stageOf` to `orderedTaskGroups` for grouped projects.
- [x] T4.5 `requestRemoveProject` forgets the project's choice on success.

## 5. The tint (FR5, FR6, FR7)

- [x] T5.1 The key button gets `data-stage` and a title naming the stage for
  a staged row only.
- [x] T5.2 `desktop/src/style.css`: six `--stage-*-bg` tokens in the dark and
  the light token blocks; the `[data-stage]` rules on the sidebar key.
- [x] T5.3 Keep the key column aligned: adjust `task-key-slot` for every row
  if the tint's padding needs it; `sidebar-alignment.ui.cjs` passes.

## 6. UI test, changelog

- [x] T6.1 `desktop/tests/stage-grouping.ui.cjs`: entry unchecked by default;
  checking reorders the rows and sets `aria-checked`; survives a reload;
  another project unaffected; keys carry `data-stage` and the stage title;
  a free console carries none.
- [x] T6.2 `CHANGELOG.md`: the `Added` line of the plan under
  `[Unreleased]`.

## 7. Gates

- [x] T7.1 `cd desktop && npx vite build`, then `node --test tests/` (sandbox
  off for the Electron suites); restore `internal/webui` `.gitkeep` if a web
  build removed it.
- [x] T7.2 Screenshots of the sidebar in the light and dark themes, grouping
  on and off, attached to the pull request.
- [x] T7.3 `git diff origin/main --stat` reviewed: only this change and the
  four base commits of `feat/desktop-conversation-test`.

## Notes from the implementation

- T5.3: every sidebar key now has the same `2px 4px` box and rounded corners,
  tinted or not, and the key slot moved 4px left with 8px more minimum width,
  so the key text keeps its place and the title column stays one column.
- T7.2: the screenshots come from `stage-grouping.ui.cjs`, which switches the
  theme with `page.emulateMedia`; setting `nativeTheme.themeSource` from the
  test did not reach the page in time.
- The menu check mark is a trailing `✓` drawn by CSS from `aria-checked`, so
  the other menu entries keep their alignment.
