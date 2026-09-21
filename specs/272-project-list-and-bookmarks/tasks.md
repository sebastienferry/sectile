# #272 — Implementation Checklist

Ordered checklist ensuring the backend schema and APIs are solid before connecting the web interface and updating the switcher UI.

---

## 1. Storage — User Project Bookmarks

- [x] **T1** Add the `user_project_bookmarks` table DDL and index to `initSchema` in `internal/db/db.go`.
- [x] **T2** Create `internal/db/bookmarks.go` with core storage methods:
  - `GetUserProjectBookmarks(userID string) ([]string, error)`
  - `BookmarkProject(userID, projectID string) error`
  - `UnbookmarkProject(userID, projectID string) error`
  - `ToggleProjectBookmark(userID, projectID string) (bool, error)`
  - `EnsureDefaultBookmark(userID string) error` (seeds default project if user has 0 bookmarks)
- [x] **T3** Enrich `GetProjectsForUser(userID string) ([]models.Project, error)` in `internal/db/db.go` to set `Bookmarked` on each project for `userID`.
- [x] **T4** In `DeleteProject(id string)` (`internal/db/db.go`), clean up rows from `user_project_bookmarks WHERE project_id = ?`.
- [x] **T5** Create `internal/db/bookmarks_test.go`:
  - Test default bookmark seeding on first access.
  - Test bookmarking and unbookmarking isolation between two users.
  - Test toggle functionality.
  - Test deletion cascade when a project is deleted.

---

## 2. Server — API Endpoints & Scoping

- [x] **T6** Add `Bookmarked bool \`json:"bookmarked"\`` to `models.Project` in `internal/models/models.go`.
- [x] **T7** Update `HandleProjects` (`internal/handlers/handlers.go`):
  - In `GET`: Call `h.db.GetProjectsForUser(h.webSessionUser(r))` to return projects with user bookmark flags.
  - In `POST`: After creating a project, bookmark it automatically for `h.webSessionUser(r)`.
- [x] **T8** Create `internal/handlers/userbookmarks.go` for `/api/me/project-bookmarks`:
  - `GET /api/me/project-bookmarks`: Return list of bookmarked project IDs.
  - `PUT /api/me/project-bookmarks/{id}`: Bookmark a project.
  - `DELETE /api/me/project-bookmarks/{id}`: Unbookmark a project.
  - `POST /api/me/project-bookmarks/{id}/toggle`: Toggle bookmark.
- [x] **T9** Register routes in `internal/handlers/auth.go` / `handlers.go` and ensure `requireSession` protects `/api/me/project-bookmarks`.
- [x] **T10** Update `HandleTasks` and `HandleTaskFacets` in `internal/handlers/handlers.go`:
  - When `projectId` is empty or `"all"`, scope query results to projects bookmarked by `h.webSessionUser(r)`.
- [x] **T11** Create `internal/handlers/userbookmarks_test.go`:
  - Test `GET`, `PUT`, `DELETE`, and `POST /toggle` on `/api/me/project-bookmarks`.
  - Verify that unauthenticated requests receive `401 Unauthorized`.
  - Verify that user A's bookmarks do not affect user B.

---

## 3. Web Interface — Context & State Management

- [x] **T12** Update `Project` interface in `web/src/types/index.ts` to include `bookmarked?: boolean`.
- [x] **T13** In `web/src/context/AppContext.tsx`:
  - Add `toggleProjectBookmark: (projectId: string) => Promise<boolean>`.
  - Implement optimistic UI updates when toggling bookmarks.
  - Ensure `fetchProjects` populates `bookmarked` and handles selection fallback if an active project is unbookmarked.
- [x] **T14** In `web/src/lib/taskIdentity.ts`:
  - Update `tasksInProject` or task filtering to account for bookmarked projects when `selectedProjectId === 'all'`.

---

## 4. Web Interface — Sidebar Switcher & Search

- [x] **T15** In `web/src/components/Sidebar.tsx`:
  - Add a search input inside the project dropdown menu (`Rechercher un projet...`).
  - When search is empty: filter displayed projects to `p.bookmarked === true`.
  - When search is non-empty: filter across all projects matching the search text.
  - Add a star / bookmark toggle icon button (`Star` from `lucide-react`) on each project row.
  - Display filled star for bookmarked projects and outline star for unbookmarked projects.
  - Connect click handler to `toggleProjectBookmark` with `e.stopPropagation()`.

---

## 5. Web Interface — Modals (QuickAdd, TaskDetail, Clone)

- [x] **T16** In `web/src/components/QuickAddModal.tsx`:
  - Group projects in the select dropdown with bookmarked projects first (`<optgroup label="Favoris">`), followed by remaining projects (`<optgroup label="Autres projets">`).
- [x] **T17** In `web/src/components/TaskDetailModal.tsx`:
  - Group projects in the reassignment select dropdown with bookmarked projects first.
- [x] **T18** In `web/src/components/CloneTaskModal.tsx`:
  - Group projects in the destination project select dropdown with bookmarked projects first.

---

## 6. Gates & Verification

- [x] **T19** Run Go build and tests:
  ```bash
  go build ./... && go test -race ./internal/db/... ./internal/handlers/...
  ```
- [x] **T20** Run frontend checks in `web/`:
  ```bash
  npm run lint && npm run build && npm test
  ```
- [x] **T21** Verify acceptance criteria against `spec.md` (US1 to US9).
