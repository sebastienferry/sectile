# Plan #639 - Switch to a ticket's new execution

Behaviour: `spec.md`. This file says where and how.

## Stack

- Desktop: Electron renderer in plain ES modules (`desktop/src`).
- Tests: `node --test` unit tests (`desktop/tests/*.test.mjs`) and Playwright
  Electron UI tests (`desktop/tests/*.ui.cjs`) against a fake agent.
- No server, agent or web change; no new dependency.

## Architecture

### One pure rule, in `task-order.mjs`

The ordering that picks a row's representative already lives in
`desktop/src/task-order.mjs` (`compareRuns`). The rule that picks the
execution to follow joins it, so the row and the console rank executions the
same way (FR-002):

```js
// followedExecution returns the execution the console moves to after a poll
// (#639): among the executions next lists and previous did not, those of the
// displayed execution's ticket, ranked like the row ranks them. keyOf groups
// executions per row; eligible filters out what the console never follows.
export function followedExecution(previous,next,selectedId,keyOf,eligible){
 const shown=previous.find(run=>run.id===selectedId)
 if(!shown||!eligible(shown))return null
 const known=new Set(previous.map(run=>run.id)),key=keyOf(shown)
 const fresh=next.filter(run=>!known.has(run.id)&&keyOf(run)===key&&eligible(run))
 return fresh.sort(compareRuns)[0]||null
}
```

- `shown` is looked up in the **previous** list: when the list is empty (first
  poll after start, or after `restartLocalAgent`/`stopLocalAgent` reset
  `runs=[]`) there is nothing on display to follow, which gives the baseline of
  FR-005 without a flag.
- `eligible` is supplied by `main.js`: not `freeConsole`, not `macroRun`, not
  `hiddenRun` (hidden project or archived execution) (FR-004). A free console
  has its own key anyway; the macro exclusion is the settled scope.

### Wiring in `refresh()`

`refresh()` (`desktop/src/main.js`) already compares the two polls before
replacing `runs` (`announce(transitions(runs,next))`). The follow rule is
computed at the same place, from the same two lists, and applied after the
existing re-selection of the execution on display:

```js
const followed=followedExecution(runs,next,selected,taskKey,run=>!freeConsole(run)&&!macroRun(run)&&!hiddenRun(run))
...
runs=next;last=serialized
await updateDisconnected(...)
const current=runs.find(run=>run.id===selected)
if(followed&&!hiddenProject(followed.projectId))select(followed,true,{deferrable:true})
else if(current&&(...status/session change...))select(current,true,{deferrable:true})
if(!selected){...}
```

- `hiddenRun` reads `disconnectedProjects`, which `updateDisconnected` updates
  after the lists are compared; `select` itself returns early for a hidden
  project, so the re-check at the call site only avoids the `else` branch
  being skipped for nothing.
- `select(run,true,{deferrable:true})` is the background selection the poll
  already uses: it keeps the Tickets pane open and defers the sidebar rebuild
  while `sidebarBusy()` (FR-006). For a queued or preparing execution it writes
  the `consoleNotice` and detaches; the existing status-change branch attaches
  it once it runs (FR-007).
- Every launch source is covered by the poll: the Relaunch dialog, the Tickets
  pane and `confirmDeclareReviewed` call `refresh()` after launching, and the
  web board's launches arrive with the next periodic poll (FR-003).

### `launchTaskWork`

`launchTaskWork` selects the launched execution itself after `await
refresh()`. When that refresh ran, the follow rule has already selected it in
the background, and the explicit `select(launched)` would reset and attach the
console a second time. It is kept for the case where `refresh()` returned
early (a poll already in flight), and guarded:

```js
if(launched&&launched.id!==selected)select(launched)
```

(FR-008). No other launch path changes.

## Data contracts

None. The agent's `/desktop/runs` list already carries `id`, `taskId`,
`projectId`, `kind`, `macroKey`, `status` and `createdAt`.

## Tests

- Unit (`desktop/tests/task-order.test.mjs`, new): `followedExecution`
  returns the new execution of the displayed ticket; `null` for another
  ticket's execution, an empty previous list, an ineligible shown or new
  execution, no new execution; the active one first when several appear, then
  the newest.
- UI (`desktop/tests/follow-new-execution.ui.cjs`, new): a fake agent whose
  `runs` array the test mutates between polls, in the style of
  `task-order-hold.ui.cjs`. Scenarios: finished execution on display → new
  running execution selected (title, `#execution-history` value); picked older
  execution from the history → switch; new queued execution → notice shown,
  then attach once it runs; new execution of another ticket → no switch; free
  console on display → no switch; Tickets pane open → stays open.
- The UI tests need a renderer build (`npx vite build` in `desktop/`); restore
  `internal/webui/dist/.gitkeep` afterwards if the build removes it.

## Documentation

- `CHANGELOG.md` `[Unreleased]` → `### Fixed`: one line.
- No ADR: a local UI behaviour, no trade-off beyond the settled decisions.

## Risks

- **A new execution the previous poll missed for another reason.** The list
  only grows by new executions or after a reset to `[]`, which FR-005 covers.
  Un-archiving never changes an execution's ID.
- **Switching while the user types in a live console.** Accepted by the
  owner (round 2): the history drop-down is the way back.
