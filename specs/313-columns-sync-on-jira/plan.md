# #313 — Technical plan

Behaviour and acceptance criteria live in [`spec.md`](./spec.md). This file records the
implementation choices only.

## Stack and constraints

- Backend: Go, `internal/tracker` (capability interface), `internal/trackerapi` (adapters),
  `internal/db` (board logic), `internal/handlers` (HTTP API).
- Frontend: React + TypeScript under `web/src`, state in `context/AppContext.tsx`.
- No database migration. No new HTTP route: `/boards`, `/board-columns`,
  `/tracker-statuses` and `/detected-statuses` all already exist.
- No change to `JiraAdapter`: `ListBoards`, `ListBoardColumns` and `ListStatuses` are
  already correct and covered by `internal/trackerapi/jira_test.go`.

## Architecture of the fix

The chain is already complete from the adapter up to the HTTP layer; three links are
missing or mis-wired.

```
UI (ProjectModal / BoardColumnsEditor)
  │  [A] no board picker  ─────────────► add one
  │  [B] one column per status ────────► mirror board columns
  ▼
HTTP /api/projects/{id}/boards            (ok)
     /api/projects/{id}/board-columns     (ok)
     /api/projects/{id}/tracker-statuses  (ok, but see [C])
     /api/projects/detected-statuses      [D] GitHub-only
  ▼
db.ListProjectTrackerBoardsAs             (ok, CapBoard dispatch)
db.ImportProjectBoardColumns              (ok)
db.SyncProjectBoardColumns                (ok, merge rules reused as-is)
db.GetProjectTrackerStatuses              [C] `if IssueTracker == "github"`
db.afterTrackerSync                       [E] skipped when BoardID == ""
  ▼
tracker.TicketingSystem (CapBoard) → JiraAdapter (ok)
```

## Decisions

### D1 — Capability dispatch in `GetProjectTrackerStatuses`

`internal/db/board.go:117`. Keep the GitHub ProjectsV2 block exactly as it is, behind
`if proj.IssueTracker == "github"`, including the `open` / `closed` fallback. Add an
`else` arm that resolves the tracker with `trackerReaderFor`, checks
`ts.Supports(tracker.CapBoard)`, and calls `ts.ListStatuses(ctx, tracker.ProjectRequest{Project: proj})`
under `boardAPITimeout`, returning the status names de-duplicated case-insensitively in
the tracker's order. A tracker without `CapBoard` returns an empty list and no error, as
today. Rejected alternative: a `case "jira"` branch — the clarification settled on
capability dispatch so GitLab inherits the path.

### D2 — Column-shaped detection

`/api/projects/detected-statuses` (`internal/handlers/handlers.go:520`) keeps its current
`{"statuses": [{id, name}]}` payload for the GitHub and draft-project paths, and gains a
`columns` field for a saved project whose tracker has `CapBoard`:

```json
{ "statuses": [{"id": "st-0", "name": "To Do"}],
  "columns":  [{"name": "To Do", "statuses": ["To Do", "Backlog"]}] }
```

`columns` is produced by a new `db.DetectProjectBoardColumns(ctx, projectID)` that
resolves the board (project `BoardID`, else first scrum board, else first board) and calls
`ts.ListBoardColumns`; `statuses` in that case is the flattened union of the palette so
existing consumers keep working. Rejected alternative: a new endpoint — the clarification
explicitly keeps the API surface as it is.

### D3 — The editor mirrors columns instead of inventing them

`web/src/components/BoardColumnsEditor.tsx:91-109`. When the response carries a non-empty
`columns` array, build the column list from it — one entry per board column, with its
statuses, in board order — and merge against the current columns with the same intent as
the backend merge (match by lower-cased name; keep the hidden flag; keep hand-assigned
statuses the board claims nowhere; append unknown user columns at the end). When
`columns` is absent, keep the current one-column-per-status code untouched, which is the
GitHub draft path.

