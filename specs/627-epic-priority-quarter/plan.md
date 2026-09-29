# Plan #627 - Roadmap: give each epic its own priority and quarter

## Stack

Go server (`internal/db`, `internal/handlers`, `internal/models`), SQLite and
PostgreSQL through the numbered migrations, React 19 + TypeScript web app
(`web/`), `node --test` unit tests and Go tests. The tracker writes reuse the
queued tracker operations (`internal/db/trackerops.go`). No agent, desktop or
tracker adapter change: Jira already supports `UpdateIssue` with `Labels` and
`RemovedLabels`, and `GetIssue`.

## Design

### 1. Storage

- **Migration 34** `macros.priority_quarter` (33 was taken by #621 while this
  ticket was in progress) in `internal/db/migrations.go`,
  nowhere else (not in `ensureMacrosTable`, which is the frozen baseline):

  ```sql
  ALTER TABLE macros ADD COLUMN priority TEXT NOT NULL DEFAULT '';
  ALTER TABLE macros ADD COLUMN quarter TEXT NOT NULL DEFAULT '';
  ```

  The baseline creates `macros` before the migrations run
  (`migrations.go:587`), so the ALTER always finds the table.
- `models.MacroMeta` gains:

  ```go
  // Priority is the epic's own priority, "p0" to "p3", "" when none.
  Priority string `json:"priority"`
  // Quarter is the epic's quarter, "2026-Q4", "" when none.
  Quarter string `json:"quarter"`
  // LabelsWritable tells the panel whether Sectile writes the epic axes on the
  // tracker. Computed on read, never stored.
  LabelsWritable bool `json:"labelsWritable"`
  ```

  Stored values are always normalized: lowercase `p0`..`p3`, `YYYY-Qn` with an
  uppercase `Q`.
- Every `SELECT ... FROM macros` that fills a `MacroMeta` reads the two new
  columns (`macros.go`, `GetProjectMacros` and the single-macro read).
- A narrow writer, `SaveMacroAxes(projectID, key string, priority, quarter
  *string) (*models.MacroMeta, error)`, in the style of `saveMacroMetaFull`
  (insert-if-missing, lock the row, merge, update `updated_at`). It leaves
  `saveMacroMetaFull`, `UpdateMacro` and their callers untouched: the handler
  calls it after `UpdateMacro` when either field is present. The macro move
  between projects carries both columns.

### 2. Axis vocabulary (`internal/db/macroaxes.go`, new)

```go
const PriorityLabelPrefix = "priority:"
const QuarterLabelPrefix  = "quarter:"

// NormalizeEpicPriority: "P1", "p1", "priority:p1" -> "p1"; "" -> ""; else error.
func NormalizeEpicPriority(v string) (string, error)
// NormalizeQuarter: "2026-Q4", "2026.q4", "2026 Q4" -> "2026-Q4"; "" -> ""; else error.
func NormalizeQuarter(v string) (string, error)
func PriorityLabel(p string) string          // "p1" -> "priority:p1", "" -> ""
func QuarterLabel(q string) string           // "2026-Q4" -> "quarter:2026-q4"
func PriorityFromLabels(labels []string) string
// QuarterFromLabels: the first valid "quarter:" label, else the first valid
// bare "YYYY-Qn" label, else "".
func QuarterFromLabels(labels []string) string
// isQuarterLabel: true for either form, valid or not a quarter is false.
func isQuarterLabel(label string) bool
```

Case is ignored on read, a leading `#` is stripped as `HorizonFromLabels`
does, an invalid value reads as absent. The error messages of the two
normalizers are French, since the handler returns them to the user.

### 3. Pushability

`macroTracker` (`macrohorizons.go:87`) already refuses milestones, epics of
another project and trackers without `CapUpdate`/`CapLabels`, but only once
the queued op runs, as an error. The handler needs the answer before
enqueuing, and the panel needs it before any edit. Add:

```go
// macroKeyLabelable: not a milestone key, and belongsToProject holds.
func macroKeyLabelable(key string, proj *models.Project) bool
// epicLabelsSupported: the tracker supports CapEpic, CapUpdate and CapLabels.
func (d *DB) epicLabelsSupported(proj *models.Project) bool
// MacroLabelsWritable: both, for the handler.
func (d *DB) MacroLabelsWritable(projectID string, key string) bool
```

A macro is writable when both hold. It resolves
the tracker once per project; `GetProjectMacros` computes the tracker support
once and sets `LabelsWritable` on every macro it returns.

### 4. Tracker writes

- Generalize `PushMacroHorizonLabel` into
  `pushMacroLabels(ctx, projectID, key string, added, removed []string) error`
  holding the `macroTracker` call, the timeout and the `UpdateIssue`; the
  horizon push becomes a caller with no behaviour change.
- `PushMacroPriorityLabel(ctx, projectID, key, priority)`: adds
  `PriorityLabel(priority)` (if any), removes the other three of
  `priority:p0..p3`.
