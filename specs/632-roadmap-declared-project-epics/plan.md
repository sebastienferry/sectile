# Plan #632 - Roadmap: show the epics of declared tracker projects

## Stack

Go server (`internal/db`, `internal/handlers`, `internal/models`,
`internal/trackerapi`), SQLite and PostgreSQL through the numbered migrations,
React 19 + TypeScript web app (`web/`), `node --test` unit tests and Go tests.
The tracker writes reuse the queued tracker operations
(`internal/db/trackerops.go`). No agent or desktop change.

## Design

### 1. Storage

- **Migration 37** `projects.roadmap_axis_writes` in
  `internal/db/migrations.go` only, never in the frozen baseline:

  ```sql
  ALTER TABLE projects ADD COLUMN roadmap_axis_writes INTEGER NOT NULL DEFAULT 0;
  ```

  Renumber if `main` lands a 37 first. The rewind test helpers of
  `internal/db/migrations_test.go` (the `dropRepositoryColumns` family) and the
  forget-and-replay fixtures drop the new column.
- `models.Project` gains `RoadmapAxisWrites bool` (`json:"roadmapAxisWrites"`),
  the create and update requests gain `RoadmapAxisWrites *bool`. The two project
  `SELECT`s of `db.go` (around `:6065` and `:6204`), the `INSERT` (`:6396`) and
  the `UPDATE` (`:6635`) carry the column. The value is forced to `false` when
  the project is not a Jira project or declares no key, so a stale `true` can
  never open writes after the declaration is emptied.
- Foreign epics are stored in `macros` under the declaring project's id, as the
  own epics are. No new table, no origin column: the origin is the key prefix.
- `models.MacroTodo` gains `TargetTrackerProject string`
  (`json:"targetTrackerProject,omitempty"`), a declared key. It is exclusive with
  `TargetProjectID`: saving a line with both keeps the declared key and empties
  the other. The todos are a JSON column, so no migration.

### 2. Origins (`internal/db/roadmapprojects.go`)

```go
// macroOrigin is the key prefix of an epic key, upper-cased: "DATA-12" -> "DATA".
// A key without a dash (a milestone, M-<n> keys excepted) has no origin: "".
func macroOrigin(key string) string

// isForeignMacro tells an epic of a declared or former roadmap project from an
// own one. Milestones and local M-<n> keys are never foreign.
func isForeignMacro(key string, proj *models.Project) bool

// isDeclaredRoadmapProject reports whether a key is in the current declaration.
func isDeclaredRoadmapProject(proj *models.Project, key string) bool
```

`belongsToProject` stays the single definition of "own": `isForeignMacro` is
`!isMilestoneKey(key) && !belongsToProject(key, proj)` on a Jira project, and
`false` elsewhere. `isRoadmapProjectKey` keeps its meaning for story keys.

`models.MacroMeta` gains two computed fields, never stored, filled by
`GetProjectMacros` and by every handler that returns a macro:

```go
// Origin is the tracker project key of the epic, "" for a milestone or a local key.
Origin string `json:"origin,omitempty"`
// Foreign tells an epic of a roadmap project, which Sectile reads without writing.
Foreign bool `json:"foreign,omitempty"`
```

### 3. Reading the declared keys (`internal/db/macrohorizons.go`)

- `remoteMacros(ctx, proj)` reads the own key as today. Then, for each key of
  `proj.RoadmapProjects`, it calls `ListEpics` with a shallow copy of the project
  whose `JiraProject` is the declared key, under its own
  `context.WithTimeout(ctx, macroReadTimeout)`. The copy carries the same id, so
  `forProject` resolves the same site and the same acting person's token.
- It returns the epics keyed by key and a `[]keyFailure{Key string; Err error}`.
  A failure of the own key returns the error as today; a failure of a declared
  key is appended and the loop goes on.
- `ImportMacroHorizons` imports the foreign epics exactly like the own ones
  (`saveMacroMetaFull` then `SaveMacroAxes` from labels), and appends to its
  summary one French sentence per failing key:
  `projet de roadmap DATA non lu : <err>`. The summary counts stay the total.
- The periodic sync (`db.go:4428`) and the "import labels" handler
  (`handlers.go:1163`) call `ImportMacroHorizons` already: nothing else changes.
- No `SyncIssues` call ever receives a declared key: the task import stays scoped
  to the own key.

### 4. Writes on foreign epics

The crossing point is `macroTracker` (`macrohorizons.go:87`), shared by the
horizon, free labels, priority and quarter writes. It gains the axis being
written:

```go
type macroAxis int
const (
	axisHorizon macroAxis = iota
	axisLabels
	axisPriority
	axisQuarter
)
func (d *DB) macroTracker(projectID, macroKey string, axis macroAxis) (tracker.TicketingSystem, *models.Project, error)
```

- An own epic: unchanged.
- A foreign epic: refused with today's sentence, except for `axisPriority` and
  `axisQuarter` when `proj.RoadmapAxisWrites` is true and the origin is still
  declared. It then returns the tracker with the project copy of section 3, so
  `UpdateIssue` targets the right Jira project.
