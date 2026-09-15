# Design

## Context
`orderedTaskGroups()` sorts task groups with `compareRuns`, whose key is `(stateRank, executionTime, identity)`. `executionTime()` returns `createdAt` while a run is `queued`/`preparing` and `startedAt` afterwards, so the key changes value the moment a run starts. `render()` rebuilds the whole `#runs` list every time the 2 s poll observes any payload difference, so a re-sorted list is also a recreated DOM.

## Decisions

### 1. Immutable group anchor instead of actual start time
Group order uses `anchorTime(executions)` = the smallest parsable `createdAt` among the group's visible executions, newest first. `startedAt` no longer participates in ordering at all.

- A run starting does not move its row (cause 2 removed).
- A relaunch adds an execution whose `createdAt` is later than the anchor, so the anchor is unchanged and the group keeps its position (cause 3 removed). The new execution still becomes the row's representative, so the badge and the state group update — that is a state change, which #72 legitimately reorders on.
- Rejected: keeping `startedAt` and freezing only during interaction. Rows would still jump whenever the user is not hovering, which is the behaviour the ticket reports.
- Rejected: a frozen first-seen order per project (clarification option b). It drops #72's active/queued/finished grouping, which the clarification chose to preserve.

Representative selection inside a group keeps `(stateRank, newest createdAt, identity)`, so it is derived from the same immutable basis as the order.

Unparsable and missing `createdAt` values keep the existing fallback semantics: a group with no usable timestamp sorts last within its state group, ties resolve on ascending task then execution identity.

### 2. Defer refresh-driven renders instead of keyed row reconciliation
The clarification asked for keyed row reuse so a rebuild does not drop hover and focus. Instead, `render()` accepts a `deferrable` flag used only by the refresh path (`updateDisconnected` and the two `select()` calls in `refresh()`). When the pointer is over a `.local-task` row or focus is inside one, a deferrable render records `pendingRender`, refreshes the header and the per-row indicators through the targeted `renderTaskRowStates()`, and returns before touching the list. `pointerout` and `focusout` on `#runs` schedule a flush on the next tick, once the new hover and focus targets have settled.

The hold is scoped to the task rows, not to the whole `#runs` container: project headings, the capacity counters and the execution queue view keep refreshing live, since none of them move under the cursor.

Rationale: no DOM is recreated during the interaction, so hover, focus and an in-progress click survive by construction, and the change stays local. Keyed reconciliation would have required reconciling project sections too (the list is cleared with `replaceChildren()`, which detaches rows and loses focus anyway), for the same observable result.

User-initiated renders — selection, collapse, queue toggle, rename, archive — call `render()` without the flag and always apply immediately, so the UI never feels stuck while the pointer sits in the sidebar.

Trade-offs: while the pointer rests over a row, a state change updates its badge but the row keeps its position until the pointer leaves — that is the intended acceptance criterion. `renderTaskRowStates()` resolves a row by the execution id stamped on it, so an execution launched during the hold only appears once the hold is lifted.

## Risks
- A pointer left over the sidebar indefinitely keeps the order stale. Accepted: the badges stay live and any click flushes the order.
- `pointerleave` does not fire if the window loses focus with the pointer inside. A `blur`-triggered flush is out of scope; the next user action renders.
