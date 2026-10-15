# Tasks #841 - Always provide the ticket link in WebUI toasts

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Types, strings and pure helpers

- [ ] T1.1 Add `moreLinks?: number` to `ToastMessage`.
- [ ] T1.2 Add `toasts.moreTickets` and `toasts.ticketUnavailable` to the
      catalog type and to both languages (FR5).
- [ ] T1.3 Add `issueRefLink`, `cappedLinks` and `withTicketLinks` to
      `web/src/lib/issueToast.ts` (FR3, FR4, US4).
- Tests (`web/tests/issueToast.test.mjs`): `issueRefLink` with id and key,
  without id, without key; `cappedLinks` with 3, 5, 7 links and with
  `undefined` entries (US4.1-US4.3); `withTicketLinks` keeps an existing `link`
  first, merges `links`, sets `moreLinks` only when positive, returns the toast
  unchanged with nothing to add.

## 2. Rendering

- [ ] T2.1 `ToastContainer` renders the "+N more" line after the links (US4.2).
- Tests: browser test, a toast with 5 links and `moreLinks: 2` shows five
  "Open" buttons and the text "+2 more".

## 3. Context plumbing

- [ ] T3.1 `openTaskById` with board lookup, server read and the
      `ticketUnavailable` error (US2.2-US2.4).
- [ ] T3.2 `ticketLink` and `ticketLinks`, exposed on the context (FR2).
- [ ] T3.3 Optional ticket argument on `refusalToast` and `tokenOfferToast`
      (US3).
- Tests: browser test, lazy link of an unloaded id opens the answer of
  `GET /api/tasks/{id}`; a 404 shows "Ticket J-9 could not be opened" and opens
  nothing; a token refusal shows the offer button before "Open {key}".

## 4. Single-ticket handlers in `AppContext.tsx`

- [ ] T4.1 Activity poller: completed, failed, token offer (US2.1, US3.2).
- [ ] T4.2 `runSkill`, `retryActivity`, `cancelActivity`.
- [ ] T4.3 `createTask` warnings, `cloneTask` and `updateTask` errors.
- [ ] T4.4 `syncSingleTask`, `moveTaskToTrackerStatus`, `moveTask`,
      `moveTaskWorkflowStage`, `transitionTaskStage`, `confirmInteractiveStep`.
- [ ] T4.5 `getTaskComments`, `postTaskComment`, `setTaskTeam`,
      `setTaskSprint`, `setTaskMacro`, `togglePin`, `convertTask`.
- [ ] T4.6 `deleteTask`: link on the error only (US5).
- Tests: browser test (extend `issue-toast-links.browser.mjs`), for
  `syncSingleTask` success and `updateTask` error, "Open {key}" opens the
  ticket; a simulated activity going from `running` to `failed` for a loaded
  ticket shows "Open {key}" with its tracker icon, for an unloaded one shows
  "Open {key}" and opens it after the read; `deleteTask` success shows no
  button.

## 5. Multi-ticket handlers

- [ ] T5.1 `createBatchTasks` success, `setTasksSprint`, `setTasksTeam`,
      `moveTasksToMacro`, `migrateTasks` (success and error).
- Tests: browser test, `setTasksSprint` with 3 loaded ids shows 3 links, with 7
  shows 5 links and "+2 more".

## 6. Components

- [ ] T6.1 `TaskCard`, `CopyStepPromptButton`, `TaskDetailModal`,
      `TrackerBacklogView`.
- [ ] T6.2 `RemoteRunBadge`, `ExecutionRecovery`.
- [ ] T6.3 `ListView` bulk toasts (deletion success without link),
      `SprintTimelineView` backlog planned/assigned.
- Tests: covered by the context helpers; run the existing suites of these
  components to check nothing regressed.

## 7. Wrap-up

- [ ] T7.1 `CHANGELOG.md`, `[Unreleased]` / `Changed`: "WebUI notifications
      about a ticket (runs, stage moves, syncs, comments, sprint or team
      changes, errors) now always offer a link to open it; notifications about
      several tickets list up to five. (#841)" (AC3).
- [ ] T7.2 Run `npm run lint`, `npx tsc -b`, `npm test` and the toast browser
      test from `web/` (AC4).
- [ ] T7.3 Check the inventory of spec FR6 against `git grep addToast web/src`
      once more: every remaining unlinked toast is out of scope by the spec.
