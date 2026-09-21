# #313 — Columns sync on JIRA

- Ticket: https://github.com/sebastienferry/sectile/issues/313 (Bug, `#clarified`)
- Branch: `feat/313`
- Clarification: [`docs/clarifications/313.md`](../../docs/clarifications/313.md)
- Technical approach: [`plan.md`](./plan.md) — implementation checklist: [`tasks.md`](./tasks.md)

## Context

A Sectile project owns a list of board columns (`Project.TrackerColumns`): each column is
a name plus the tracker statuses it groups, and the agentic workflow stages are mapped
onto those columns (`Project.StageColumns`). For a GitHub project the columns can be
detected from the tracker and refreshed on every sync. For a Jira project nothing
happens: the "Détecter" button reports "Aucune colonne trouvée", the status palette is
empty, and a tracker sync never touches the columns.

The Jira adapter is not the defect. `JiraAdapter` declares `CapBoard` and already
implements `ListBoards`, `ListBoardColumns` (from
`/rest/agile/1.0/board/{id}/configuration`, with status ids resolved to names) and
`ListStatuses` (from `/rest/api/3/project/{key}/statuses`, project-scoped). The defect is
the wiring around it, on four points confirmed during clarification:

1. No UI ever selects a board, so `Project.BoardID` stays empty — `listProjectBoards` and
   `importProjectBoardColumns` exist in `AppContext.tsx` but nothing calls them.
2. `afterTrackerSync` only runs `SyncProjectBoardColumns` when `BoardID` is non-empty, so
   the automatic refresh never fires on Jira.
3. `GetProjectTrackerStatuses` is guarded by `if proj.IssueTracker == "github"` and
   returns an empty slice for any other tracker.
4. `/api/projects/detected-statuses` only queries GitHub ProjectsV2, and the editor turns
   whatever it returns into one column per status name instead of mirroring the board's
   column → statuses grouping.

The clarification settled all five open product questions; this specification records the
resulting behaviour. Nothing here changes GitHub's observable behaviour.

## Decisions being specified

1. **Explicit board picker.** The project settings show the tracker's boards
   (`GET /api/projects/{id}/boards`) for any tracker with `CapBoard`, pre-selected with
   the project's first scrum board, and persist the choice in `Project.BoardID`.
2. **"Détecter" mirrors the board.** On a `CapBoard` tracker, detection produces one
   Sectile column per board column carrying the statuses that column groups. The
   one-column-per-status behaviour is kept only for the GitHub draft path.
3. **The palette is the project's statuses.** The draggable status list comes from the
   tracker's `ListStatuses` — a superset of the board's statuses — so a status the board
   does not show can still be assigned to a column by hand.
4. **Every tracker sync re-applies the columns.** Once a board is known,
   `afterTrackerSync` runs `SyncProjectBoardColumns` with its existing merge rules.
5. **Capability dispatch, not a Jira branch.** The GitHub-only conditions are replaced by
   a dispatch on `CapBoard`. GitHub keeps its ProjectsV2 path, GitLab inherits the new
   one, and validation is performed on Jira.

Out of scope: any write-back of columns to Jira, sprints, the column data model, and any
change to GitHub's observable behaviour.

## User stories

### US1 — Pick the Jira board of a project (priority: P1)

As the owner of a Jira-tracked project, I open the project settings and choose which Jira
board drives the columns, so that Sectile knows what to mirror.

- **Given** a project whose tracker is Jira and which has at least one scrum or kanban
  board, **when** I open the board section of the project settings, **then** a board
  selector lists the boards returned by `GET /api/projects/{id}/boards` with their name
  and type, and the project's first scrum board is pre-selected when `BoardID` is empty.
- **Given** a project whose `BoardID` is already set, **when** I open the same section,
  **then** the selector shows that board as the current selection.
- **Given** I select a board, **when** the selection is confirmed, **then**
  `POST /api/projects/{id}/board-columns` is called with that `boardId`, the project's
  `BoardID` is persisted, and the columns shown in the editor are the ones the call
  returned.
- **Given** a project whose tracker has no `CapBoard` capability (or a project that does
  not exist yet), **when** I open the section, **then** no board selector is shown and
  the existing column editing is unaffected.
- **Given** the board list cannot be read (credential missing, Jira unreachable),
  **when** the section loads, **then** the error message from the API is surfaced to the
  user and the rest of the editor stays usable.

### US2 — Detect the columns of a Jira board (priority: P1)

As the owner of a Jira-tracked project, I press "Détecter" and get my Jira board's columns
as Sectile columns, so that the board reads like the one my team uses in Jira.

- **Given** a Jira board with columns `To Do` (statuses `To Do`, `Backlog`),
  `In Progress` (status `In Progress`) and `Done` (status `Done`), **when** I press
  "Détecter", **then** the editor holds exactly three columns with those names, in the
  board's order, each carrying the statuses that Jira groups under it.
- **Given** the project already has a column whose name matches a board column, **when**
  detection runs, **then** that column keeps the statuses assigned by hand that the board
  claims nowhere, and keeps its hidden flag.
- **Given** the project has a column the board does not know, **when** detection runs,
  **then** that column is kept at the end of the list with the statuses nothing else
  claims, and is dropped only when it has no unclaimed status left.
- **Given** a workflow stage is mapped to a column that disappeared from the board,
  **when** detection runs, **then** that mapping entry is dropped and every other stage
  mapping is left untouched.
