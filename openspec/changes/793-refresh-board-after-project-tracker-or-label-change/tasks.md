## Implementation Checklist
- [x] 1. Add `projectSaveRefreshesBoard` to `web/src/lib/projectTrackers.ts` and unit tests in `web/tests/projectTrackers.test.mjs`.
- [x] 2. Update `tasksInProject` in `web/src/lib/taskIdentity.ts` to check `task.projectIds` (falling back to `task.projectId`) and add unit tests in `web/tests/taskIdentity.test.mjs`.
- [x] 3. Update `updateProject` in `web/src/context/AppContext.tsx` to call `fetchTasks()` and `fetchTaskFacets()` after `fetchProjects()` when `projectSaveRefreshesBoard` returns `true`.
- [x] 4. Add a `Fixed` entry under `## [Unreleased]` in `CHANGELOG.md` and run web unit tests and build.

## Specification Validation
Run `openspec validate 793-refresh-board-after-project-tracker-or-label-change --strict`.
