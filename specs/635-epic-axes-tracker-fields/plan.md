# Plan #635 - Map the epic axes to tracker fields

Behaviour: `spec.md`. This file holds the implementation choices.

## Stack and constraints

- Server: Go, `internal/db` (storage, import, pushes), `internal/trackerapi`
  (Jira adapter), `internal/handlers` (HTTP), `internal/taskmcp` (MCP).
- Web: React and TypeScript, `web/src/components/ProjectModal.tsx`,
  `web/src/lib/roadmap.ts`, `web/src/types/index.ts`, `web/src/locales`.
- New project columns only in `internal/db/migrations.go`, after version 38
  (`projects.roadmap_axis_writes`), each `NOT NULL DEFAULT` so an empty value
  is today's behaviour. Never in the frozen baseline. Every rewind helper that
  drops `roadmap_axis_writes` (`migrations_test.go` `dropRepositoryColumns` and
  the list at `:449`, `activerun_test.go:171`, `branchformat_test.go:122`,
  `specartifacts_test.go:132`, `stagecommits_test.go:92`) drops the new
  columns too. Renumber if `main` lands a migration first.
- Code, comments and tests in English; user-facing strings in French, with
  their English counterparts in the web locales.
- The repository is public: no real Jira instance identifier anywhere.

## Part A - Axis label prefixes

### Data

`models.Project` gains:

```go
// EpicAxisPrefixes names the label prefixes of the epic's own axes (#635).
// An empty field keeps the default prefix of its axis.
EpicAxisPrefixes EpicAxisPrefixes `json:"epicAxisPrefixes"`

type EpicAxisPrefixes struct {
    Priority  string `json:"priority,omitempty"`
    Quarter   string `json:"quarter,omitempty"`
    Readiness string `json:"readiness,omitempty"`
}
```

Stored as one JSON column, migration 39 `projects.epic_axis_prefixes`
(`TEXT NOT NULL DEFAULT '{}'`). `UpdateProjectRequest` gains
`EpicAxisPrefixes *EpicAxisPrefixes`. The project SELECTs (`db.go:6068`,
`:6210`), INSERT (`:6405`) and UPDATE (`:6648`) carry it.

### Domain

In `internal/db/macroaxes.go`:

- An `axisPrefixes` value resolved from a project
  (`prefixesFor(proj *models.Project) axisPrefixes`), defaults filled in. The
  existing constants stay as the defaults.
- `CleanEpicAxisPrefixes(in models.EpicAxisPrefixes) (models.EpicAxisPrefixes, error)`
  applies FR-A2. Called by `UpdateProject` and `CreateProject` before storage;
  the errors are French sentences naming the axis and the prefix.
- `PriorityFromLabels`, `QuarterFromLabels`, `ReadinessFromLabels`,
  `isQuarterLabel`, `PriorityLabel`, `QuarterLabel`, `ReadinessLabel`,
  `AllPriorityLabels`, `AllReadinessLabels` become methods of `axisPrefixes`
  (or take it as first argument). `epicPriorityPattern` no longer hard-codes
  `priority:`: the value is matched after `strings.CutPrefix` of the effective
  prefix. `NormalizeEpicPriority` and `NormalizeReadiness` keep accepting the
  default-prefixed forms they accept today for typed input.
- `PushMacroPriorityLabel`, `PushMacroQuarterLabel`, `PushMacroReadinessLabel`
  load the project (they already resolve it through `macroTracker`) and use its
  prefixes.
- `ImportMacroHorizons` (`macrohorizons.go:263`) and `pendingAxisPushes`
  (`:406`) use `prefixesFor(proj)`.

In `internal/db/macrolabels.go`:

- `macroAxisPrefixes` becomes a function of the project:
  `axisLabelPrefixes(proj) []string` = `roadmap:` + the three effective
  prefixes. `IsMacroAxisLabel(label)` becomes `isMacroAxisLabel(proj, label)`;
  `cleanLabelEdit` and `ValidateMacroLabelEdit` (which already has the project
  id) pass the project. Every caller found by
  `grep -rn IsMacroAxisLabel internal` is updated.

