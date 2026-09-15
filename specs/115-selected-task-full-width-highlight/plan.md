# Implementation plan — #115

## Stack

Electron desktop renderer: plain DOM rendering in `desktop/src/main.js`, plain CSS in
`desktop/src/style.css`, Playwright UI tests under `desktop/tests/*.ui.cjs` run by
`node --test`.

## Architecture

Selection state already lives in the module-level `selected` execution id. `renderRuns()`
rebuilds each `.local-task` row on every render, so the selected flag only needs to be
computed once per row and applied to both the row and the `.run` button.

## Target files

| File | Change |
| --- | --- |
| `desktop/src/main.js` | Compute `isSelected` once per task group; add `selected` to the row `className` alongside the existing `.run` class. |
| `desktop/src/style.css` | Add `.project-group .local-task.selected` background and radius; make `.project-group .local-task .run.selected` transparent. |
| `desktop/tests/task-order-render.ui.cjs` | Extend with an assertion on the row-level highlight. |

## Data contracts

None. No IPC, storage or server payload is touched.

## Risks

`.run.selected` is asserted by existing UI tests; the class must be preserved. CSS
specificity must keep the scoped `.project-group .local-task .run.selected` transparent
rule ahead of the generic `.run.selected` rule.
