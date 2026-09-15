# Implementation checklist

- [x] 1. Write and validate the OpenSpec change under `openspec/changes/69-sidebar-task-order-stability/` (`openspec validate --strict`).
- [x] 2. Rework `desktop/src/task-order.mjs`: drop `startedAt` from the sort key, add the immutable group anchor (earliest parsable `createdAt` among visible executions), keep state rank, unknown-time placement and identity ties.
- [x] 3. Update `desktop/tests/task-order.ui.cjs` to the new contract: anchor-based order, no `startedAt` influence, relaunch keeps the anchor, fallbacks and ties unchanged.
- [x] 4. Add the interaction hold in `desktop/src/main.js`: `render({deferrable:true})` from the refresh path, `pendingRender` when the pointer or focus is on a `.local-task` row, targeted status refresh during the hold.
- [x] 5. Flush the pending render on `pointerout`/`focusout` of `#runs` (next tick, once hover and focus have settled); keep user-initiated renders immediate.
- [x] 6. Add `desktop/tests/task-order-hold.ui.cjs` and realign `desktop/tests/task-order-render.ui.cjs`: the row sequence is stable while hovering across a state change, the badge still updates, and the new order applies once the pointer leaves.
- [x] 7. Build the desktop app (`npm --prefix desktop run build`).
- [x] 8. Run the desktop UI suite (`npm --prefix desktop run test:ui`) and the repository suite (`make test`); report real output.
- [x] 9. Re-read the diff, merge `origin/main`, commit, push and mark the pull request ready.