- **Given** the selected board exposes no usable column, **when** I press "Détecter",
  **then** the failure is reported to the user and the existing columns are left as they
  were.
- **Given** a GitHub project, **when** I press "Détecter", **then** the result is exactly
  what it is today.

### US3 — Assign a Jira status to a column by hand (priority: P2)

As the owner of a Jira-tracked project, I see the project's statuses in the palette, so I
can place a status the board does not show into one of my columns.

- **Given** a Jira project whose workflows expose statuses `To Do`, `In Progress`,
  `In Review`, `Done`, **when** the board editor opens, **then** the palette lists those
  statuses, whether or not the selected board groups them.
- **Given** a status is already assigned to a column, **when** the palette renders,
  **then** that status is not offered again in the palette.
- **Given** the status list cannot be read, **when** the editor opens, **then** the
  palette is empty, the column list is still shown, and the failure is surfaced rather
  than being silently swallowed.
- **Given** a GitHub project, **when** the editor opens, **then** the palette is what it
  is today (ProjectsV2 single-select options, falling back to `open` / `closed`).

### US4 — Keep the columns in step with Jira on every sync (priority: P2)

As a user of a Jira-tracked project, I get the board's current columns after each tracker
synchronisation, without having to press anything.

- **Given** a Jira project with a `BoardID`, **when** a tracker sync completes, **then**
  `SyncProjectBoardColumns` runs and the sync report carries its outcome as a step.
- **Given** a Jira project whose `BoardID` is empty but whose tracker has `CapBoard`,
  **when** a tracker sync completes, **then** the sync resolves the project's first scrum
  board (falling back to its first board), persists it as `BoardID`, and applies its
  columns.
- **Given** the board read fails during the sync, **when** the sync completes, **then**
  the imported work items are kept, the failure appears as a warning step, and the sync
  is not marked as failed because of it.
- **Given** a tracker without `CapBoard`, **when** a sync completes, **then** no board
  step is attempted, as today.

### US5 — Same wiring for every board-capable tracker (priority: P3)

As a maintainer, I want the board plumbing selected by capability rather than by tracker
name, so a third tracker does not need a new branch.

- **Given** `GetProjectTrackerStatuses` is called for a project whose tracker has
  `CapBoard` and is not GitHub, **when** it runs, **then** it returns the tracker's
  statuses instead of an empty slice.
- **Given** it is called for a GitHub project, **when** it runs, **then** the returned
  list is unchanged, including the `open` / `closed` fallback.
- **Given** it is called for a tracker with neither the GitHub path nor `CapBoard`,
  **when** it runs, **then** it returns an empty list and no error, as today.

## Functional requirements

- **FR1** — For any project whose tracker declares `CapBoard`, the project settings expose
  a board selector fed by `GET /api/projects/{id}/boards`, pre-selected with the first
  scrum board when `Project.BoardID` is empty.
- **FR2** — Selecting a board persists it in `Project.BoardID` and imports that board's
  columns through the existing `POST /api/projects/{id}/board-columns`.
- **FR3** — On a `CapBoard` tracker, detection returns the board's columns with their
  grouped statuses, in board order; one column per status is used only on the GitHub draft
  path.
- **FR4** — `GetProjectTrackerStatuses` dispatches on capability: the GitHub ProjectsV2
  path for GitHub, the tracker's `ListStatuses` for any other `CapBoard` tracker, an empty
  list otherwise.
- **FR5** — The merge rules of `SyncProjectBoardColumns` are unchanged and are the single
  merge used by both "Détecter" and the automatic sync: the tracker owns column names and
  order; hand-assigned statuses the tracker claims nowhere survive; the hidden flag
  survives; user-only columns are kept at the end while they hold an unclaimed status;
  stage mappings to vanished columns are dropped.
- **FR6** — `afterTrackerSync` runs the board step for every `CapBoard` tracker, resolving
  and persisting a board when none is recorded, and never fails the import on a board
  error.
- **FR7** — Every failure of the board, column or status reads is reported to the user
  with the message the tracker gave; no path replaces a failure with an empty result.
- **FR8** — GitHub's observable behaviour — detection result, palette content, sync steps
  — is identical before and after this change.

## Non-functional requirements

- **NFR1** — The status list stays project-scoped (`/rest/api/3/project/{key}/statuses`);
  no code path reads the whole Jira instance's status catalogue.
- **NFR2** — No database migration: `BoardID`, `TrackerColumns` and `StageColumns` already
  exist on the project model.
- **NFR3** — Board, column and status reads keep the existing `boardAPITimeout` (60 s) and
  run on behalf of the acting user, so a personal Jira credential is resolved.
- **NFR4** — User-facing strings added or rewritten in Go code are English, per the
  repository policy. The pre-existing French strings in `internal/db/board.go` are
  corrected only on the lines this change already touches.

## Acceptance criteria

The ticket is done when, on a Jira-tracked project:

1. The project settings show a board picker, pre-selected with the first scrum board, and
   the choice survives a reload.
2. "Détecter" produces one column per Jira board column with its grouped statuses, and the
   toast reports the number of columns.
3. The palette lists the project's statuses and a status can be dragged into a column.
4. A tracker sync re-applies the board columns and reports the step, while hand-assigned
   statuses and hidden columns survive.
5. The same run on a GitHub project produces the same result as before the change.
6. `go build ./...`, `go test ./...` and the web build/tests pass.

## Open requirements

None. The clarification closed all five product questions; nothing in this specification
is blocked.
