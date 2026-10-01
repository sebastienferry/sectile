# Tasks #688 - Group a project's tasks by stage

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. One stage list (FR3)

- [ ] T1.1 Export `STAGES` from `desktop/src/workflow.mjs` and use it for the
  module's own `stages`.
- [ ] T1.2 `desktop/src/task-list-order.mjs` imports `STAGES` instead of its
  private `STAGE_ORDER`.
- [ ] T1.3 `desktop/tests/workflow.test.mjs`: `STAGES` is the six stages in
  workflow order; `task-list-order.test.mjs` passes unchanged.

## 2. Ordering by stage (FR3, FR4)

- [ ] T2.1 `orderedTaskGroups(groups,{stageOf}={})` in
  `desktop/src/task-order.mjs`: stage rank first when `stageOf` is given,
  unknown last, then the existing comparison.
- [ ] T2.2 `desktop/tests/task-order.ui.cjs`: existing cases unchanged without
  `stageOf`; new cases for the stage order, ties in current order, unstaged
  last, input not mutated.

## 3. Stage cache (FR8, FR9)

- [ ] T3.1 `taskStages` map beside `taskTitles` in `desktop/src/main.js`.
- [ ] T3.2 `refreshPRs` stores `taskStage(task)` when it is one of `STAGES`,
  deletes the entry otherwise; a failed request leaves it.

## 4. The option (FR1, FR2)

- [ ] T4.1 `stageGroupedProjects` set read from `localStorage`
  (`stageGroupedProjects`), `toggleStageGrouping(projectID)`.
- [ ] T4.2 "Group by stage" item in `projectMenu`, after "Show execution
  queue", as `menuitemcheckbox` with `aria-checked` and a check glyph.
- [ ] T4.3 Widen the menu's focus and arrow selectors to `[role^=menuitem]`.
- [ ] T4.4 Pass `stageOf` to `orderedTaskGroups` for grouped projects.
- [ ] T4.5 `requestRemoveProject` forgets the project's choice on success.

## 5. The tint (FR5, FR6, FR7)

- [ ] T5.1 The key button gets `data-stage` and a title naming the stage for
  a staged row only.
- [ ] T5.2 `desktop/src/style.css`: six `--stage-*-bg` tokens in the dark and
  the light token blocks; the `[data-stage]` rules on the sidebar key.
- [ ] T5.3 Keep the key column aligned: adjust `task-key-slot` for every row
  if the tint's padding needs it; `sidebar-alignment.ui.cjs` passes.

## 6. UI test, changelog

- [ ] T6.1 `desktop/tests/stage-grouping.ui.cjs`: entry unchecked by default;
  checking reorders the rows and sets `aria-checked`; survives a reload;
  another project unaffected; keys carry `data-stage` and the stage title;
  a free console carries none.
- [ ] T6.2 `CHANGELOG.md`: the `Added` line of the plan under
  `[Unreleased]`.

## 7. Gates

- [ ] T7.1 `cd desktop && npx vite build`, then `node --test tests/` (sandbox
  off for the Electron suites); restore `internal/webui` `.gitkeep` if a web
  build removed it.
- [ ] T7.2 Screenshots of the sidebar in the light and dark themes, grouping
  on and off, attached to the pull request.
- [ ] T7.3 `git diff origin/main --stat` reviewed: only this change and the
  four base commits of `feat/desktop-conversation-test`.
