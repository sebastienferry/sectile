# Plan #679 - Store and correct the Jira ticket priority mapping

Behaviour: `spec.md`. This plan carries over `specs/635-epic-axes-tracker-fields/plan.md`
"Part B", updated to `main` at `4e6df214`.

## Stack

Go server (`internal/models`, `internal/db`, `internal/tracker`,
`internal/trackerapi`, `internal/handlers`, `internal/taskmcp`), React web
client (`web/src`). No desktop change: `desktop/src/main.js` only displays
the priority.

## Data

`models.Project` (`internal/models/models.go`) gains:

```go
// PriorityMapping is how the tracker's ticket priorities map to Sectile's
// four levels (#679). Empty until the first discovery.
PriorityMapping PriorityMapping `json:"priorityMapping"`

type PriorityMapping struct {
    Options   []PriorityMappingOption `json:"options,omitempty"`   // scheme order, most urgent first
    Preferred map[Priority]string     `json:"preferred,omitempty"` // level -> option id
}

type PriorityMappingOption struct {
    ID      string   `json:"id"`
    Name    string   `json:"name"`
    Level   Priority `json:"level"`
    Guessed bool     `json:"guessed,omitempty"`
    Manual  bool     `json:"manual,omitempty"`
}

// PriorityOption is one option of a tracker's priority scheme, as read.
type PriorityOption struct {
    ID   string
    Name string
}
```

- Migration `projects.priority_mapping`, `TEXT NOT NULL DEFAULT '{}'`, in
  `internal/db/migrations.go` only, never in the frozen baseline. Its number
  is the next free one on `main` when the branch is brought up to date for
  review: 47 today, 48 if `feat/732` (which also claims 47) merges first.
- The column is read and written by the project SELECT, INSERT and UPDATE
  statements of `internal/db/db.go`, as `epic_axis_prefixes` is (JSON text).
- Every rewind helper that drops `epic_axis_prefixes`
  (`activerun_test.go`, `branchformat_test.go`, `migrations_test.go` twice,
  `specartifacts_test.go`, `stagecommits_test.go`) also drops
  `priority_mapping`, and the forget-and-replay fixtures of
  `migrations_test.go` learn the new version.
- `UpdateProjectRequest` gains `PriorityMapping *PriorityMapping`. On save,
  the server validates it against the stored mapping: an option id or a
  preferred id it does not carry, or a level outside the four, answers 400.
  A line whose level differs from the stored one, or whose `Guessed` the
  client cleared, is stored with `Manual` true and `Guessed` false. Option
  ids, names and order are always taken from the stored mapping, never from
  the client.

## Pure functions

New `internal/models/prioritymapping.go`, so `db` and `trackerapi` share them
without an import cycle:

- `(m PriorityMapping) Empty() bool`;
- `(m PriorityMapping) Writable(level Priority) bool`: a sure line carries it;
- `(m PriorityMapping) WritableLevels() []Priority`, most urgent first;
- `(m PriorityMapping) OptionFor(level Priority, offered []string) (id string, ok bool)`:
  the preferred option when sure and offered, else the most urgent sure
  offered option of the level; `offered` nil means every option;
- `MergePriorityMapping(stored PriorityMapping, scheme []PriorityOption, classify func(name string, rank, n int) (Priority, bool)) PriorityMapping`:
  keeps existing lines untouched (refreshing only the name and the order of
  the scheme), appends new ones classified (`bool` true is sure), drops gone
  ones and any preferred id naming them.

## Tracker

- `internal/tracker/ticketing.go` gains an optional interface:

  ```go
  // PrioritySchemeReader is implemented by an adapter whose tracker has a
  // configurable ticket priority scheme (Jira).
  type PrioritySchemeReader interface {
      PriorityScheme(ctx context.Context, project *models.Project, fresh bool) ([]models.PriorityOption, error)
  }
  ```

- `JiraAdapter` implements it: the project's create screen for its first
  configured issue type (`jiraCreatePriorities`), else the site list
  (`readJiraPriorities`, which already reads `/rest/api/3/priority` for an
  OAuth client lacking the admin scope of `/priority/search`). `fresh` skips
  the 10-minute caches of `jira_priority.go`.
- `trackerapi.ClassifyJiraPriority(name string, rank, n int) (models.Priority, bool)`
  wraps `jiraPriorityOf` (sure) and `priorityAt` (guessed) for the db layer.
- `priorityFieldFor` (`jira_priority.go:370`) gains the project's mapping.
  With an empty mapping its path is unchanged (FR-10). Otherwise it picks
  `mapping.OptionFor(level, screen option ids)`, and with no sure option
  returns `ErrGuessedPriority`, a typed error carrying the level and the
  writable levels, whose message is the refusal sentence below.
- The mapping reaches the adapter through `req.Project`, which both
  `tracker.CreateIssueRequest` and `tracker.UpdateIssueRequest` already
  carry, loaded when the write runs; a queued write therefore sees the
  mapping as it is then (FR-8).
  - `UpdateIssue` (`jira.go:359`) returns `ErrGuessedPriority`, failing the
    queued activity with the sentence.
  - `CreateIssue` (`jira.go:215`) and `setPriorityAfterCreate` (`:332`) drop
    the field and set `PriorityNotice string` on the returned `models.Task`
    (JSON `priorityNotice,omitempty`, never stored).

