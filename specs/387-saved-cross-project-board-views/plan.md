# Implementation plan: saved cross-project board views

Behaviour: [spec.md](spec.md). Checklist: [tasks.md](tasks.md).

## Stack and scope

- Go server (`internal/db`, `internal/handlers`, `cmd/server/main.go`), SQLite
  and PostgreSQL through the numbered migrations of ADR 0021.
- React + TypeScript web interface (`web/src`), shared by the desktop shell.
- No tracker, synchronisation, agent, MCP or workflow change. No new dependency.

## Architecture

A saved view is an **overlay on the existing all-projects code path**, not a new
board. The task list and facet endpoints gain one optional parameter, `viewId`.
When it is present, the server replaces the bookmark-based all-projects scope
with the view's project list and adds the view's label predicate; every other
filter keeps working unchanged on top. The web interface treats an open view as
"all projects" for columns, grouping, card actions and detail, and only changes
what it sends to the server, how it keys remembered filters, and what the
sidebar, header and quick-add show.

Resolving the view server-side (instead of sending `projectId[]` and `labels[]`
from the browser) keeps one source of truth, lets `?view=<id>` work from a cold
load, and enforces ownership in one place.

## Data model

Migration `version: 3`, `name: "board_views"` in `internal/db/migrations.go`
(the next free version; recheck against `origin/main` before merging). Same
statements on both engines:

```sql
CREATE TABLE IF NOT EXISTS board_views (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    name        TEXT NOT NULL,
    name_key    TEXT NOT NULL,          -- lower(trim(name)), uniqueness key
    project_ids TEXT NOT NULL DEFAULT '[]',  -- JSON array of project ids
    labels      TEXT NOT NULL DEFAULT '[]',  -- JSON array, normalized
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_board_views_user_name ON board_views (user_id, name_key);
CREATE INDEX IF NOT EXISTS idx_board_views_user ON board_views (user_id);
```

JSON arrays in TEXT follow the precedent of `tasks.labels` and
`projects.enabled_views` (ADR 0024). A join table for projects was considered
and rejected: the list is read whole, never queried by project except on project
deletion, which is rare and can rewrite the few affected rows.

`project_ids` stores canonical project ids (`projects.id`), resolved from a slug
on write the way `ToggleProjectBookmark` does.

Go model in `internal/models/models.go`:

```go
type BoardView struct {
    ID         string    `json:"id"`
    Name       string    `json:"name"`
    ProjectIDs []string  `json:"projectIds"`
    Labels     []string  `json:"labels"`
    CreatedAt  time.Time `json:"createdAt"`
    UpdatedAt  time.Time `json:"updatedAt"`
}
```

`user_id` is not serialized: the owner is always the caller.

## Persistence layer (`internal/db/boardviews.go`, new)

- `ListBoardViews(userID) ([]models.BoardView, error)`: `ORDER BY created_at ASC`.
- `GetBoardView(userID, id) (*models.BoardView, error)`: `WHERE id = ? AND
  user_id = ?`; returns a sentinel `ErrBoardViewNotFound` for a missing or
  foreign row, never a distinct error (FR-005, FR-043).
- `CreateBoardView(userID, name, projectIDs, labels)` and
  `UpdateBoardView(userID, id, patch)`: validate and normalize, then write.
  - name: trimmed, required; `name_key = strings.ToLower(name)`; the unique index
    violation maps to `ErrBoardViewNameTaken` (check first for a clear message,
    keep the index as the guarantee).
  - projects: at least one; each must resolve to an existing project, else
    `ErrBoardViewUnknownProject`; duplicates dropped, order kept.
  - labels: `normalizeViewLabels` trims, drops empties, deduplicates on
    `strings.ToLower`, keeping the first spelling (FR-004).
- `DeleteBoardView(userID, id)`: owner-scoped delete; not found maps to the
  sentinel.
- `DeleteProject` (`internal/db/db.go`): after removing the project, rewrite
  every `board_views` row whose `project_ids` contains its id or slug to drop it
  (FR-052). Done in Go (select candidate rows with `project_ids LIKE '%"<id>"%'`,
  unmarshal, filter, update), which is engine-agnostic and exact after the
  unmarshal. Reading also ignores ids that no longer resolve, so a partial
  failure never shows a ghost project.