- `PushMacroQuarterLabel(ctx, projectID, key, quarter)`: reads the epic with
  `GetIssue` to learn its current labels (the bare form is an open set that
  cannot be listed in advance), removes every label for which
  `isQuarterLabel` holds except the target, adds `QuarterLabel(quarter)` (if
  any). Two tracker calls, both under `macroWriteTimeout`.
- New op kinds in `trackerops.go`:

  ```go
  // TrackerOpEpicPriority mirrors the priority of one epic as a label.
  TrackerOpEpicPriority TrackerOpKind = "epic_priority"
  // TrackerOpEpicQuarter mirrors the quarter of one epic as a label.
  TrackerOpEpicQuarter TrackerOpKind = "epic_quarter"
  ```

  `TrackerOp` gains `Priority string` and `Quarter string`. Activity texts, in
  French like their neighbours: action `Priorité de <key> ➔ <P1|aucune>` /
  `Trimestre de <key> ➔ <2026-Q4|aucun>`, summary `Label de priorité de <key>
  en file d'attente` / `Label de trimestre de <key> en file d'attente`. A
  failure is reported as `<axe> posé(e) dans Sectile mais pas sur <key> : <erreur>`
  (FR7).
- `PendingHorizonPushes` keeps its name and route and widens its test: a macro
  is a candidate when any of horizon, priority, quarter is set, and pending
  when any set axis differs from what its epic labels read
  (`HorizonFromLabels`, `PriorityFromLabels`, `QuarterFromLabels`).
  `PushPendingHorizons` pushes each differing axis of each pending macro and
  names each failure as `<key> (<axe>) : <erreur>`. The toolbar button label
  keeps its meaning ("labels to push"); its tooltip names the three axes. The
  push-all activity keeps its texts, which the web already translates.
- The new activity texts get their English rendering in
  `web/src/locales/operations.ts` (`priorityAction`, `priorityClearAction`,
  `prioritySummary`, the quarter ones and the two "➔ aucun(e)" targets), with
  their samples in `web/tests/activityText.test.mjs`.

### 5. Read-back

`ImportMacroHorizons` reads `PriorityFromLabels` and `QuarterFromLabels` on
each epic and, when non-empty, stores them with `SaveMacroAxes`. An empty
reading keeps the local value (FR8). The summary gains the counts:
`%d macro(s) lue(s) (%d classée(s), %d priorisée(s), %d datée(s), %d terminée(s))`.
The read never writes to the tracker, so a bare quarter label is left as is
(US7.2, US7.3).

### 6. API

`POST|PUT|PATCH /api/projects/{id}/macros[/{key}]` (`handlers.go:1337`)
accepts two more optional fields:

```json
{ "priority": "p1" | "", "quarter": "2026-Q4" | "" }
```

The handler normalizes both (400 with the normalizer's message on an invalid
value), saves, then for each field present enqueues `TrackerOpEpicPriority`
or `TrackerOpEpicQuarter` **only when** `MacroLabelsWritable` holds. The
response keeps its shape `{ macro, epic, labelNote }`; `labelNote` becomes
`"labels en file d'attente"` when something was queued, and
`"conservé dans Sectile, non écrit sur le tracker"` when an axis was saved on
a non-writable macro. No new route: the seeding uses this one per epic.

### 7. Web

- `web/src/types/index.ts`: `MacroMeta` gains `priority?: EpicPriority | ''`,
  `quarter?: string`, `labelsWritable?: boolean`, and
  `export type EpicPriority = 'p0' | 'p1' | 'p2' | 'p3'`.
- `web/src/lib/epicAxes.ts` (new, pure, unit-tested):

  ```ts
  export const EPIC_PRIORITIES: EpicPriority[]           // p0..p3
  export const EPIC_PRIORITY_LEVEL: Record<EpicPriority, Priority> // p0->urgent .. p3->low
  export function normalizeQuarter(input: string): string | null  // null = invalid, '' = clear
  export function titleAxes(title: string): { priority: EpicPriority | ''; quarter: string }
  export interface SeedLine { key: string; title: string; priority?: EpicPriority; quarter?: string }
  export function seedProposals(rows: EpicRow[]): SeedLine[]
  export type PrioritySort = 'backlog' | 'priority-desc' | 'priority-asc'
  export function sortByPriority(rows: EpicRow[], sort: PrioritySort): EpicRow[] // stable
  export type PriorityFilter = EpicPriority | 'none' | null
  export function matchesPriority(row: EpicRow, filter: PriorityFilter): boolean
  ```

  `titleAxes` regexes: priority `/(?:^|[^\p{L}\p{N}])\[?P([0-3])\]?(?=$|[^\p{L}\p{N}])/iu`,
  quarter `/(?:^|[^\p{L}\p{N}.])(\d{4})[.\- ]Q([1-4])(?=$|[^\p{L}\p{N}])/iu`;
  first match of each wins. `normalizeQuarter` accepts the same three
  separators. The server's `NormalizeQuarter` accepts exactly the same forms,
  so the client check never lets through what the server refuses.