## Refusal before the local write

- `db.CheckPriorityWritable(proj *models.Project, level models.Priority) error`
  returns nil for a non-Jira project, an empty mapping or a writable level,
  else `ErrGuessedPriority` with the English sentence:
  "Priority \"high\" is not mapped with certainty to a Jira priority of this
  project. Accepted priorities: urgent, medium. Confirm the mapping in the
  Tracker tab of the project settings."
  With no writable level at all, the list reads "none".
- Called at the top of `UpdateTaskBy` (`db.go:2831`) when `req.Priority` is
  set and differs from the stored priority, before `updateTaskBy` writes
  anything. Its callers, the web handler (`handlers.go:3333`) and MCP
  `update_task` (`taskmcp/server.go:645`), refuse alike.
- The web handler maps `ErrGuessedPriority` to 422 with the sentence; MCP
  returns it as the tool error.
- `CreateTaskAs` (`db.go:2501`) passes the priority through; the adapter
  decides (FR-7), and the notice travels back on the returned task to the
  web handler and to MCP `create_task`, which adds it to its answer.

## Discovery

- New `internal/db/prioritymapping.go`:
  `(d *DB) RefreshPriorityMapping(ctx context.Context, projectID string, fresh bool) (string, error)`
  reads the scheme through `PrioritySchemeReader`, merges, stores, and
  returns an English summary ("4 priorities, 2 guessed"). A tracker that does
  not implement the interface returns "" and nil.
- Called from the project sync (`db.go`, after the board step at `:4467`)
  under `describesProject && the adapter implements PrioritySchemeReader`, as
  the next numbered step; an error is a "⚠️" step and does not fail the sync,
  like the board.
- `POST /api/projects/{id}/priority-mapping/refresh`, beside the
  `board-columns` sub-action of `handlers.go` and gated the same way, runs it
  with `fresh` true under `h.actingContext(r)` and returns the project.

## Web

- `web/src/types/index.ts`: `priorityMapping?` on `Project`,
  `priorityNotice?` on `Task`.
- `ProjectModal.tsx`, Tracker tab, Jira projects only: a priority mapping
  table. Columns: option name, level select, a "guessed" badge with a confirm
  button; under each level carried by several sure options, a choice of the
  preferred one, the most urgent preselected; a line per level no sure option
  carries saying it cannot be written; a refresh button calling the endpoint.
  Saved with the project. Labels in `web/src/locales/projectSettings.ts`, in
  French and English.
- The card form and the priority chips already show a failed update's message
  in the error toast; the 422 sentence reaches them unchanged.
- `ListView.tsx` `handleBulkPriority` (`:335`): continues past a refused
  ticket, then shows one toast with the written count and the refused keys
  with the sentence. The partition of results lives in a small pure helper
  tested by `node --test`.
- The creation flow (`QuickAddModal.tsx` and the other `createTask` callers
  that toast) adds the `priorityNotice` to its success toast when present.

## Changelog

`CHANGELOG.md`, `## [Unreleased]`:

- `Added`: a Jira project shows how its ticket priorities map to Sectile's
  levels in the Tracker tab, where guessed lines can be confirmed or changed.
- `Changed`: on a Jira project, a priority update that the mapping only
  guessed is refused, and a creation goes out without that priority.

## Rejected alternatives

- **Refusing in the Jira adapter only.** The local write would already have
  happened and the next sync would undo it; the owner chose the refusal
  before any write (#635 round 2).
- **Discovery at every background sync.** The board and team steps already
  run only on a full sync; rediscovering every poll costs a request for a
  scheme that rarely changes.
- **French messages.** The owner's convention since 2026-10-05 writes new
  user-facing strings in English; neighbouring French sync steps are not
  translated in passing.

## Test plan

- `internal/models/prioritymapping_test.go`: merge (new, gone, untouched,
  manual, preferred dropped with its option), `Writable`, `WritableLevels`,
  `OptionFor` with and without a preferred option and with a restricted
  screen.
- `internal/trackerapi/jira_priority_test.go`: classification of Atlassian's
  default, the Server scheme and French names (all sure) and `P1`..`P4`
  (guessed); `PriorityScheme` from the create screen and from the site list,
  `fresh` bypassing the cache; update refused with `ErrGuessedPriority`;
  creation without priority and its notice; empty mapping unchanged.
- `internal/db`: `RefreshPriorityMapping` on a fake reader; sync runs it on a
  full sync and not on a background one; `UpdateTaskBy` refuses before any
  write (row unchanged, no activity queued, other fields unwritten), accepts
  an unchanged priority, leaves non-Jira and empty mappings alone; project
  save validation; the migration and the rewind helpers on SQLite and
  PostgreSQL (`SECTILE_TEST_POSTGRES_DSN`, throwaway UTF8 database).
- `internal/handlers`: 422 with the sentence; the refresh endpoint.
- `internal/taskmcp`: `update_task` returns the sentence; `create_task`
  reports the notice.
- Web `node --test web/tests`: the bulk result helper.
- Gates: `go test ./...`, `go vet ./...`, `cd web && npx tsc --noEmit`,
  `npx oxlint`, `node --test web/tests`.
