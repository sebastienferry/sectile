# Stabilise the desktop sidebar task list

## Why
Desktop task rows change position on their own while the user is reading or clicking them (#69). Three causes are established in `docs/clarifications/69.md`: the ordering time basis flips from submission time to actual start time when a run starts, a relaunch makes a new execution the group representative and teleports the row, and a 2 s poll re-applies the order over a fully rebuilt DOM so the movement lands under the cursor.

## What Changes
- Order task groups on an immutable anchor: the earliest submission time among the group's visible executions, never the actual start time. A row therefore stops moving when a run starts or when a new run is launched on the same task.
- Keep the state grouping introduced by #72: active before queued before finished. Only a genuine state change repositions a row.
- Defer refresh-driven sidebar re-renders while the pointer is over the task list or keyboard focus is inside it, and apply the pending order as soon as that interaction ends. Live status badges keep updating during the freeze.
- Amend the #72 ordering contract and its tests accordingly, and add stability coverage.

## Scope
Desktop sidebar task list only: `desktop/src/task-order.mjs`, the sidebar rendering in `desktop/src/main.js`, and the desktop test suite. No server, API, database, web board, execution queue view, or task-modal change. The `startedAt` field stays exposed by the local agent run contract; it simply stops driving row order.

## Impact
`desktop/src/task-order.mjs`, `desktop/src/main.js`, `desktop/tests/task-order.ui.cjs`, `desktop/tests/task-order-render.ui.cjs`, spec `desktop-task-ordering`. Settled clarification: `docs/clarifications/69.md`.
