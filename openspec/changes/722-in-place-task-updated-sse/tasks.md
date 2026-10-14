# Tasks: Stop Every Browser Reloading Three Full Lists on Each `task_updated` Event (#722)

- [x] 1. Hydrate `task` and `activity` via `GetTaskByID` and `GetActivityByID` in `NewHandler`'s post-back listener (`internal/handlers/handlers.go`) and cover in `internal/handlers/postback_test.go`.
- [x] 2. Create `web/src/lib/taskUpdatedEvent.ts` with `taskMatchesBoardScope`, `applyUpdatedTask`, `activityMatchesProjectScope`, `applyUpdatedActivity`, and `createCoalescedRefetch`.
- [x] 3. Update `web/src/context/AppContext.tsx` to apply `task_updated` SSE events in place without issuing three full list reloads per browser.
- [x] 4. Add unit tests in `web/tests/taskUpdatedEvent.test.mjs` and update `CHANGELOG.md`.
