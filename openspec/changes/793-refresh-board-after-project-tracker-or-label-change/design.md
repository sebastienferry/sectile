## Context
`updateProject` (`web/src/context/AppContext.tsx`) sends `PUT /api/projects/:id` and calls `await fetchProjects()`, whereas `deleteProject`, `toggleProjectBookmark`, and `updateBoardView` also call `fetchTasks()` and `fetchTaskFacets()`. Since `#741`, a project's ticket membership is determined by its selected `trackers` and `label`. In addition, `tasksInProject` (`web/src/lib/taskIdentity.ts`) only inspects `task.projectId`, whereas multi-project tickets (`#741`) expose all matching project IDs in `task.projectIds`.

## Decisions
1. **Pure scope-and-payload predicate (`projectSaveRefreshesBoard`)**: Add a pure helper in `web/src/lib/projectTrackers.ts` that returns `true` when the `ProjectSavePayload` includes `trackers !== undefined` or `label !== undefined` and the saved project (`id` and `slug`) is visible in the current scope:
   - the active saved board view (`selectedViewId`) includes the project in `currentBoardView.projectIds`; or
   - no saved board view is active and `selectedProjectId === 'all'`, `selectedProjectId === project.id`, or `selectedProjectId === project.slug`.
2. **Refresh tasks and facets in `updateProject`**: In `web/src/context/AppContext.tsx`, after `await fetchProjects()` in `updateProject`, if `projectSaveRefreshesBoard` is true, call `fetchTasks()` and `fetchTaskFacets()`.
3. **Multi-project support in `tasksInProject`**: Update `tasksInProject` in `web/src/lib/taskIdentity.ts` to read `task.projectIds?.length ? task.projectIds : task.projectId ? [task.projectId] : []` when matching a specific `projectId` or a `bookmarkedProjectIds` set.

## Target Files and Tests
- `web/src/lib/projectTrackers.ts`: add `projectSaveRefreshesBoard`.
- `web/src/lib/taskIdentity.ts`: check `task.projectIds` in `tasksInProject`.
- `web/src/context/AppContext.tsx`: call `fetchTasks()` and `fetchTaskFacets()` in `updateProject` when `projectSaveRefreshesBoard` holds.
- `web/tests/projectTrackers.test.mjs` and `web/tests/taskIdentity.test.mjs`: unit tests covering tracker/label saves across single-project, `'all'`, and saved-view scopes, sprint-only saves, and multi-project task filtering.
- `CHANGELOG.md`: add a `Fixed` entry under `## [Unreleased]`.

## Rejected Alternatives
- Unconditionally calling `fetchTasks()` and `fetchTaskFacets()` on every `updateProject` call: `SprintTimelineView` calls `updateProject(id, { sprints })`, which does not change ticket membership or facets.
- Relying on `tasks.length` effect to refresh facets: when a tracker or label change replaces N tickets with another N tickets, `tasks.length` does not change, so `fetchTaskFacets()` must be called explicitly alongside `fetchTasks()`.