- `web/src/lib/roadmap.ts`: `EpicRow.priority` becomes `EpicPriority | ''`
  read from `meta?.priority`; the loop at `:213` is removed. `EpicRow` gains
  `quarter: string` from `meta?.quarter`.
- `web/src/components/RoadmapView.tsx`:
  - Row badges (`:560`, `:658`): `PRIORITY_META[EPIC_PRIORITY_LEVEL[p]]` and
    the label `P0`..`P3`; a muted "no priority" badge when empty.
  - Panel: a priority segmented control (P0, P1, P2, P3, clear) and a quarter
    text field with a clear button, both next to the horizon control. The
    quarter is validated with `normalizeQuarter` on Enter or blur, the inline
    error shows under the field, and the field shows the normalized value
    after save. When `meta.labelsWritable === false`, one muted line under
    both controls says the values stay in Sectile.
  - Both call the existing `saveMacroMeta(projectId, key, patch)`
    (`AppContext.tsx`), whose patch type gains `priority` and `quarter`, and
    which takes an optional `{ quiet: true }` so that the seeding reports its
    refusals once instead of one toast per epic. The handler always returns
    `labelsWritable` on the saved macro, since the client replaces its copy
    with it.
  - Toolbar: a priority filter select and a sort select, local `useState`, not
    persisted. The filter applies in the same place as `showClosed` and the
    search (`:385`), so the tab counts follow; the sort applies to the list of
    the current tab (`:427`) after the filter. An active filter adds a chip to
    `activeFilterChips`.
  - Seeding: a toolbar button opens a modal built from `seedProposals(allRows)`
    (checkbox per proposed value, all ticked). Confirm calls `saveMacroMeta`
    once per epic with the ticked axes, sequentially, collects the refused
    saves by key, refreshes the macros, and shows a toast with the count and
    the failures. Tracker refusals arrive later in the activities, as for any
    queued write.
- `web/src/locales/planning.ts` (roadmap section, fr and en): priority names
  `P0`..`P3`, "no priority", quarter field label and placeholder, invalid
  quarter message, "kept in Sectile" line, filter and sort labels, seeding
  button, modal title, empty state, confirm, cancel, result toast.

### 8. Interplay with #626

If #626 lands first with a list of protected axis prefixes, add `priority:`
and `quarter:` to it, and treat bare `YYYY-Qn` labels as axis labels there.
If #627 lands first, #626 inherits that duty; the implementation leaves a
comment next to the prefix constants saying so.

## Rejected alternatives

- **Deriving the values from stored labels only** (no columns): would work for
  Jira but leaves GitHub milestones and local projects without a place to
  keep them, against decision 2 of the clarification.
- **A server-side seeding endpoint**: the titles and current values are
  already on the client, and each write must go through the same path as a
  panel edit anyway; a second path would duplicate validation and queueing.
- **Enqueuing an op for non-writable macros and letting it fail**, as the
  horizon does today for milestones: it produces a failed activity per edit
  for a case that is expected, not an error.
- **Extending `saveMacroMetaFull` with two more positional pointers**: eleven
  positional pointers across a dozen callers; a narrow writer is safer.

## Target files

- `internal/db/migrations.go`, `internal/db/migrations_test.go` (rewind
  helpers drop the two columns where they rewind before 33)
- `internal/models/models.go`
- `internal/db/macros.go`, `internal/db/macroaxes.go` (new),
  `internal/db/macroaxes_test.go` (new)
- `internal/db/macrohorizons.go`, `internal/db/macrohorizons_test.go`
- `internal/db/trackerops.go`
- `internal/handlers/handlers.go`, its macro handler tests
- `web/src/types/index.ts`, `web/src/lib/epicAxes.ts` (new),
  `web/src/lib/roadmap.ts`, `web/src/components/RoadmapView.tsx`,
  `web/src/context/AppContext.tsx`, `web/src/locales/planning.ts`
- `web/tests/epicAxes.test.mjs` (new), `web/tests/roadmap-epic-axes.browser.mjs`
  (new, opt-in Playwright test of the panel, filter, sort and seeding),
  `web/src/locales/operations.ts` and `web/tests/activityText.test.mjs`
- `CHANGELOG.md`, `docs/API_AND_DATA_SPEC.md` (the activity queue list; the
  document describes no macro endpoint, and `docs/USER_GUIDE.md` has no
  roadmap section to extend)

## Risks

- **Rewind tests.** A new `ADD COLUMN` migration breaks the db tests that
  rewind the schema unless the helpers drop the new columns; run the full
  `internal/db` suite on SQLite and PostgreSQL.
- **Empty column after upgrade.** Expected (AC2); the changelog line and the
  seeding button make it explicit.
- **Quarter push reads before writing.** A label added on the tracker between
  the read and the write survives; acceptable, the next push corrects it.
