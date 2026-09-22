# #349 — Technical plan

Behaviour and acceptance criteria live in [`spec.md`](./spec.md). This file records the
implementation choices only.

## Stack and constraints

- Backend: Go. `internal/models` (project and request shapes), `internal/db` (schema,
  migration, task queries, creation), `internal/handlers` (HTTP API).
- Frontend: React + TypeScript under `web/src`; board state in `context/AppContext.tsx`,
  filter bar in `components/TaskFilters.tsx`, card menu in `components/TaskCard.tsx`,
  project settings in `components/ProjectModal.tsx`.
- **Two SQL backends.** `internal/db/dialect_sqlite.go` and `dialect_postgres.go` back the
  same statements, so the membership condition must be portable: no `json_each`, no
  SQLite-only case-folding assumption (Postgres `LIKE` is case-sensitive, SQLite's is not).
- No new HTTP route. `GET /api/tasks`, `GET /api/tasks/facets` and `PATCH /api/tasks/:id`
  already exist and are enough.
- No tracker adapter change. `JiraAdapter.labelOps` (`jira.go:391`) already emits `add` /
  `remove` operations, and `CreateIssue` already forwards the labels it is given.

## Architecture

```
ProjectModal (Général tab)        [A] new "Label du projet" input, whitespace refused
  │  POST/PATCH /api/projects
  ▼
models.Project.ProjectLabel        [B] new field + Create/UpdateProjectRequest
db: projects.project_label         [C] migration 3 + the SELECT/INSERT/UPDATE statements
  ▼
TaskFilters                        [D] new "Tout le board" toggle (shown when label set)
AppContext.buildTaskQuery          [E] membership=all when the toggle is on
  │  GET /api/tasks?membership=...
  │  GET /api/tasks/facets?membership=...
  ▼
handlers.handleTasks / facets      [F] parse membership
db.GetTasksForUser / FacetsForUser [G] extra AND term from membershipConditionUnsafe()
  ▼
TaskCard burger menu               [H] "Ajouter au projet" / "Retirer du projet"
  │  PATCH /api/tasks/:id {labels}
  ▼
db.UpdateTask                      (unchanged — already diffs and queues add/remove)

db.CreateTaskAs / ConvertTaskToRemote [I] stamp the project label before creating
```

## Decisions

### D1 — Storage, as a numbered migration (ADR 0021)

`models.Project` gains `ProjectLabel string \`json:"projectLabel"\``, next to `IssueTypes`
since both are "what this project holds", not connection parameters.
`CreateProjectRequest` gains `ProjectLabel string`, `UpdateProjectRequest` gains
`ProjectLabel *string` (the pointer convention of that struct: absent means "leave alone").

The column is **one numbered migration and nothing else**. ADR 0021 froze the baseline —
the `CREATE TABLE`, the legacy `ALTER` block and `lateColumns` all describe schema version 1
and are never edited again — and it is also the only form that reaches a PostgreSQL database
created before the column existed, since PostgreSQL does not run the legacy block at all:

```go
{
    version:    3, // 2 is tasks.creator, merged on main first (#358)
    name:       "projects.project_label",
    statements: []string{"ALTER TABLE projects ADD COLUMN project_label TEXT NOT NULL DEFAULT '';"},
},
```

`internal/db/db.go` then only names the column where it is read and written: the two project
SELECTs (l. 5710, 5848) with their scan targets, the INSERT (l. 6029) and the UPDATE
(l. 6254) with their arguments.

When this was written it was the repository's first numbered migration, which the test
harness had not met yet; `tasks.creator` (#358) has since taken version 2 on main.
`forgetSchemaVersion`, the helper eight test files use to make a current-schema database look
pre-versioning, deletes the version rows but leaves the columns the migrations added, so
the migration was replayed over a database that already carried it. The helper now also drops
the post-baseline columns, which is what "pre-versioning" actually means, and the two
synthetic migrations in `migrations_test.go` are numbered from `latestVersion()` rather than
from `baselineVersion` so they no longer collide with a real version. Hedging the migration with
`IF NOT EXISTS` was rejected: SQLite has no such form for `ADD COLUMN`, and idempotent-by-
failure is precisely what ADR 0021 replaces.

Rejected: a user-level fallback like `JiraProject` has. A label whose purpose is to separate
two Sectile projects sharing one tracker key is meaningless at user level — a global value
would apply the same slice to both. `IssueTypes` is the precedent: project-only.

### D2 — Validation in one place

`models.NormalizeProjectLabel(raw string) (string, error)` in `internal/models`: trims the
value, returns it unchanged when empty, and refuses it when `strings.ContainsFunc(v, unicode.IsSpace)`
after trimming. One function called by the create handler and the update handler, so the API
cannot be bypassed by the UI and vice versa; the UI repeats the check only to give immediate
feedback.

The error message is English, per the repository policy for Go strings:
`project label cannot contain whitespace`.

### D3 — The membership condition, built in Go, portable in SQL

New file `internal/db/projectlabel.go`.

```go
// membershipConditionUnsafe returns an extra WHERE term restricting the task list to
// the tickets that belong to their project, or "" when no project in scope has a
// label. Unsafe: the caller already holds d.mu.
func (d *DB) membershipConditionUnsafe(projectID, userID string) (string, []interface{})
```

It reads the labelled projects in scope with one query —
`SELECT id, slug, project_label FROM projects WHERE TRIM(project_label) <> ''`, narrowed by
`id = ?`/`slug = ?` when a single project is selected — and, for each, emits

```sql
(tasks.project_id NOT IN (?, ?) OR LOWER(tasks.labels) LIKE ? ESCAPE '\')
```

joined with `AND`, where the two placeholders are the project's id and slug and the pattern
is built in Go:

```go
encoded, _ := json.Marshal(strings.ToLower(label))
pattern := "%" + escapeLike(string(encoded)) + "%"
```

The label is JSON-encoded because that is how the `labels` column is written:
`encoding/json` escapes `&`, `<`, `>`, `"` and `\`, so `R&D` is stored as `"R\u0026D"`.
`escapeLike` prefixes `\`, `%` and `_` with `\`. Matching the label inside its JSON quotes
is what makes it a whole-label match: `%"team-alpha"%` does not match `["team-alphabet"]`.
`LOWER` on both sides makes it case-insensitive on Postgres too, where `LIKE` is not.

Returning `""` when nothing in scope carries a label is what keeps FR10 free: an unlabelled
deployment runs the exact query it runs today.

Rejected alternatives:
- `EXISTS (SELECT 1 FROM json_each(tasks.labels) ...)` — exact and pretty, SQLite-only.
- A correlated subquery joining `projects` per row — portable, but the LIKE pattern would
  then be built in SQL from the column, where the `%` and `_` of a stored label could not be
  escaped.
- Filtering in Go after the query — loses the facets and the counts, which the clarification
  ruled out explicitly.

### D4 — One parameter, two endpoints

`GetTasksForUser` and `GetTaskFacetsForUser` take a trailing `membershipAll bool`; when it
is false they append `membershipConditionUnsafe(...)` to their conditions. `GetTasks` and
`GetTaskFacets` (the no-user wrappers) pass `false`, so the default is "this project only"
everywhere, including the callers inside `internal/db` that use them for other purposes.

`internal/handlers/handlers.go` reads `membership` on both handlers:
`membershipAll := r.URL.Query().Get("membership") == "all"`. Any other value, including an
absent one, means `project`.

`GetTaskFacetsForUser` builds its facet queries from `scopeSQL`; the membership term is
appended to that same string so all nine facet queries inherit it without touching each one.

### D5 — Creation stamps the label once, at the top

`CreateTaskAs` (`db.go:2322`) already normalises the labels at one point:
`req.Labels = SetWorkflowLabel(req.Labels, "#new")`. The stamp goes immediately after —
`req.Labels = addProjectLabel(req.Labels, proj)` — so it reaches both the `CreateIssue` call
and the `labelsJSON` written locally, with no second insertion point to keep in step.

`ConvertTaskToRemote` (`db.go:5402`) — the other path that calls `CreateIssue`, named
`PushTaskToTracker` in the clarification, which no function carries — gets the same line
after its `SetWorkflowLabel` call, on `task.Labels`.

`addProjectLabel(labels []string, proj *models.Project) []string` lives in
`projectlabel.go` and is a no-op when the project is nil, has no label, or already carries
it (compared lower-cased, `#` prefix trimmed, like `HasPinnedLabel`).

### D6 — Frontend: one boolean, mirroring `pinnedOnly`

`AppContext`:
- `showAllTickets: boolean` + `setShowAllTickets`, `useState(false)`;
- `setShowAllTickets` calls `persistFilter({ showAllTickets: value ? '1' : null })`, so the
  absent key restores the "this project only" default (this is why the stored flag is the
  *widening* one, not the narrowing one);
- restored in the existing per-project effect: `setShowAllTicketsState(stored.showAllTickets === '1')`;
- `buildTaskQuery` appends `membership=all` when it is true, and it joins the dependency
  array;
- `fetchTaskFacets` appends the same parameter so the facets follow the list;
- exposed on the context value with `projectLabel` — the selected project's label, derived
  from `projects` — so `TaskFilters` and `TaskCard` do not each re-derive it.

`web/src/types/index.ts`: `projectLabel?: string` on `Project`, and the two context
entries on the `AppContextType` interface.

### D7 — Frontend: the toggle and the two menu entries

`TaskFilters.tsx`: a third toggle next to "Épinglés" and "En cours", rendered only when the
selected project has a label, using the same button markup and the `LayoutGrid` icon.
Label "Tout le board"; title explains what it widens.

`TaskCard.tsx`: in the common part of the menu (after "Copier la référence", before the
sprint entry), one entry driven by the card's project:

```tsx
const cardProjectLabel = projects.find(p => p.id === task.projectId)?.projectLabel?.trim() || ''
const carriesProjectLabel = cardProjectLabel !== '' && (task.labels || []).some(
  l => l.toLowerCase() === cardProjectLabel.toLowerCase())
```

"Ajouter au projet" calls `updateTask(task.id, { labels: [...(task.labels||[]), cardProjectLabel] })`;
"Retirer du projet" calls it with the label filtered out case-insensitively. Both close the
menu and show a toast, like the neighbouring entries. French copy, matching that block.

Note: `task.projectId` may hold the project's slug rather than its id (`projectScope` copes
with both), so the lookup falls back to matching the slug.

### D8 — ProjectModal

A "Label du projet" input in the left column of the `general` tab, under "Identifiant /
Slug" — it belongs with the project's identity, not with its tracker connection (answer 7 of
the clarification). `onChange` strips whitespace as it is typed (`value.replace(/\s+/g, '')`),
which is the immediate feedback and makes an invalid submit impossible from the UI; the hint
line under it says what an empty value means.