## Task selection

`GetTasksForUser` and `GetTaskFacetsForUser` gain a `viewID` input. To avoid a
fifteenth positional argument, introduce a small struct for the new call site
and keep the existing signatures as thin wrappers:

```go
type TaskScope struct {
    UserID    string
    ProjectID string
    ViewID    string
}
```

When `ViewID` is set:

1. Load the view with `GetBoardView(UserID, ViewID)`; not found returns
   `ErrBoardViewNotFound` (handler: 404).
2. Scope: `project_id IN (?, ...)` over the view's ids **and** their slugs
   (same dual form as `projectScope`, since `tasks.project_id` may hold either).
   An empty project list yields an empty result without querying.
3. `projectId`, if also sent, is ignored in favour of the view.
4. Labels: when the view has labels, filter the scanned rows in Go:
   keep a task when any `strings.ToLower(taskLabel)` is in the set of lowered
   view labels. The rows are already unmarshalled by the existing scan loop, and
   the list has no SQL pagination, so filtering after the scan changes no
   ordering or paging behaviour.
5. Every other filter (`q`, `label`, `sprint`, ... ) is applied as today.
6. Facets use the same scope and the same label predicate, so the counters and
   dropdowns describe the view, not the whole board.

Rejected for step 4:

- `labels LIKE '%x%'`, as the existing `label` filter does: substring semantics,
  which FR-011 excludes (`backend` would match `backend-api`).
- An SQL JSON predicate (`json_each` on SQLite, `jsonb_array_elements_text` on
  PostgreSQL): exact, but two dialect variants plus a cast of a TEXT column, for
  no measurable gain at board sizes. Revisit only if task lists become paginated
  in SQL.

## HTTP API

New handler file `internal/handlers/boardviews.go`, registered in
`cmd/server/main.go` next to the project bookmarks, behind the session guard:

| Method | Path | Body | Success | Errors |
| --- | --- | --- | --- | --- |
| GET | `/api/me/board-views` | none | `200 BoardView[]` | 401 |
| POST | `/api/me/board-views` | `{name, projectIds, labels}` | `201 BoardView` | 400 validation, 409 name taken |
| GET | `/api/me/board-views/{id}` | none | `200 BoardView` | 404 |
| PATCH | `/api/me/board-views/{id}` | any of `{name, projectIds, labels}` | `200 BoardView` | 400, 404, 409 |
| DELETE | `/api/me/board-views/{id}` | none | `204` | 404 |

- `GET /api/tasks?viewId=<id>` and `GET /api/tasks/facets?viewId=<id>`: 404 when
  the view is not the caller's. Other parameters unchanged.
- Error messages follow the surface language of the existing handlers (French
  for what the interface displays).
- A foreign id and a missing id return the same 404 body.

Document the endpoints and the `viewId` parameter in
`docs/API_AND_DATA_SPEC.md`, and the table in its data section.

## Web interface

### Types and API client

- `web/src/types/index.ts`: `BoardView` mirroring the Go model.
- Fetch helpers alongside the existing bookmark calls in
  `web/src/context/AppContext.tsx` (list, create, update, delete).

### State (`AppContext.tsx`)

- `boardViews: BoardView[]`, loaded after sign-in with the projects.
- `selectedViewId: string | null`. Opening a view sets it and sets
  `selectedProjectId` to `'all'`, so every component that branches on
  `'all'` keeps its current behaviour (FR-020, FR-021). Selecting a project or
  the all-projects entry clears it (FR-041).
- `buildTaskQuery`, facet fetch: send `viewId` instead of `projectId` when a view
  is open.
- Remembered filters: `filterStorageKey` becomes scope-aware,
  `sectile_filters_view_<id>` for a view, unchanged for projects. The existing
  "write in the setter, not in an effect" rule is kept, and switching scope
  reloads the filters of the destination key (FR-031, FR-032).