- `pushMacroLabels` passes the axis through; `PushMacroHorizonLabel` passes
  `axisHorizon`, `PushMacroPriorityLabel` and `PushMacroQuarterLabel` their own,
  `ValidateMacroLabelEdit` and `PushMacroLabels` pass `axisLabels`.

`MacroLabelsWritable(projectID, key)` becomes
`MacroAxesWritable(projectID, key string, bulk bool) (priority, quarter, horizon bool)`,
or keeps its name and gains the same parameters; either way:

- horizon writable: own epic and labelled tracker (today's rule);
- priority and quarter writable: the horizon rule, or, for a foreign epic, the
  axis opt-in open, the origin still declared and `bulk` false.

`LabelsWritable` on `MacroMeta` keeps meaning "the horizon is written". A new
computed `AxesWritable bool` (`json:"axesWritable"`) says whether the priority
and the quarter are, for a single panel edit. The panel note reads
`AxesWritable`.

- **Horizon of a foreign epic** (`handlers.go:1425`): the handler enqueues
  `TrackerOpEpicHorizon` only when the horizon is writable, and answers
  `labelNote: "conservé dans Sectile, non écrit sur le tracker"` otherwise. Today
  it enqueues and the operation fails in the queue.
- **Priority and quarter** (`handlers/macroaxes.go`): `enqueueMacroAxes` reads
  `AxesWritable` instead of `LabelsWritable`.
- **Bulk writes**: the macro `PUT` accepts `"bulk": true`. The seeding
  confirmation (`RoadmapView.tsx:774`, `runSeed`) sends it. With `bulk`, a foreign
  epic's priority and quarter are stored and never enqueued.
- **Pending pushes** (`pendingAxisPushes`, `macrohorizons.go:311`) keep calling
  `belongsToProject` and never list a foreign epic.
- The queued operation re-checks through `macroTracker` when it runs, so an opt-in
  closed or a key undeclared between the click and the run refuses the write with
  a readable activity rather than writing.

### 5. Story creation in a declared key (`internal/db/macros.go`)

`CreateStoryFromMacroTodo` (the function around `:500`):

1. The `isRoadmapProjectKey(proj, todo.StoryKey)` refusal stays for a line
   already attached to a declared story key.
2. When `todo.TargetTrackerProject` is set: refuse with
   `« DATA » n'est plus un projet de roadmap de <projet> : choisissez une autre cible pour cette ligne`
   unless `isDeclaredRoadmapProject(proj, key)`; refuse when the project is not a
   Jira project.
3. Call a new `createStoryInRoadmapProject(ctx, proj, key, macroKey, title)`
   (new file `internal/db/remotetarget.go`) that:
   - resolves the tracker with the project copy of section 3;
   - calls `CreateIssue` with the title, `IssueType` = the story type the project
     uses for its own stories, and `ParentKey` = the macro key;
   - on a refusal, returns the adapter's error, which already names the missing
     mandatory fields (`missingRequiredFields`);
   - on a created issue whose parent was not accepted, returns the key with a
     notice `épic <macro> non posé comme parent sur Jira : <err>`;
   - records no task: no `CreateTaskAs`, no local row, no sync.
4. Record `todo.StoryKey = created.Key` through `SaveMacroMeta` as today, and
   return a notice ending with `la story reste dans Jira, hors du board`.

The response shape is unchanged; `task` carries the key, title and external URL
of the created issue with an empty `ID`, and the web reads its absence of id as
"not imported" (no navigation to a task that does not exist).

### 6. Web (`web/`)

- `web/src/lib/roadmapOrigins.ts` (new), pure and unit tested:
  - `macroOrigin(key)`; `offeredOrigins(project, rows)` per FR4;
  - `originCounts(rows)` over the rows the other filters let through (FR5);
  - `normalizeOriginSelection(selection, offered, ownKey)` per FR6;
  - `readOriginSelection(projectId, storage?)` and `writeOriginSelection(...)`
    under `sectile_roadmap_origins:<projectId>`, tolerant like
    `roadmapViewPrefs.ts`.
- `RoadmapView.tsx`:
  - a `selectedOrigins` state, read from storage per project on project change;
  - `unlabelledRows` (`:559`) also filters on the origin; the counts use a copy of
    the same pipeline without the origin filter;
  - a toolbar multi-select "Projets" beside the priority filter, shown per FR4,
    each entry `KEY count`, the own key first;
  - an active filter chip (`activeFilterChips`, `:432`) when the selection is not
    the default;
  - the row shows the origin as a small key badge when `meta.foreign`;
  - the panel shows the read-only mark when `meta.foreign`, hides
    `EpicLabelEditor` (`:1802`), the framing comment post and the epic edit
    actions; the priority and quarter controls stay, and the note under them
    (`:1795`) reads `axesWritable` and, for a foreign epic with the opt-in closed,
    says the opt-in would write them;
  - `runSeed` sends `bulk: true`.
- `web/src/lib/lookups.ts`: `targetProjectOptions` stays; a new
  `roadmapTargetOptions(project)` returns the declared keys of a Jira project.
  The picker (`RoadmapView.tsx:2358`) renders the Sectile projects, then an
  option group "Projets Jira de la roadmap" with one option per declared key
  whose value is `tracker:<KEY>`, mapped to `targetTrackerProject` on save.
- `ProjectModal.tsx` (`:1193`): a checkbox under "Roadmap projects", shown when
  the parsed list is not empty, bound to `roadmapAxisWrites`, sent `false` when
  the list is empty or the tracker is not Jira.
- `web/src/types/index.ts`: `Project.roadmapAxisWrites`, `MacroMeta.origin`,
  `MacroMeta.foreign`, `MacroMeta.axesWritable`, `MacroTodo.targetTrackerProject`.
- Strings in `web/src/locales/planning.ts` and `projectSettings.ts`, French and
  English, including the new help text of `roadmapProjectsHelp` (FR18).

### 7. Documentation

- `CHANGELOG.md`, three `Added` lines under `[Unreleased]` (FR20).
- `docs/adrs/0043-declared-roadmap-projects-are-read-except-new-stories-and-opted-in-axes.md`
  (renumber if taken): context (#426's rule), decision (FR21), consequences
  (stories not imported, labels on other teams' epics once opted in, bulk writes
  never cross).
- `docs/` pages describing the roadmap settings, if any mentions the roadmap
  projects, follow the new help text.

## Data contracts

```http
PUT /api/projects/{id}/macros/{key}
{ "priority": "p1", "quarter": "2026-Q4", "bulk": true }
-> { "macro": { ..., "origin": "DATA", "foreign": true, "labelsWritable": false, "axesWritable": false }, "labelNote": "conservé dans Sectile, non écrit sur le tracker" }

PUT /api/projects/{id}
{ ..., "roadmapProjects": ["DATA", "OPS"], "roadmapAxisWrites": true }
```

The slicing line's JSON gains `targetTrackerProject`; the creation endpoint is
unchanged.

## Rejected alternatives

- **One JQL `project in (PE, DATA, OPS)`**: one unreachable key fails the whole
  read, the own epics included.
- **A `macros.origin` column**: the prefix already says it, and a stored copy can
  drift from the key.
- **Importing the created story into the project**: contradicts the rule that no
  work item of a declared project is imported (clarification Q2).
- **Encoding the declared key in `TargetProjectID`** (`jira:DATA`): every reader
  of that field would have to learn to tell a Sectile id from a tracker key.
- **Hiding the axis controls while the opt-in is closed** (Taskativ): rejected in
  clarification round 3.
- **Syncing the origin selection on the server**: every other roadmap view
  choice is per browser (`roadmapViewPrefs.ts`).

## Risks

- The declared project's creation screen can require fields the own project does
  not; the refusal names them, and the line stays creatable once fixed on Jira.
- N declared keys add N epic reads per sync; each has its own timeout, so the
  worst case is bounded by N times `macroReadTimeout`.
- A person whose token cannot read a declared key sees it in the failures of
  every read until the key is removed; the message says which key.

## Target files

- `internal/db/migrations.go`, `internal/db/migrations_test.go`
- `internal/models/models.go`
- `internal/db/db.go`
- `internal/db/roadmapprojects.go`, `internal/db/roadmapprojects_test.go`
- `internal/db/macrohorizons.go`, `internal/db/macroaxes.go`,
  `internal/db/macrolabels.go`, `internal/db/macros.go`
- `internal/db/remotetarget.go` (new) and its test
- `internal/handlers/handlers.go`, `internal/handlers/macroaxes.go`
- `web/src/lib/roadmapOrigins.ts` (new) and its test, `web/src/lib/lookups.ts`
- `web/src/components/RoadmapView.tsx`, `web/src/components/ProjectModal.tsx`
- `web/src/types/index.ts`, `web/src/locales/planning.ts`,
  `web/src/locales/projectSettings.ts`
- `CHANGELOG.md`, `docs/adrs/0043-*.md`

## Implementation notes

What the implementation settled differently from the design above, keeping the
acceptance criteria:

- **ADR number**: 0043, `main` having taken 0042 in the meantime.
- **Opt-in location**: the checkbox sits under the "Roadmap projects" field of
  the Tracker settings, as D10 of the clarification says.
- **Horizon guard**: the handler skips the queued horizon write for a foreign
  epic only (`MacroIsForeign`), not for every macro `MacroLabelsWritable`
  refuses. `macroTracker` does not require `CapEpic`, so a wider guard would
  have changed what a GitLab project queues today.
- **Writes on a foreign epic** keep the project itself, not a copy: Jira writes
  by key. The copy (`roadmapProjectView`) is used for the epic read and the
  story creation, which need the key of the declared project.
- **Story notice**: the creation in a declared project returns a notice only
  when Jira refuses the parent, as `createStoryUnder` does. The web tells the
  story stays in Jira from the answer's task without an id.
- **Framing comment**: it is stored in Sectile and never posted to Jira, so
  nothing had to be hidden for FR9; the panel hides the free label editor.
- **Link of a story left in Jira**: the slicing line builds its Jira page from
  the project's tracker URL or the Jira site setting, the story not being a
  local task.
- **Selection storage**: a choice made on a page holds for its project even when
  the storage refuses it, and another project reads its own.
