# Tasks: Stop Taking the Database Write Lock on Every Task List Read for the Default Bookmark (#723)

- [x] 1. Add `userBookmarkCache` and update `EnsureDefaultBookmark`, `getUserProjectBookmarksUnsafe`, `BookmarkProject`, `UnbookmarkProject`, and `ToggleProjectBookmark` in `internal/db/bookmarks.go`.
- [x] 2. Clear `d.bookmarks` on project deletion in `internal/db/db.go` and consult `d.bookmarks` in `bookmarkedProjectsUnsafe` (`internal/db/membership.go`).
- [x] 3. Add a test in `internal/db/bookmarks_test.go` proving that an `all`-scope `GetTasksInScope` for a user who already has a bookmark does not take the write lock and issues no bookmark statement.