- URL: on open, `history.replaceState` sets `?view=<id>` (preserving other
  parameters such as `task`); on leaving, removes it. On startup, a `view`
  parameter takes precedence over the stored project selection; if the view is
  not in `boardViews` (or the task request answers 404), clear the parameter,
  fall back to the stored selection and show a toast "Vue indisponible"
  (FR-042, FR-043). Persist the last open view like the selected project so a
  reload without the parameter also restores it.
- On a 404 for `viewId` during a refresh (view deleted elsewhere), fall back to
  the all-projects board.

### Sidebar (`web/src/components/Sidebar.tsx`)

- In the existing `views` section ("Vues"), below the built-in entries, list
  `boardViews` by name, the open one highlighted, each with an edit affordance;
  then a "Nouvelle vue" action (FR-040). Collapsed sidebar: icon only, name in
  the tooltip, as the other entries.
- Opening a view keeps the active board/backlog mode.

### Create / edit dialog (`web/src/components/BoardViewModal.tsx`, new)

- Fields: name, project multi-select (all projects, not only bookmarks), label
  chips with free entry and suggestions from the facet labels of the selected
  projects. Delete button with confirmation when editing (FR-051).
- Client-side validation mirrors the server's (FR-002, FR-001), the server
  remains authoritative and its 409/400 messages are shown as returned.

### Board and cards

- Header (`Header.tsx`): shows the view name when one is open (FR-024).
- `TaskCard.tsx`: when a view is open, render a compact project badge (project
  icon and name, from `taskProject`, already resolved on the card) (FR-023).
- Empty view (no project left, FR-052): the board shows an explanatory empty
  state with an "Edit view" action.

### Quick add (`web/src/components/QuickAddModal.tsx`)

- When a view is open: the project select lists only the view's projects and
  starts **unselected**, submission requires a choice (FR-060). When the view
  has exactly one label, prefill it in `labels` (FR-061); otherwise leave labels
  empty (FR-062).

### Translations

New strings go through `web/src/locales/translations.ts`, French as the rest of
the interface.

## Documentation and records

- `CHANGELOG.md`: one `Added` line under `[Unreleased]`.
- `docs/API_AND_DATA_SPEC.md`: endpoints, `viewId`, table.
- `docs/UX_COMPONENTS.md`: the sidebar entries and the dialog, if the file lists
  sidebar components.
- `README.md`: add the feature to the feature list if one exists.
- ADR `docs/adrs/0025-saved-board-views-are-personal-overlays.md`: records why a
  view is a personal selection over the existing board rather than a project
  membership label (#354) or a merged cross-project board, and why duplicates
  are shown rather than collapsed.

## Target files

| File | Change |
| --- | --- |
| `internal/db/migrations.go` | migration 3 `board_views` |
| `internal/db/boardviews.go` (new) | CRUD, normalization, sentinels |
| `internal/db/db.go` | `TaskScope`, view scope and label predicate in task list and facets; project deletion cleanup |
| `internal/models/models.go` | `BoardView` |
| `internal/handlers/boardviews.go` (new) | CRUD endpoints |
| `internal/handlers/handlers.go` | `viewId` on `/api/tasks` and `/api/tasks/facets` |
| `cmd/server/main.go` | route registration |
| `web/src/types/index.ts` | `BoardView` |
| `web/src/context/AppContext.tsx` | state, queries, filter keys, URL |
| `web/src/components/Sidebar.tsx` | view entries, create action |
| `web/src/components/BoardViewModal.tsx` (new) | create/edit/delete dialog |
| `web/src/components/Header.tsx` | view name |
| `web/src/components/TaskCard.tsx` | project badge in a view |
| `web/src/components/QuickAddModal.tsx` | source project and label rules |
| `web/src/locales/translations.ts` | strings |
| docs listed above | documentation |

## Risks

- **Filter key migration**: none needed; view keys are new and project keys are
  unchanged.
- **Positional arguments**: `GetTasksForUser` already takes fourteen; the
  `TaskScope` wrapper keeps callers (`agent_desktop`, MCP, handlers) untouched.
- **Migration number collision** with a concurrent branch: recheck the last
  version on `origin/main` right before pushing.
- **PostgreSQL**: the migration and queries use only portable SQL; the suite
  must run against a throwaway PostgreSQL (never the dev database).
