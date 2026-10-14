## ADDED Requirements

### Requirement: Default Bookmark Read-Only Fast Path
The database layer SHALL NOT acquire the exclusive write lock (`d.mu.Lock()`) or issue bookmark statements during an `all`-scope `GetTasksInScope` read when the user already has at least one project bookmark, while still seeding the deployment default project bookmark when a user has zero bookmarks.

#### Scenario: User who already has a bookmark reads all-scope tasks without taking the write lock or issuing bookmark statements
- **GIVEN** a user who already has a project bookmarked
- **WHEN** `GetTasksInScope` is called for that user with `ProjectID: "all"` while a read lock is held on the database
- **THEN** `GetTasksInScope` completes without taking the exclusive write lock and issues zero statements against `user_project_bookmarks`

#### Scenario: User with zero bookmarks gets the deployment default project bookmarked
- **GIVEN** a user with zero project bookmarks
- **WHEN** `EnsureDefaultBookmark`, `GetUserProjectBookmarks`, or an `all`-scope `GetTasksInScope` is called for that user
- **THEN** the deployment default project is bookmarked for that user
