# Tasks — #115

1. [ ] In `desktop/src/main.js`, compute the selected flag once per task group and apply
       `selected` to the `.local-task` row while keeping it on the `.run` button.
2. [ ] In `desktop/src/style.css`, paint the selection on `.local-task.selected`
       (background `#203033`, `border-radius: 8px`) and set the scoped
       `.local-task .run.selected` background to transparent.
3. [ ] Extend `desktop/tests/task-order-render.ui.cjs` with an assertion that the selected
       row container carries the highlight and that a non-selected row does not.
4. [ ] Run the desktop test suites (`npm run test`, `npm run test:ui`) and the repository
       build; quote the real output.

## Test plan

- UI: selecting a task marks `.local-task.selected`, and switching selection moves it.
- UI: existing `.run.selected` locators still resolve (regression guard).
- Manual: the highlight visually spans from the task ID to the trailing controls.