`projectLabel` joins the `useState` block, the `editingProject` hydration (l. ~254) and the
submitted payload (l. ~397).

## Tests

| What | Where | Why it is the right place |
| --- | --- | --- |
| `NormalizeProjectLabel`: trims, accepts empty, refuses inner whitespace | `internal/models/projectlabel_test.go` (new) | Pure function, no DB |
| Membership filter: labelled project returns only carriers; case-insensitive; no prefix match; unlabelled project returns everything; `membershipAll` returns everything | `internal/db/projectlabel_test.go` (new) | Existing `internal/db` tests open a real SQLite store; this is the behaviour of FR3–FR5 |
| Facets follow the membership scope | same file | FR6 |
| `CreateTaskAs` stamps the label locally and does not duplicate it | same file | FR9 |
| Column survives the migration on a base created without it | `internal/db/migrations_test.go` (extend) | NFR3, existing pattern |
| `membership=all` parsed from the query string | `internal/handlers` test if one covers `/api/tasks` today, otherwise covered by the db test + manual | Avoid inventing a handler test harness that does not exist |
| Toggle persistence and `membership=all` in the query string; menu entry visibility | `web/tests/projectLabel.test.mjs` (new, `node --test`) | Matches how `boardColumns.test.mjs` and `runStates.test.mjs` test pure helpers |

The web test suite tests extracted helpers rather than rendering React. The two testable
pieces are extracted for that reason: `membershipParam(showAll)` and
`projectLabelState(task, project)` in `web/src/lib/projectLabel.ts`, imported by the
component and asserted by the test.

## Risks

- **A project whose board is empty right after configuring a label.** Expected and recorded
  in the clarification; the toggle is the escape hatch and the UI hint says so.
- **Postgres `LIKE` case sensitivity.** Addressed by `LOWER()` on both sides; the Postgres
  test files in `internal/db` are gated on an available server, so this is verified by
  reading rather than by a run in CI — stated here rather than assumed.
- **`task.projectId` holding a slug.** Handled in both D3 (id *and* slug in the `NOT IN`)
  and D7 (lookup fallback).
