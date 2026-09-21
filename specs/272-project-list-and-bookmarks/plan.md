# #272 — Implementation Plan

## Stack

- **Backend**: Go 1.x, SQLite (via `database/sql` in `internal/db`), HTTP handlers (`internal/handlers`), JSON models (`internal/models`).
- **Frontend**: React 18, TypeScript, Tailwind CSS, Lucide React icons (`web/src`).
- **Desktop**: Electron interface consumes the standard web application and REST API without modifications.

---

## Architecture Decisions

### D1 — Dedicated `user_project_bookmarks` table

User project bookmarks are personal relationships between a user (`users.id`) and a project (`projects.id`). Rather than storing an opaque JSON array inside `user_settings`, a dedicated relational join table is used:

```sql
CREATE TABLE IF NOT EXISTS user_project_bookmarks (
    user_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, project_id)
);
CREATE INDEX IF NOT EXISTS idx_user_project_bookmarks_user ON user_project_bookmarks (user_id);
```

**Rationale**:
- Efficient joins and existence checks for project queries.
- Clean cascade cleanup on project deletion (`DeleteProject`) and user deletion (`DeleteUser`).
- Does not bloat the `user_settings` row or cause race conditions when saving profile preferences concurrently.

### D2 — Enriched `GET /api/projects` and `/api/me/project-bookmarks` Endpoints

1. **Enriched `GET /api/projects`**:
   - The handler inspects `webSessionUser(r)`.
   - Each project returned includes a `bookmarked: bool` field indicating whether the calling user has bookmarked it.
   - This eliminates extra network round-trips during app bootstrap.
2. **Dedicated Bookmark Endpoints**:
   - `GET /api/me/project-bookmarks`: returns an array of bookmarked project IDs `string[]` for the caller.
   - `PUT /api/me/project-bookmarks/{projectId}`: bookmarks the specified project for the caller.
   - `DELETE /api/me/project-bookmarks/{projectId}`: removes the bookmark for the caller.
   - `POST /api/me/project-bookmarks/{projectId}/toggle`: convenience endpoint that toggles bookmark status and returns `{"bookmarked": bool}`.
   - Handled in `internal/handlers/userbookmarks.go`, guarded by `requireSession`.

### D3 — Automated Seeding and Creation Hook

1. **Lazy Migration / Seeding**:
   - When retrieving projects or bookmarks for an authenticated user with zero recorded bookmarks:
     - The system queries for the default project (`is_default = 1`, falling back to the oldest project if none is marked default).
     - It inserts a bookmark record into `user_project_bookmarks` for `(user_id, default_project_id)`.
     - The default project is returned with `bookmarked: true`.
2. **Project Creation Hook**:
   - When `HandleProjects` processes `POST /api/projects`, it retrieves the acting user ID via `h.webSessionUser(r)`.
   - Upon successful creation, the new project ID is immediately inserted into `user_project_bookmarks` for that user.

### D4 — Scoping "All Projects" View and Facets to Bookmarked Projects

When `selectedProjectId === 'all'` (or empty `projectId`):
1. **`GET /api/tasks`**:
   - In `HandleTasks`, if `projectId == ""` or `projectId == "all"`, and the caller is an authenticated user:
   - The query filters `tasks WHERE project_id IN (SELECT project_id FROM user_project_bookmarks WHERE user_id = ?)`.
   - If the user has no bookmarks, the default project fallback applies.
2. **`GET /api/task-facets`**:
   - In `HandleTaskFacets`, when `projectId == ""` or `projectId == "all"`, facets are scoped to the caller's bookmarked projects, matching the task list behavior.
3. **Frontend Defense-in-Depth**:
   - `AppContext.tsx` and `taskIdentity.ts`: `tasksInProject` also filters tasks against the list of bookmarked project IDs when `selectedProjectId === 'all'`.

### D5 — Sidebar Project Dropdown UX

In `web/src/components/Sidebar.tsx`:
1. **Search Field**:
   - An input at the top of the project dropdown with placeholder `Rechercher un projet...` (or localized).
