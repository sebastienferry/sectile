# Design: In-Place `task_updated` SSE Updates (#722)

## Context

- `Handler.BroadcastEvent` (`internal/handlers/handlers.go:229`) sends `Event{ Type: "task_updated", Task, Activity, Error }` to local SSE subscribers and publishes `BusMessage{ Type: "task_updated", TaskID, ActivityID, Error }` on the database bus.
- On receiving replicas, `deliverRelayedEvent` (`internal/handlers/handlers.go:258`) reads `GetTaskByID(msg.TaskID)` and `GetActivityByID(msg.ActivityID)` once per replica and broadcasts the hydrated `Event` to its local browsers.
- On the writing replica, `RegisterPostBackListener` (`internal/handlers/handlers.go:113`) receives `task` and `activity` from `notifyPostBackListeners`. Re-reading `GetTaskByID(task.ID)` and `GetActivityByID(activity.ID)` when present guarantees that computed fields (`ProjectID`, `ProjectIDs`, `Batch`, `UserName`, `TaskKey`, `TaskTitle`) are identical on both the writing replica and relayed replicas.

## Decisions

1. **In-Place Task & Activity State Updates (`web/src/lib/taskUpdatedEvent.ts`)**:
   - `applyUpdatedTask(tasks, incoming, scope)`:
     - Evaluates whether `incoming` matches the active project/saved-view scope and active board filters (`status`, `priority`, `label`, `sprint`, `team`, `parent`, `assignee`, `myTasksOnly`, `trackerStatuses`, `issueTypes`, `pinnedOnly`, and server-applicable `searchQuery`).
     - When `incoming` is already in `tasks` (`sameTask`): replaces it in place if it still matches `scope`, or removes it if it moved out of `scope`.
     - When `incoming` is not in `tasks`: prepends it if it matches `scope`, or leaves `tasks` unchanged.
   - `applyUpdatedActivity(activities, stats, incoming, selectedProjectId, task)`:
     - When `incoming.id` is already in `activities`: replaces it in place and adjusts `stats` status counters if `status` changed.
     - When `incoming.id` is not in `activities` and `incoming` belongs to `selectedProjectId` (or `selectedProjectId === 'all'`): prepends `incoming` and increments `stats.total` and the bucket for `incoming.status`.
   - `createCoalescedRefetch(delayMs, run)`:
     - Used only when a `task_updated` event carries neither `task` nor `activity`, coalescing a burst of bare events into a single refetch.

2. **Stable SSE Subscription (`web/src/context/AppContext.tsx`)**:
   - Read the latest board scope and callbacks through a ref inside the `EventSource` listener so changing `selectedTask` or filters does not close and reopen `/api/events`.
