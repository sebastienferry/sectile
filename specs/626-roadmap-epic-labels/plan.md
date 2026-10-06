# Plan #626 - Roadmap: keep and show an epic's labels

Implementation plan for `spec.md`. The behaviour lives there; this file says
where it goes and how.

## Stack and surfaces

- Server: Go, `internal/db` (schema, macros, tracker queue), `internal/handlers`
  (macro routes), `internal/models`.
- Web: React and TypeScript, `web/src/components/RoadmapView.tsx`,
  `web/src/lib/roadmap.ts`, `web/src/context/AppContext.tsx`,
  `web/src/types/index.ts`, `web/src/locales/planning.ts`.
- Desktop: nothing of its own; it serves the same web build.
- No tracker adapter changes: `JiraAdapter.ListEpics` already returns the
  labels through `j.search`, and `UpdateIssue` already takes `Labels` and
  `RemovedLabels`.

## Data model

### Schema

Migration 35 (33 when specified; #621 and #627 took 33 and 34 first) in `internal/db/migrations.go`, never in the baseline
`CREATE TABLE` of `ensureMacrosTable`:

```go
{
    version:    35,
    name:       "macros.labels",
    statements: []string{"ALTER TABLE macros ADD COLUMN labels TEXT NOT NULL DEFAULT '[]';"},
},
```

The column holds a JSON array of strings, as `todos` does. Existing rows read
`[]` (US1.5).

### Model

`models.MacroMeta` gains:

```go
// Labels are the epic's labels as the tracker returns them, horizon labels
// included. Only the epic sync and a successful label edit write them.
Labels []string `json:"labels"`
```

Always serialised as an array, never `null` (FR3). `web/src/types/index.ts`
`MacroMeta` gains `labels: string[]`.

### Reads and writes

In `internal/db/macros.go`:

- `parseMacroLabels(raw string) []string`, next to `parseMacroTodos`: empty
  or invalid JSON reads as an empty slice.
- Every `SELECT` that builds a `MacroMeta` selects and parses `labels`:
  `GetProjectMacros` (`:141`), the locked read in `saveMacroMetaFull`
  (`:296`), the read in `RefineMacro` (`:1228`, `:1232`), and the reads of the
  macro move (`:935`, `:982`).
- `saveMacroMetaFull` gains a trailing `labels *[]string` parameter. `nil`
  leaves the stored list as it is (FR2); a non-nil value replaces it after
  trimming blanks away, order kept. Its `INSERT ... ON CONFLICT` (`:354`)
  writes the column. Every existing caller passes `nil`.
- The macro move (`:991`) copies `labels` into the target project (FR4).
- The milestone upsert in `GetProjectMacros` (`:128`) does not touch `labels`.

### Axis prefixes

In `internal/db/macros.go`, beside `RoadmapLabelPrefix`:

```go
// macroAxisPrefixes are the label prefixes the roadmap owns on an epic. A
// label under one of them is written by its own control, never as a free label.
var macroAxisPrefixes = []string{RoadmapLabelPrefix}

func IsMacroAxisLabel(label string) bool
```

The match lower-cases, trims and strips a leading `#`, as `HorizonFromLabels`
does. #627 and #635 extend this list.

## Sync

`ImportMacroHorizons` (`internal/db/macrohorizons.go:209`) passes the epic's
labels to `saveMacroMetaFull`:

```go
labels := append([]string{}, epic.Labels...) // non-nil: an epic without labels clears them
```

Nothing else in the sync changes. `db.go:4463` already limits it to trackers
that support `CapEpic`, which is how FR12 holds for the sync.

## Label edit

### Endpoint

`POST /api/projects/{id}/macros/{key}/labels`, a new sub-route handled in the
macro block of `internal/handlers/handlers.go`, beside `refine` and `slicing`
(`:1269`, `:1294`).

Request:

```json
{ "add": ["client-acme"], "remove": ["domain-billing"] }
```

Responses:

- `202 Accepted` with `{ "activity": <TaskActivity>, "labelNote": "..." }`
  when the write is queued.
- `400` with `{ "error": "<French sentence>" }` for every refusal of FR11.
  No activity is created.

The handler calls `db.ValidateMacroLabelEdit`, then `EnqueueTrackerOp` with
the acting context, as the horizon enqueue does (`handlers.go:1363`).

### Validation

`func (d *DB) ValidateMacroLabelEdit(projectID, key string, add, remove []string) (cleanAdd, cleanRemove []string, err error)`
in a new `internal/db/macrolabels.go`:

1. Trim each label; refuse an empty one ("un label vide ne peut pas être
   posé") and one containing whitespace ("un label ne peut pas contenir
   d'espace : « %s »"), since Jira refuses them.
2. Refuse an axis label on either side ("« %s » appartient à la roadmap : il se
   règle depuis les onglets d'horizon").
3. Drop from `add` what the stored list already carries, ignoring case, and
   from `remove` what it does not carry; deduplicate both.
4. Refuse when both lists end empty ("rien à modifier sur les labels de %s").
5. Call `macroTracker(projectID, key)` and return its refusal as is: it already
   covers milestones, local keys, epics of another project and trackers
   without label support, with the sentences US4.10 expects.

### Tracker activity

In `internal/db/trackerops.go`:

- `TrackerOpEpicLabels TrackerOpKind = "epic_labels"`.
- `TrackerOp` gains `Labels []string` and `RemovedLabels []string`.
- Enqueue description: action "Labels de <key>", summary "Labels de <key> en
  file d'attente", one step per added (`+ label`) and removed (`- label`)
  label.
- `runEpicLabelsOp` calls `PushMacroLabels`.

`func (d *DB) PushMacroLabels(ctx, projectID, key string, add, remove []string) (string, error)`
in `internal/db/macrolabels.go`:

1. `macroTracker` again (the state may have changed since the enqueue).
2. `ts.UpdateIssue` with `Labels: add, RemovedLabels: remove` only, under
   `macroWriteTimeout`, as `PushMacroHorizonLabel` does (FR9, US4.6).
3. On success only, apply the edit to the stored list inside
   `saveMacroMetaFull`'s transaction: remove the `remove` labels (ignoring
   case), append the missing `add` labels (FR10). Return "labels de %s mis à
   jour (+n, -m)".
4. On failure, return the error; nothing local is written and nothing is
   recorded as pending (US4.5). The existing activity poll in `AppContext`
   (`:1861`) raises the failure toast.

## Web

### Helpers, `web/src/lib/roadmap.ts`

Pure functions, unit-tested:

- `EPIC_AXIS_LABEL_PREFIXES = ['roadmap:']`, mirroring the server list, with a
  comment pointing at `macroAxisPrefixes`.
- `isEpicAxisLabel(label)`, same normalisation as the server.
- `freeEpicLabels(meta?: MacroMeta): string[]`.
- `epicLabelInventory(rows: EpicRow[]): { label: string; count: number }[]`,
  one entry per free label (case-insensitive, first spelling kept), sorted by
  label.
- `matchesEpicLabels(row, selected: string[]): boolean`, OR semantics, true
  when `selected` is empty.
- `pruneSelectedLabels(selected, inventory): string[]`.
- `canEditEpicLabels(project, row): boolean`: the project's tracker is Jira,
  the key is not `M-<n>`, and `belongsToProjectKey` holds. The server stays
  the authority.

### `RoadmapView.tsx`

- State `selectedLabels: string[]`, not persisted (US3.7).
- `rows` memo (`:383`): after `showClosed` and search, compute the inventory
  from that list, then apply `matchesEpicLabels`. The tab `counts` (`:401`)
  already derive from `rows`, so they follow (US3.2).
- An effect prunes `selectedLabels` whenever the inventory changes (US3.6).
- Toolbar, beside the closed-epics toggle (`:826`): a "Labels" button opening
  a popover of checkboxes with counts, only when the inventory is not empty
  (US3.5). Each selected label joins `activeFilterChips` (`:271`) with a clear
  control (US3.8).
- `renderMacroRow` (`:556`): badges after the status badge, styled like the
  squad chip; in the condensed shape at most two plus `+n` with a `title`
  listing the rest (FR5).
- Panel: a "Labels" block under the header, before the display-mode content.
  Chips with a remove control and an input with a `datalist` of the
  inventory's labels minus the epic's own when `canEditEpicLabels`; read-only
  chips plus the refusal sentence otherwise, shown only when the epic has free
  labels or can be edited.

### `AppContext.tsx`

`editMacroLabels(projectId, key, patch: { add?: string[]; remove?: string[] }): Promise<boolean>`
posts to the new endpoint. On `400` it raises an error toast with the server's
sentence and returns `false`; on `202` it returns `true` and lets the activity
poll and the roadmap's macro reload (`RoadmapView.tsx:366`, keyed on
`activeJobCount`) bring the new list. Client-side checks mirror FR11 so that
the obvious refusals never leave the browser.

### Strings, `web/src/locales/planning.ts`

French and English, with their types: filter button, filter empty state,
"+n", panel block title, add placeholder, remove control label, refusal for an
axis label, an empty label and a label with a space.

## Changelog

Under `## [Unreleased]`, `### Added`:

> Roadmap: on Jira projects, an epic's tracker labels now show on its row, can
> filter the roadmap, and can be added or removed from its panel; the
> `roadmap:` horizon labels stay managed by the horizon tabs (#626).

## Tests

Go, `internal/db`:

- `macrohorizons_test.go`: import records labels in order, horizon label
  included; a later import drops a removed label; an epic without labels
  clears them; a horizon or framing save keeps them.
- `macrolabels_test.go` (new): each refusal of `ValidateMacroLabelEdit`;
  `PushMacroLabels` sends only the delta to a fake tracker and updates the
  stored list; a failing fake tracker leaves the list unchanged.
- `macros_test.go`: the move keeps the labels.
- `postgres_macro_test.go`: the round trip of the column on PostgreSQL
  (needs `SECTILE_TEST_POSTGRES_DSN`).
- `migrations_test.go`: `dropCredentialAccountColumn` also runs
  `ALTER TABLE macros DROP COLUMN labels`, so the rewind fixtures stay valid;
  `TestMigrationsAreNumberedInOrder` covers numbering.

Go, `internal/handlers`: the endpoint returns `202` and queues one
`epic_labels` activity for a valid edit, `400` and no activity for an axis
label.

Web, `web/tests/roadmapEpicLabels.test.mjs` (new): `isEpicAxisLabel`,
`freeEpicLabels`, `epicLabelInventory`, `matchesEpicLabels`,
`pruneSelectedLabels`, `canEditEpicLabels`.

Web, `web/tests/roadmap-epic-labels.browser.mjs` (added at review, opt-in
like the other `*.browser.mjs`): the real `RoadmapView` with a mocked context
checks the badges, the axis labels left out, the OR filter and its chips, the
editor's refusals before anything is sent, the add and remove calls, and the
read-only epic.

Manual, on a Jira project: US2, US3, US4.1, US4.2, US4.10; and a GitHub
project's roadmap unchanged (AC4).

## Deviations during implementation

- `macroAxisPrefixes` and `IsMacroAxisLabel` live in the new
  `internal/db/macrolabels.go`, beside the validation that uses them, rather
  than in `macros.go`.
- `RefineMacro` reads no `MacroMeta` it returns, so it does not select
  `labels`.
- The accepted delta is applied by `applyMacroLabelEdit`, its own transaction
  under the row lock, rather than through `saveMacroMetaFull`, which only
  replaces the list.
- The rewind fixtures that drop the columns of recent migrations
  (`activerun_test.go`, `migrations_test.go`, `specartifacts_test.go`,
  `stagecommits_test.go`) also drop `macros.labels`; `TestPushStageCommitsMigration`
  forgets versions from 32 up, since a database stamped 33 never replays 32.
- `MacroMeta.labels` is optional on the web side, so a client talking to an
  older server reads no labels instead of failing.
- The toolbar filter and the panel editor are their own components,
  `EpicLabelFilter.tsx` and `EpicLabelEditor.tsx`, mounted by `RoadmapView`.
- A queued edit raises an info toast, so the click has visible feedback until
  the activity completes and the roadmap reloads.

- After #627 merged, the protected axes are `roadmap:`, `priority:`,
  `quarter:` and the bare quarter form (`2026-Q3`), which its import reads as
  the epic's quarter, on both the server and the web side.

## Rejected alternatives

- Filtering the horizon labels out before storing: `PendingHorizonPushes`
  and later axes need the real list (clarification, decision 1).
- Reusing the application-wide label filter: it filters tickets on the server,
  and would make epics vanish from the roadmap for a filter set on the board.
- Local-first edits with a pending list: rejected by the owner in round 2.
- Refreshing the edited epic from the tracker after the write: no single-epic
  read exists on the interface, and applying the confirmed delta is exact.
- A `labelsEditable` flag computed by the server on every macro read: it would
  resolve the tracker on each `GET`. Superseded: #627 added such a flag,
  `labelsWritable`, on main while this branch was open, and
  `canEditEpicLabels` now prefers it, keeping the client rule for a server
  that does not send it.
