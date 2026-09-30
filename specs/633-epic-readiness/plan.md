# Plan #633 - Roadmap: let a person decide an epic's readiness

## Stack

Go server (`internal/db`, `internal/handlers`, `internal/models`), SQLite and
PostgreSQL through the numbered migrations, React 19 + TypeScript web app
(`web/`), `node --test` unit tests, the opt-in Playwright browser tests and Go
tests. The tracker writes reuse the queued tracker operations
(`internal/db/trackerops.go`). No agent, desktop or tracker adapter change:
Jira already supports `UpdateIssue` with `Labels` and `RemovedLabels`.

Everything below extends what #627 put in place for the priority and the
quarter. Part A (sections 1 to 7) does not depend on #628; Part B (section 8)
is implemented once #628 is merged on `main`.

## Part A

### 1. Storage

- **Migration 37** `macros.readiness` in `internal/db/migrations.go`, nowhere
  else (never in `ensureMacrosTable`, the frozen baseline). 37 is the next free
  number today (36 is `task_activities.credential_missing`); renumber if `main`
  takes it first.

  ```sql
  ALTER TABLE macros ADD COLUMN readiness TEXT NOT NULL DEFAULT '';
  ```

- The rewind helpers drop the column wherever they already drop
  `macros.quarter`: `migrations_test.go` (`:70`, `:444`),
  `activerun_test.go:168`, `branchformat_test.go:110`,
  `specartifacts_test.go:129`, `stagecommits_test.go:83`. Check the
  forget-and-replay fixtures too.
- `models.MacroMeta` gains:

  ```go
  // Readiness is the level a person decided, "idea", "shaping" or "ready",
  // "" when nobody decided (#633). The suggestion is the web app's, never stored.
  Readiness string `json:"readiness"`
  ```

- Every `SELECT ... FROM macros` that fills a `MacroMeta` reads the column
  (`macros.go:153`, `:395`, and the move between projects at `:1034` to
  `:1109`, which carries it like `quarter`).
- `SaveMacroAxes` (`macroaxes.go:188`) takes a third pointer,
  `readiness *string`, normalized with `NormalizeReadiness`, read under the
  same row lock and written in the same `UPDATE`. Its two callers (the macro
  handler and `ImportMacroHorizons`) pass it.

### 2. Axis vocabulary (`internal/db/macroaxes.go`)

```go
const ReadinessLabelPrefix = "readiness:"

// ReadinessLevels in funnel order; the index is the rank used on read.
var ReadinessLevels = []string{"idea", "shaping", "ready"}

// NormalizeReadiness: "Ready", "readiness:ready", "#ready" -> "ready";
// "" -> ""; anything else -> error (French message, returned to the user).
func NormalizeReadiness(value string) (string, error)
func ReadinessLabel(level string) string   // "ready" -> "readiness:ready", "" -> ""
func AllReadinessLabels() []string
// ReadinessFromLabels: the most advanced valid readiness label, "" when none.
func ReadinessFromLabels(labels []string) string
```

`cleanLabel` already handles case and `#`. `internal/db/macrolabels.go:26`
adds `ReadinessLabelPrefix` to `macroAxisPrefixes`, which makes
`IsMacroAxisLabel` refuse it in the free-label editor (FR10).

### 3. Tracker writes

- `PushMacroReadinessLabel(ctx, projectID, key, readiness)`, modelled on
  `PushMacroPriorityLabel`: target `ReadinessLabel(readiness)`, removes the
  other entries of `AllReadinessLabels()`. The set is closed, so no `GetIssue`
  read is needed. Failure wording:
  `maturité posée dans Sectile mais pas sur <key> : <erreur>`; success
  `« readiness:ready » posé sur <key>` or
  `labels de maturité retirés de <key>`.

  The French user-facing noun for the axis is "maturité", as the panel label
  reads (see section 6); the code keeps `readiness`.
- New op kind in `trackerops.go`, next to the #627 ones:

  ```go
  // TrackerOpEpicReadiness mirrors the readiness of one epic as a label (#633).
  TrackerOpEpicReadiness TrackerOpKind = "epic_readiness"
  ```

  `TrackerOp` gains `Readiness string`. Activity texts, French like their
  neighbours: action `Maturité de <key> ➔ <Idée|En cadrage|Prête|aucune>`,
  summary `Label de maturité de <key> en file d'attente`. The runner case calls
  `PushMacroReadinessLabel`.
