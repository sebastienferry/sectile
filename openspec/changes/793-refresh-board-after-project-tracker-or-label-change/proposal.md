## Why
When a user saves a project after adding, removing, or reordering its trackers or editing its label (`#741`), `updateProject` in `web/src/context/AppContext.tsx` only refreshes `fetchProjects()` and does not re-fetch the tasks or filter facets, leaving the open board and list with stale ticket membership until a manual page reload.

## What Changes
- Refresh `fetchTasks()` and `fetchTaskFacets()` in `updateProject` after `fetchProjects()` whenever the saved payload carries `trackers` or `label` and the updated project is part of the active board scope (the selected project, the `'all'` scope, or the active saved board view).
- Update `tasksInProject` (`web/src/lib/taskIdentity.ts`) to inspect `task.projectIds` (falling back to `[task.projectId]`) so tickets belonging to multiple projects are retained in both single-project and bookmarked `'all'` scopes.

## Capabilities
### New Capabilities
- project-membership-refresh: immediate reload of open board, list, and filter facets when a project's tracker selection or label is saved.

### Modified Capabilities
None.

## Impact
Web client context (`web/src/context/AppContext.tsx`), pure helpers (`web/src/lib/projectTrackers.ts`, `web/src/lib/taskIdentity.ts`), their unit tests (`web/tests/projectTrackers.test.mjs`, `web/tests/taskIdentity.test.mjs`), and `CHANGELOG.md` (`Fixed` under `## [Unreleased]`).

## Non-goals
Server-side `PUT /api/projects/:id`, `GET /api/tasks`, or `GET /api/tasks/facets` query changes (the server already computes updated membership immediately), and refreshing tasks on unrelated project metadata edits such as sprint dates.

## Open Questions
None.
