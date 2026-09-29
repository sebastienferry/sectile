# Tasks #639 - Switch to a ticket's new execution

Order matters: the pure rule first, then its wiring, then the UI test and the
changelog. Each task names its test.

## Phase 1 - The rule (FR-001, FR-002, FR-004, FR-005)

- [ ] T001 Export `followedExecution(previous,next,selectedId,keyOf,eligible)`
  from `desktop/src/task-order.mjs`, ranking with `compareRuns`.
  Test: `desktop/tests/task-order.test.mjs` (new) - new execution of the
  displayed ticket returned; another ticket's, an empty previous list, an
  ineligible shown or new execution, no new execution give `null`; with two
  new executions, the active one wins over the queued one, and the newest
  wins between two of the same state.

## Phase 2 - Wiring (FR-003, FR-006, FR-007, FR-008)

- [ ] T002 `desktop/src/main.js` `refresh()`: compute `followedExecution` from
  `runs` and `next` before `runs=next`, with `taskKey` and the eligibility
  `!freeConsole && !macroRun && !hiddenRun`; after `updateDisconnected`,
  select it in the background (`select(followed,true,{deferrable:true})`),
  otherwise keep the existing re-selection of the displayed execution.
- [ ] T003 `launchTaskWork`: select the launched execution only when it is not
  already the selected one.

## Phase 3 - UI test (US1, US2, US3)

- [ ] T004 `desktop/tests/follow-new-execution.ui.cjs` (new): fake agent with a
  mutable run list. Scenarios:
  - #1's finished execution on display, a running execution of #1 appears →
    the title and `#execution-history` show it (US1.1);
  - the older execution picked from the history, another one appears → switch
    (US1.4);
  - a queued execution of #1 appears → the console notice is shown; it turns
    running → the console attaches (US1.5);
  - a new execution of #2 appears → #1 stays on display (US2.1);
  - a free console on display, a ticket execution appears → no switch
    (US2.2);
  - the Tickets pane open → it stays open after the switch (US3.1).
  Build the renderer with `npx vite build` first.

## Phase 4 - Changelog (US4)

- [ ] T005 `CHANGELOG.md` `[Unreleased]` → `### Fixed`: "**The desktop console
  follows a ticket's new execution.** When a new execution of the ticket on
  display starts or is queued, whether launched from the desktop, the web
  board or another client, the console switches to it; earlier executions
  stay in the execution history. (#639)"

## Phase 5 - Verification

- [ ] T006 Run `node --test tests/task-order.test.mjs` and the new UI test
  alone, then `tests/execution-history.ui.cjs`, `tests/next-step.ui.cjs` and
  `tests/task-order-hold.ui.cjs`, which touch the same selection paths. The
  full suites run in the pull request pipeline.