- `pendingAxisPush` gains `readiness bool`; `pendingAxisPushes` includes a
  macro whose `Readiness` is set in `decided`, and flags it when
  `ReadinessFromLabels(labels) != meta.Readiness`. `PushPendingHorizons`
  pushes it and names a failure `<key> (maturité) : <erreur>`. The toolbar
  button tooltip names the four axes.
- `web/src/locales/operations.ts` renders the new action and summary in
  English (`readinessAction`, `readinessClearAction`, `readinessSummary`), with
  samples in `web/tests/activityText.test.mjs`.

### 4. Read-back

`ImportMacroHorizons` reads `ReadinessFromLabels(epic.Labels)` and, when
non-empty, passes it to `SaveMacroAxes`. An empty reading keeps the local
level. The summary gains a count:
`%d macro(s) lue(s) (%d classée(s), %d priorisée(s), %d datée(s), %d jugée(s), %d terminée(s))`.
Check the web's translation of that summary in `operations.ts` and its test.

### 5. API

`POST|PUT|PATCH /api/projects/{id}/macros[/{key}]` (`handlers.go:1337`, request
struct near `:1393`) accepts one more optional field:

```json
{ "readiness": "idea" | "shaping" | "ready" | "" }
```

The handler passes it to `SaveMacroAxes` (400 with the normalizer's message on
an invalid value, nothing saved). `enqueueMacroAxes`
(`internal/handlers/macroaxes.go`) gains a `readiness bool` and enqueues
`TrackerOpEpicReadiness` only when `saved.LabelsWritable`; the `labelNote`
values are unchanged. No new route.

### 6. Web logic

- `web/src/types/index.ts`: `export type EpicReadiness = 'idea' | 'shaping' | 'ready'`;
  `MacroMeta` gains `readiness?: EpicReadiness | ''`.
- `web/src/lib/epicAxes.ts`:

  ```ts
  export const EPIC_READINESS: EpicReadiness[] // idea, shaping, ready (funnel order)
  /** FR2. Pure; reads tasks, meta.description and meta.todos only. */
  export function suggestReadiness(input: {
    childCount: number
    description?: string
    todos?: { done: boolean; storyKey?: string }[]
  }): EpicReadiness
  ```

- `web/src/lib/roadmap.ts`: `EpicRow` gains `readiness: EpicReadiness | ''`
  (from `meta?.readiness`) and `suggestedReadiness: EpicReadiness` (from
  `suggestReadiness({ childCount: children.length, description:
  meta?.description, todos: meta?.todos })`), set next to `suggested` at
  `:235`. `EPIC_AXIS_LABEL_PREFIXES` (`:361`) adds `'readiness:'`, which
  `freeEpicLabels` and the label filter already honour.
- `MATURITY_META` and `maturityOf` are unchanged.

### 7. Web UI

- **Badge** (`RoadmapView.tsx`): a `readinessBadge(row, className)` helper,
  like `priorityBadge`, rendered in the expanded row (`renderMacroRow`, next
  to the maturity badge at `:878`) and in the condensed row. Decided: solid
  colours per level (idea muted, shaping `--status-warn`, ready
  `--status-ok`, reusing the `MATURITY_META` palette). Suggested: the same
  hue, dashed border and transparent background, the name followed by `?`,
  `title={strings.readiness.suggestedTitle}`. Not clickable.
- **Tickets' stage badge**: text becomes `strings.maturity[row.maturity]`
  with the new prefixed values, plus `title={strings.maturityTitle}`.
- **Panel**: a "Maturité" / "Readiness" chip group placed after the priority
  group (`:1718`) in the same flex row, built like it: `aria-pressed` on the
  decided chip, a click on it sends `readiness: ''`, a click elsewhere sends
  the level. When `selected.readiness === ''`, the chip of
  `selected.suggestedReadiness` gets the suggestion style and `?`, with the
  tooltip. No separate clear button: the second click clears (clarification
  decision 2). `saveAxes` (`:754`) and the `saveMacroMeta` patch type in
  `AppContext.tsx` widen to `readiness?: EpicReadiness | ''`. The existing
  `keptLocal` line (`:1795`) already covers the new axis.