Foreign epics (#632): the prefixes are those of the project the roadmap
belongs to, the `proj` already passed down `roadmapProjectMacros` and
`foreignAxisWritable`. No per-roadmap-project prefix.

### HTTP and web

- The project JSON already flows through the project endpoints; the new field
  rides along. A refused prefix answers 400 with the French sentence.
- `web/src/types/index.ts`: `epicAxisPrefixes?: { priority?: string; quarter?: string; readiness?: string }` on `Project`.
- `web/src/lib/roadmap.ts`: `EPIC_AXIS_LABEL_PREFIXES` becomes
  `epicAxisLabelPrefixes(project)`, and `isEpicAxisLabel(label, project)`,
  `freeEpicLabels(meta, project)` take the project. A client-side
  `cleanEpicAxisPrefix` mirrors the server cleaning for inline validation.
  Callers: `EpicLabelEditor.tsx`, `RoadmapView.tsx` and any other found by
  grep, which already know the current project.
- `ProjectModal.tsx`: a "Roadmap" section of the Tracker tab, beside
  `roadmapProjects` (`:1199`), three text inputs with the default prefix as
  placeholder, shown when the tracker can carry epic labels (Jira today, the
  same condition the roadmap uses to write labels). Strings in
  `web/src/locales/projectSettings.ts`.

## Part B - Ticket priority mapping

### Data

`models.Project` gains:

```go
// PriorityMapping is how the tracker's ticket priorities map to Sectile's
// four levels (#635). Empty until the first discovery.
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
```

Migration 40 `projects.priority_mapping` (`TEXT NOT NULL DEFAULT '{}'`).
`UpdateProjectRequest` gains `PriorityMapping *PriorityMapping`; a write from
the web sets `Manual` and clears `Guessed` on the lines it changes, and the
server refuses an option id or a preferred id the stored mapping does not
carry, and a level that is not one of the four.

Pure functions, in a new `internal/models/prioritymapping.go` so both `db` and
`trackerapi` use them without an import cycle:

- `(m PriorityMapping) Writable(level Priority) bool`;
- `(m PriorityMapping) OptionFor(level Priority, offered []string) (id string, ok bool)`:
  the preferred option when sure and offered, else the most urgent sure offered
  option of the level. `offered` nil means "every option".
- `(m PriorityMapping) WritableLevels() []Priority`;
- `MergePriorityMapping(stored PriorityMapping, scheme []PriorityOption, classify func(name string, rank, n int) (Priority, bool)) PriorityMapping`
  for FR-B2/B3: keeps existing lines untouched, appends new ones, drops gone
  ones, drops a preferred id whose option is gone.

### Tracker

- A new optional interface in `internal/tracker`:

  ```go
  // PrioritySchemeReader is implemented by an adapter whose tracker has a
  // configurable ticket priority scheme (Jira).
  type PrioritySchemeReader interface {
      PriorityScheme(ctx context.Context, project *models.Project) ([]models.PriorityOption, error)
  }
  ```

  The Jira adapter implements it from the project's create screen when
  readable (`jiraCreatePriorities` for the project's first configured issue
  type), else the site list (`readJiraPriorities`), bypassing the 10-minute
  cache on demand from the Tracker tab.
- The classifier passed to `MergePriorityMapping` is `jiraPriorityOf`
  (sure), else `priorityAt(rank)` (guessed). Export a small
  `trackerapi.ClassifyJiraPriority(name, rank, n)` for the db layer.
- `priorityFieldFor` (`jira_priority.go:362`) receives the project. When the
  project has a non-empty mapping, it picks with
  `mapping.OptionFor(level, screen option ids)`; no sure option means
  `errGuessedPriority`, a typed error carrying the writable levels.
  - `UpdateIssue` (`jira.go:378`) returns that error, so a queued activity
    fails with the French message (FR-B8).
  - `CreateIssue` (`jira.go:277`) and `setPriorityAfterCreate` drop the field
    and record a notice (FR-B7). The notice reaches the caller through a
    `PriorityNotice string` on the returned `models.Task` (JSON
    `priorityNotice,omitempty`, never stored), which `create_task` and the web
    creation toast show.
  - An empty mapping keeps today's path exactly (FR-B10).

### Refusal before the local write

- `db.CheckPriorityWritable(proj *models.Project, level models.Priority) error`
  returns nil for a non-Jira project, an empty mapping or a writable level, and
  otherwise the French sentence: « La priorité « high » n'est pas associée avec
  certitude à une priorité Jira de ce projet. Priorités acceptées : urgent,
  medium. Confirmez la correspondance dans l'onglet Tracker des réglages du
  projet. »
- Called at the top of `UpdateTaskBy` (`db.go:2791`) when `req.Priority` is set
  and differs from the stored one, before `updateTaskBy` writes anything. Both
  callers, the web handler (`handlers.go:3235`) and MCP `update_task`
  (`taskmcp/server.go:525`), therefore refuse alike. The handler maps the
  typed error to 422 with the message; the bulk action in `ListView.tsx`
  (`handleBulkPriority`) continues on a refused ticket and lists the refused
  ones in its toast instead of stopping at the first error.
- `CreateTaskAs` passes the priority through; the adapter decides (FR-B7).

### Discovery

- `db.RefreshPriorityMapping(ctx, projectID) (string, error)`: reads the
  scheme through `PrioritySchemeReader`, merges, stores, returns a French note
  ("4 priorités, 2 devinées").
- Called from the project sync next to `SyncProjectBoardColumns`
  (`db.go:4409`), under `describesProject`, as step "Priorités".
- `POST /api/projects/{id}/priority-mapping/refresh` (project admin rights,
  like the other project settings) runs it and returns the project.

### Web

- `ProjectModal.tsx`, Tracker tab, Jira only: a "Correspondance des priorités"
  table. Columns: option name, level select, a "devinée" badge with a confirm
  button, and per level a radio for the preferred option when several share it.
  A line for each level no sure option carries ("non inscriptible"). A refresh
  button calls the endpoint. Saved with the project.
- The priority chips and the card form show the server's 422 message in the
  existing error toast; no new component.

## Part C - Axis custom fields (after the owner's confirmation)

### Data

`models.Project` gains `EpicAxisFields`:

```go
type EpicAxisFields struct {
    Priority *EpicAxisField `json:"priority,omitempty"`
    Quarter  *EpicAxisField `json:"quarter,omitempty"`
}

type EpicAxisField struct {
    ID      string            `json:"id"`      // custom field id, read from editmeta
    Name    string            `json:"name"`    // display only
    Kind    string            `json:"kind"`    // "select" or "cascade"
    Options map[string]string `json:"options"` // axis value -> option id, "parent/child" for a cascade
    Manual  []string          `json:"manual,omitempty"` // axis values set by hand
}
```

Migration 41 `projects.epic_axis_fields` (`TEXT NOT NULL DEFAULT '{}'`).

### Tracker

- An optional `EpicFieldManager` interface on the Jira adapter:
  - `EpicAxisFieldCandidates(ctx, project, epicKey)`: the single and cascading
    select custom fields of `editmeta` of that epic, with their options
    (cascade children included);
  - `SetEpicAxisField(ctx, project, epicKey, field, optionPath string)` (empty
    path clears it);
  - `ListEpics` requests the mapped field ids along with the labels when the
    project has any, and exposes their raw option ids on the epic
    (`models.Task.AxisFieldValues map[string]string`, not stored).
- Schema type detection uses the `editmeta` schema `custom` key suffix
  (`:select`, `:cascadingselect`), never a field id or name.

### Domain

- Deduction functions, pure and table-tested on synthetic options:
  `deducePriorityOptions` (label equals `p0`..`p3`, case ignored) and
  `deduceQuarterOptions` (`YYYY - Qn`, `YYYY-Qn`, `YYYY Qn`; cascade: year
  parent with `Qn` or `YYYY - Qn` child). Fill holes only, skip `Manual`.
- The axis pushes write the label (part A) then, when a field is mapped, the
  field. A missing quarter is deduced from a fresh candidate read and stored;
  a still-missing value adds a sentence to the activity note and writes no
  field.
- The import and `pendingAxisPushes` read the mapped field first through the
  reverse option map, then the label.

### Web

- In the Roadmap section of the Tracker tab: for the priority and the quarter,
  a field picker loaded from `GET /api/projects/{id}/epic-axis-fields`
  (disabled with a message when the project has no epic), then the editable
  option map.

## Target files

| Part | File | Change |
| --- | --- | --- |
| A | `internal/models/models.go` | `EpicAxisPrefixes` on `Project` and `UpdateProjectRequest` |
| A | `internal/db/migrations.go` | migration 39 |
| A | `internal/db/db.go` | project SELECT/INSERT/UPDATE, cleaning on save |
| A | `internal/db/macroaxes.go` | prefix-aware read, write, cleaning |
| A | `internal/db/macrolabels.go` | per-project protected prefixes |
| A | `internal/db/macrohorizons.go` | import and pending pushes with prefixes |
| A | `web/src/types/index.ts`, `web/src/lib/roadmap.ts` | per-project prefixes |
| A | `web/src/components/ProjectModal.tsx`, `EpicLabelEditor.tsx`, `RoadmapView.tsx` | settings section, callers |
| A | `web/src/locales/projectSettings.ts` | strings |
| B | `internal/models/models.go`, new `internal/models/prioritymapping.go` | mapping type and pure functions |
| B | `internal/db/migrations.go` | migration 40 |
| B | `internal/tracker/ticketing.go` | `PrioritySchemeReader` |
| B | `internal/trackerapi/jira_priority.go`, `jira.go` | scheme reader, mapped write, typed refusal, creation notice |
| B | `internal/db/db.go` | `CheckPriorityWritable` in `UpdateTaskBy`, discovery in sync |
| B | new `internal/db/prioritymapping.go` | `RefreshPriorityMapping` |
| B | `internal/handlers/handlers.go` (or the project handler file) | refresh endpoint, 422 mapping |
| B | `internal/taskmcp/server.go` | surface `priorityNotice` on `create_task` |
| B | `web/src/components/ProjectModal.tsx`, `ListView.tsx` | mapping table, bulk refusal report |
| C | `internal/models/models.go`, `internal/db/migrations.go` | `EpicAxisFields`, migration 41 |
| C | `internal/trackerapi/jira_epicfields.go` (new) | candidates, write, read |
| C | `internal/db/macroaxes.go`, `macrohorizons.go` | field write and read-back |
| C | `web/src/components/ProjectModal.tsx` | field pickers and maps |
| all | `CHANGELOG.md` | one `Unreleased` line per part |
| all | `docs/` | a short section on axis prefixes and the priority mapping in the roadmap or tracker doc, if one covers the axes |

## Rejected alternatives

- **Storing prefixes as workstation overrides.** A label convention is a team
  decision about a tracker; two workstations writing different prefixes would
  fight over the same epics.
- **Migrating labels on a prefix change.** Rewriting every epic is a bulk
  tracker write nobody confirmed; the pending pushes already offer it one epic
  at a time.
- **Refusing a guessed priority in the Jira adapter only.** The tracker write
  is queued after the local write, so the card would show a level the tracker
  never received and the next sync would undo it. The refusal sits before the
  local write; the adapter check stays as a backstop for a mapping changed in
  between.
- **Keeping the mapping in the in-memory cache.** It must survive restarts and
  carry hand-set lines; the cache stays for the site list only.
- **Deducing the epic priority option by rank.** Rejected by the clarification:
  an epic priority field's order says nothing reliable about `p0` to `p3`.
- **A field id or name in the code (part C).** Forbidden by the guardrails;
  everything comes from `editmeta` and is stored on the project.

## Test plan

- `internal/db/macroaxes_test.go`: table tests of the cleaning (FR-A2), of
  reading and writing under custom prefixes, the bare quarter, values outside
  the set, two values under one prefix.
- `internal/db/macrolabels_test.go`: protection under custom prefixes, a
  former prefix accepted as a free label, `roadmap:` and bare quarters still
  refused.
- `internal/db/macrohorizons_test.go`: import and pending pushes after a prefix
  change (US4), push removing only the current prefix (US3.4), a foreign epic
  under the declaring project's prefixes.
- `internal/db/migrations_test.go` and the rewind helpers: the new columns, on
  SQLite and PostgreSQL (`SECTILE_TEST_POSTGRES_DSN` on a throwaway database).
- `internal/models/prioritymapping_test.go`: merge (new, gone, untouched,
  manual), `Writable`, `OptionFor` with and without a preferred option and a
  restricted screen.
- `internal/trackerapi/jira_priority_test.go`: classification of Atlassian's
  default, the Server scheme, French names (all sure) and `P1`..`P4` (guessed);
  update refused with the typed error; creation without priority and its
  notice; empty mapping unchanged.
- `internal/db` update tests: `UpdateTaskBy` refuses before any local write
  (row unchanged, no activity queued), for the web and MCP callers;
  `CheckPriorityWritable` on non-Jira projects.
- `internal/handlers`: 422 with the message; the refresh endpoint's rights.
- Web `node --test` (`web/tests/*.test.mjs`): `epicAxisLabelPrefixes`,
  `isEpicAxisLabel` with a project, the client prefix cleaning, the bulk
  refusal report helper.
- Part C: deduction table tests on synthetic options; Jira adapter tests with
  an `httptest` site serving a synthetic `editmeta`, whose field ids are built
  by the test (`cf-priority`) and never follow the `customfield_<n>` pattern;
  import field-first and
  pending push on disagreement; a project with no field issues no extra
  request (request counter on the fake site).
- Gates: `go test ./...`, `go vet ./...`, `cd web && npx tsc --noEmit`,
  `npx oxlint`, `node --test web/tests`.
