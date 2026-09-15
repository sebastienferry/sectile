# Implementation checklist

- [ ] 1. Write and validate the OpenSpec change under `openspec/changes/69-sidebar-task-order-stability/` (`openspec validate --strict`).
- [ ] 2. Rework `desktop/src/task-order.mjs`: drop `startedAt` from the sort key, add the immutable group anchor (earliest parsable `createdAt` among visible executions), keep state rank, unknown-time placement and identity ties.
- [ ] 3. Update `desktop/tests/task-order.ui.cjs` to the new contract: anchor-based order, no `startedAt` influence, relaunch keeps the anchor, fallbacks and ties unchanged.
- [ ] 4. Add the interaction hold in `desktop/src/main.js`: `render({deferrable:true})` from the refresh path, `pendingRender` when the pointer is over `#runs` or focus is inside it, targeted status refresh during the hold.
- [ ] 5. Flush the pending render on `pointerleave` of `#runs` and on a `focusout` that leaves `#runs`; keep user-initiated renders immediate.
- [ ] 6. Extend `desktop/tests/task-order-render.ui.cjs`: the row sequence is stable while hovering across a state change, the badge still updates, and the new order applies once the pointer leaves.
- [ ] 7. Build the desktop app (`npm --prefix desktop run build`).
- [ ] 8. Run the desktop UI suite (`npm --prefix desktop run test:ui`) and the repository suite (`make test`); report real output.
- [ ] 9. Re-read the diff, commit, push and update the pull request.