2. **List Filtering**:
   - When search is empty: displays only projects where `p.bookmarked === true` (plus the "Tous les projets" / "All projects" item).
   - When search is non-empty: filters across **all** projects by name, description, or slug.
3. **Bookmark Toggle Button**:
   - Each project row includes a star icon (`Star` from `lucide-react`):
     - Bookmarked: filled yellow/accent star `fill-current text-amber-400`.
     - Unbookmarked: outline star `text-slate-400 hover:text-amber-400`.
   - Clicking the star calls the bookmark toggle API, updates the project in `AppContext` state optimistically, and refreshes the switcher.

### D6 — Modal Project Selectors (QuickAdd, TaskDetail, Clone)

In `QuickAddModal.tsx`, `TaskDetailModal.tsx`, and `CloneTaskModal.tsx`:
- The `<select>` element groups projects using `<optgroup>`:
  - `<optgroup label="Favoris / Bookmarks">`: projects where `bookmarked === true`.
  - `<optgroup label="Autres projets / Other projects">`: remaining projects.
- If no bookmarks exist or all are bookmarked, all projects render in standard order.

---

## Data Contracts

### 1. Database Schema

```sql
-- internal/db/db.go
CREATE TABLE IF NOT EXISTS user_project_bookmarks (
    user_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, project_id)
);
CREATE INDEX IF NOT EXISTS idx_user_project_bookmarks_user ON user_project_bookmarks (user_id);
```

### 2. Go Models (`internal/models/models.go`)

```go
type Project struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Slug        string `json:"slug"`
    // ... existing fields ...
    Bookmarked  bool   `json:"bookmarked"`
}
```

### 3. REST API Endpoints

| Method | Path | Description | Access |
|---|---|---|---|
| `GET` | `/api/projects` | Enriched project list with `bookmarked` field for caller | Session |
| `GET` | `/api/me/project-bookmarks` | Array of bookmarked project IDs `string[]` | Session |
| `PUT` | `/api/me/project-bookmarks/{id}` | Add project `{id}` to caller's bookmarks | Session |
| `DELETE` | `/api/me/project-bookmarks/{id}` | Remove project `{id}` from caller's bookmarks | Session |
| `POST` | `/api/me/project-bookmarks/{id}/toggle` | Toggle bookmark status, returns `{"bookmarked": bool}` | Session |

### 4. TypeScript Models (`web/src/types/index.ts`)

```typescript
export interface Project {
  id: string
  name: string
  slug: string
  // ... existing fields ...
  bookmarked?: boolean
}
```

---

## Target Files

- `internal/db/db.go`: DDL for `user_project_bookmarks`, hook in `DeleteProject`.
- `internal/db/bookmarks.go` (new): `GetUserProjectBookmarks`, `BookmarkProject`, `UnbookmarkProject`, `ToggleProjectBookmark`, `EnsureInitialBookmark`.
- `internal/db/bookmarks_test.go` (new): Unit tests for bookmark storage, seeding, and cascade deletion.
- `internal/db/tasks.go` / `internal/db/db.go`: Scope `GetTasks` and `GetTaskFacets` to user's bookmarked projects when `projectID` is empty.
- `internal/models/models.go`: Add `Bookmarked bool` to `Project`.
- `internal/handlers/handlers.go`: Update `HandleProjects` to pass user context, hook `CreateProject` to bookmark for creator.
- `internal/handlers/userbookmarks.go` (new): Handler for `/api/me/project-bookmarks` routes.
- `internal/handlers/userbookmarks_test.go` (new): Integration tests for bookmark API endpoints.
- `web/src/types/index.ts`: Add `bookmarked?: boolean` to `Project`.
- `web/src/context/AppContext.tsx`: Add `toggleProjectBookmark`, optimize project lists and "all" task queries.
- `web/src/components/Sidebar.tsx`: Project dropdown search input, bookmark filtering, star toggle icons.
- `web/src/components/QuickAddModal.tsx`: Group projects with bookmarked first.
- `web/src/components/TaskDetailModal.tsx`: Group projects with bookmarked first.
- `web/src/components/CloneTaskModal.tsx`: Group projects with bookmarked first.