- **Strings** (`web/src/locales/planning.ts`, fr and en):
  - `readiness.levels`: `{ idea: 'Idée', shaping: 'En cadrage', ready: 'Prête' }`
    / `{ idea: 'Idea', shaping: 'Shaping', ready: 'Ready' }`;
  - `readiness.label`: 'Maturité' / 'Readiness';
  - `readiness.suggestedTitle`: 'Non décidé : suggestion de Sectile d'après
    le cadrage, le découpage et les tickets' / 'Not decided: Sectile's
    suggestion from the framing, the slicing and the tickets';
  - `readiness.chipTitle`: 'Cliquer à nouveau pour effacer' / 'Click again to
    clear';
  - `maturity` (`:49`, `:472`): 'Tickets : Brouillons / Clarifiés / Spécifiés
    / Prêts' and 'Tickets: Draft / Clarified / Specified / Ready';
  - `maturityTitle`: 'Étape atteinte par le ticket ouvert le moins avancé' /
    'Stage reached by the least advanced open ticket'.
  Run the translation check (`docs/web-translation-checks.md`).

## Part B (after #628)

### 8. Grouping axis and drop

#628 introduces an axis type (priority, quarter), a section builder, folding
keys stored by `roadmapViewPrefs.ts`, a multi-selection and a drop handler
that saves through `saveAxes` and reports once. This ticket adds a third axis
to each:

- Axis type and control: `'readiness'` option, label "Maturité" /
  "Readiness". The stored preference accepts it; any unknown stored value
  falls back to no grouping.
- Sections: `idea`, `shaping`, `ready` always present (count may be 0), then
  `none` ("Non décidé" / "Not decided") only when it holds at least one epic.
  An epic goes by `row.readiness`, never by its suggestion. Folding keys
  `readiness:<level>` and `readiness:none`.
- Drop: target level `''` for `none`; skip epics whose `readiness` already
  equals the target; `saveAxes(key, { readiness: target })` per epic; #628's
  report.
- Hidden tab stays flat, as #628 settles.

If #628's helpers are named or shaped differently from this description, adapt
to them rather than duplicating: the axis must go through the same section,
folding and drop code as priority and quarter.

## Rejected alternatives

- **Storing the suggestion**: it would go stale as tickets and slicing change,
  and a stored suggestion is indistinguishable from a decision once read back.
- **Computing the suggestion on the server**: the rows, their children and the
  slicing are already on the client, where the horizon suggestion lives too.
- **Treating "empty" as `idea`**: it hides the difference between "nobody
  decided" and "someone judged it an idea", which the `?` and the "Not
  decided" section exist to show.
- **Chips on the rows**: rejected in clarification (decision 2); the condensed
  row must stay on one line.
- **Enqueuing a write for non-writable macros and letting it fail**: rejected
  by #627 for the same reason, a failed activity for an expected case.
- **Reading the epic before pushing** (as the quarter does): the readiness set
  is closed, so the labels to remove are known in advance.

## Target files

Part A:

- `internal/db/migrations.go`, the rewind helpers listed in section 1
- `internal/models/models.go`
- `internal/db/macros.go`, `internal/db/macroaxes.go`,
  `internal/db/macroaxes_test.go`, `internal/db/macrolabels.go`
- `internal/db/macrohorizons.go`, `internal/db/macrohorizons_test.go`
- `internal/db/trackerops.go`
- `internal/handlers/handlers.go`, `internal/handlers/macroaxes.go`, their
  tests
- `web/src/types/index.ts`, `web/src/lib/epicAxes.ts`, `web/src/lib/roadmap.ts`,
  `web/src/components/RoadmapView.tsx`, `web/src/context/AppContext.tsx`,
  `web/src/locales/planning.ts`, `web/src/locales/operations.ts`
- `web/tests/epicAxes.test.mjs`, `web/tests/activityText.test.mjs`,
  `web/tests/roadmap-readiness.browser.mjs` (new)
- `CHANGELOG.md`, `docs/API_AND_DATA_SPEC.md` (the activity queue list)

Part B: the files #628 creates or changes for its axis (expected:
`RoadmapView.tsx`, `roadmapViewPrefs.ts`, a grouping helper under `web/src/lib/`
and its tests), plus `planning.ts`.

## Risks

- **Rewind tests.** A new `ADD COLUMN` breaks the db tests that rewind the
  schema unless every helper drops it; run the full `internal/db` suite on
  SQLite and PostgreSQL.
- **Migration number.** Another branch may take 37 first; renumber, and
  recreate the PostgreSQL test database after renumbering.
- **#628 not merged.** Part B cannot start; Part A ships on its own and is
  useful without it (badge, panel, label).
- **French noun.** "Maturité" labels the new axis while the old badge moves to
  "Tickets : ...". Both are visible side by side; the prefix is what keeps
  them apart (AC5). The axis name is an open point of the specification: keep
  every occurrence in `planning.ts` and `operations.ts`, and the activity
  texts in `trackerops.go`, so that a rename at review touches strings only.
