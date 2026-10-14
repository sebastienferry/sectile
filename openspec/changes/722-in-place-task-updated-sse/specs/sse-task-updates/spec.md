## ADDED Requirements

### Requirement: SSE `task_updated` Events Update Browser State In Place Without Full List Reloads
When a `task_updated` SSE event is delivered, each connected browser SHALL update its task list, selected task, activity list, and activity statistics in place from the event's `task` and `activity` payload instead of re-fetching `/api/tasks`, `/api/activities`, and `/api/activities/stats` in full.

#### Scenario: Updated task and activity are applied in place across replicas
- **WHEN** a task or activity is updated on any server replica and a `task_updated` SSE event carrying `task` and/or `activity` reaches a connected browser
- **THEN** the browser updates `tasks`, `selectedTask`, `activities`, and `activityStats` in place according to its active project/view and filters without issuing full list HTTP requests

#### Scenario: Task moving into or out of the active board scope
- **WHEN** a `task_updated` event carries a task whose updated project membership, status, or labels move it out of (or into) the browser's active project/view and filters
- **THEN** `applyUpdatedTask` removes the task from (or inserts the task into) the displayed `tasks` list in place

#### Scenario: Burst of bare `task_updated` events coalesces into at most one refetch
- **WHEN** multiple `task_updated` events carrying neither `task` nor `activity` arrive within a burst window
- **THEN** `createCoalescedRefetch` coalesces the burst into a single reload rather than one reload per event
