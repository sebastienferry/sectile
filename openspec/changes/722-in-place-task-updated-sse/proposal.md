# Proposal: Stop Every Browser Reloading Three Full Lists on Each `task_updated` Event (#722)

## Why

In `web/src/context/AppContext.tsx`, the SSE `task_updated` listener currently calls `fetchTasks()`, `fetchActivities()`, and `fetchActivityStats()` on every event. With `N` open browsers, each task update triggers `3 × N` full list reads (`GET /api/tasks`, `GET /api/activities`, `GET /api/activities/stats`) on top of regular polling, even though the SSE `task_updated` event payload already carries the updated `task` and `activity`.

## What Changes

- Ensure `NewHandler`'s post-back listener (`internal/handlers/handlers.go`) hydrates `event.Task` via `GetTaskByID` and `event.Activity` via `GetActivityByID` before broadcasting, matching `deliverRelayedEvent` on other replicas so `ProjectID`, `ProjectIDs`, `Batch`, `UserName`, `TaskKey`, and `TaskTitle` are always populated on every replica.
- Add `web/src/lib/taskUpdatedEvent.ts` with pure helpers (`taskMatchesBoardScope`, `applyUpdatedTask`, `activityMatchesProjectScope`, `applyUpdatedActivity`, `createCoalescedRefetch`) to update `tasks`, `selectedTask`, `activities`, and `activityStats` in place from the `task_updated` SSE event payload without issuing full list reloads.
- Update `web/src/context/AppContext.tsx` to apply `task_updated` events in place, keep `EventSource('/api/events')` stable across `selectedTask` and filter changes, and fall back to a single coalesced refetch only when a bare event carries neither `task` nor `activity`.

## Impact

- Affected specs: `sse-task-updates`
- Affected code:
  - `internal/handlers/handlers.go`
  - `internal/handlers/postback_test.go`
  - `web/src/lib/taskUpdatedEvent.ts`
  - `web/src/context/AppContext.tsx`
  - `web/tests/taskUpdatedEvent.test.mjs`
  - `CHANGELOG.md`