The palette (`statuses` state) continues to come from `fetchProjectTrackerStatuses` for a
saved project — with D1 it is now populated on Jira — and from the detection response for
a draft project.

### D4 — Board picker in the project settings

A `BoardPicker` block rendered at the top of `BoardColumnsEditor`, so it sits inside the
existing board section of `ProjectModal.tsx:1361` and needs no new settings category. It
is shown only when `project?.id` exists and the board list comes back non-empty (a tracker
without `CapBoard` returns an `Unsupported` error and the picker stays hidden).

- On mount for a saved project: `listProjectBoards(project.id)`.
- Selection value: `project.boardId` when set, else the first board whose `type` is
  `scrum` (case-insensitive), else the first board.
- On change: `importProjectBoardColumns(project.id, boardId)`, then feed the returned
  project's `trackerColumns` and `stageColumns` into `onColumnsChange` /
  `onStageColumnsChange`, and refresh the palette.

Both context helpers already exist (`AppContext.tsx:2048` and `:2062`) and already surface
their errors as toasts; no new context API is added.

### D5 — Automatic sync for every `CapBoard` tracker

`internal/db/db.go`, `afterTrackerSync`: drop the `strings.TrimSpace(proj.BoardID) != ""`
condition and run the board step whenever `ts.Supports(tracker.CapBoard)`.
`SyncProjectBoardColumns` already resolves the first scrum board when `BoardID` is empty;
it is extended to persist the board it resolved (`UpdateProject` with `BoardID`) so the
picker and the next sync agree on the same board. Its error keeps being reported as a
warning step and never fails the import.

### D6 — Merge rules untouched

`SyncProjectBoardColumns` (`internal/db/board.go:274`) is the single merge for both
"Détecter" (through `ImportProjectBoardColumns`) and the sync. No new merge semantics are
introduced; the only change to that function is the board persistence of D5.

### D7 — English strings

The French messages on the lines this change touches (`"aucun board sur le projet %s"`,
`"aucun board sélectionné"`, `"projet non trouvé"` in the touched functions, and the
comment block at `board.go:83-86`) become English. Untouched lines are left alone: this
ticket is not a translation pass.

## Data contracts

| Contract | Shape | Change |
| --- | --- | --- |
| `GET /api/projects/{id}/boards` | `[{id, name, type}]` (`models.TrackerBoard`) | none |
| `POST /api/projects/{id}/board-columns` | body `{boardId}` → `Project` | none |
| `GET /api/projects/{id}/tracker-statuses` | `["To Do", …]` | non-empty on Jira |
| `GET /api/projects/detected-statuses` | `{statuses:[{id,name}]}` | adds optional `columns:[{name,statuses[]}]` |
| `models.TrackerColumn` | `{name, statuses[], hidden?}` | none |
| `Project.boardId` | `string` | now written from the UI |

## Target files

- `internal/db/board.go` — D1, D2 (`DetectProjectBoardColumns`), D5 (board persistence), D7.
- `internal/db/db.go` — D5 (`afterTrackerSync`).
- `internal/handlers/handlers.go` — D2 (`columns` in `/detected-statuses`).
- `web/src/components/BoardColumnsEditor.tsx` — D3, D4.
- `web/src/types/index.ts` — the detection response type, if it is declared there.
- `internal/db/board_test.go` (new or extended) — dispatch and merge tests.
- `internal/trackerapi/jira.go` — read-only reference, not expected to change.

## Risks

- **A board with a column whose statuses all resolve to unknown ids** is skipped by
  `ListBoardColumns`, and a board with no usable column returns an error. The UI must show
  that error rather than an empty board.
- **Column names are not unique** on a Jira board in theory; the merge keys on the
  lower-cased name. Duplicates would collapse. Accepted: it is the pre-existing behaviour
  of the merge, and Jira's own UI discourages it.
- **Persisting a board resolved implicitly** (D5) changes a project's stored state during a
  sync. It is the behaviour the owner asked for in question 1, and the picker always lets
  it be overridden.
