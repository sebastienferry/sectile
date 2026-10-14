# Design: Stop Taking the Database Write Lock on Every Task List Read for the Default Bookmark (#723)

## Context
Follow-up of `#486`. `EnsureDefaultBookmark(userID)` is called on every `all`-scope board read (`GetTasksInScope`, `GetTaskFacetsInScope`, `TaskSourcesInScope`) as well as `GetProjectsForUser` and `GetUserProjectBookmarks`. Taking `d.mu.Lock()` on every call forces all concurrent readers and writers to serialize behind every board poll.

## Decisions
1. **Per-DB Bookmark Cache (`userBookmarkCache`)**:
   - Mirror `projectURLCache` (`internal/db/projectcache.go`) with a 30 s TTL and a generation counter so fills started before a write are discarded.
   - Store non-empty bookmark slices (`[]string`) per `userID`. An empty slice is never cached as satisfied so a user with zero bookmarks still triggers default-project seeding.
   - Clear the cache synchronously on `BookmarkProject`, `UnbookmarkProject`, `ToggleProjectBookmark`, and `DeleteProject`.
2. **Read-Only Fast Path in `EnsureDefaultBookmark`**:
   - If `d.bookmarks.get(userID)` hits with `len(ids) > 0`, return `nil` immediately (no lock, 0 SQL statements).
   - Otherwise, acquire `d.mu.RLock()`, load `getUserProjectBookmarksUnsafe(userID)` (which populates `d.bookmarks` when `len(ids) > 0`), and release `d.mu.RLock()`. If `len(ids) > 0` or an error occurred, return without ever taking `d.mu.Lock()`.
   - Only when `len(ids) == 0` acquire `d.mu.Lock()` and run `ensureDefaultBookmarkUnsafe(userID)`.
3. **Reuse Cached Bookmarks in `bookmarkedProjectsUnsafe`**:
   - `bookmarkedProjectsUnsafe(userID)` consults `d.bookmarks` before querying `user_project_bookmarks`, so an `all`-scope `GetTasksInScope` for a user who already has a bookmark issues zero bookmark statements.
