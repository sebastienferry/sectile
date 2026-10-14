# Proposal: Stop Taking the Database Write Lock on Every Task List Read for the Default Bookmark (#723)

## Why
`GetTasksInScope`, `GetTaskFacetsInScope`, `GetProjectsForUser`, `GetUserProjectBookmarks`, and `TaskSourcesInScope` call `EnsureDefaultBookmark(userID)` before their read. `EnsureDefaultBookmark` unconditionally takes `d.mu.Lock()` (the exclusive write lock) and runs `SELECT COUNT(*) FROM user_project_bookmarks WHERE user_id = ?`, even when the user already has bookmarks. Every `all`-scope `GET /api/tasks` therefore serializes with all other reads and writes on the instance and issues extra bookmark queries.

## What Changes
- Add a per-DB user bookmark cache (`userBookmarkCache`, 30 s TTL, cleared synchronously on bookmark and project delete writes) and read-locked fast path in `EnsureDefaultBookmark(userID)`:
  - When the user already has cached bookmarks (`len(ids) > 0`), `EnsureDefaultBookmark` returns immediately without taking any lock or issuing any SQL statement, and `bookmarkedProjectsUnsafe` / `getUserProjectBookmarksUnsafe` return the cached bookmark list without querying `user_project_bookmarks`.
  - When the cache is cold, `EnsureDefaultBookmark` checks under `d.mu.RLock()` first; if the user already has bookmarks in `user_project_bookmarks`, it caches them and returns without taking `d.mu.Lock()`.
  - Only when the user has zero bookmarks in `user_project_bookmarks` does `EnsureDefaultBookmark` acquire `d.mu.Lock()` and seed the deployment default project.
- Add tests in `internal/db/bookmarks_test.go` verifying that an `all`-scope `GetTasksInScope` for a user who already has a bookmark does not take the write lock and issues no bookmark statement, while a user with no bookmark still gets the default project bookmarked.

## Impact
- `internal/db/bookmarks.go`
- `internal/db/membership.go`
- `internal/db/db.go`
- `internal/db/bookmarks_test.go`
