# Plan #636 - Roadmap: smaller gaps found in the Taskativ comparison

Implementation plan for `spec.md`. The behaviour lives there; this file says
where it goes and how.

## Stack and surfaces

- Web, row 2: `web/src/context/AppContext.tsx` (`buildTaskQuery`), a small
  pure helper in `web/src/lib/` with its node test.
- Server, row 47: `internal/db/macrotodosmirror.go` (generalised to two parts
  of a macro), `internal/db/macros.go` (`UpdateMacro` schedules the framing
  copy), `internal/db/trackerops.go` (new queued kind),
  `internal/db/migrations.go` (columns), `internal/models/models.go` (status
  field), `internal/handlers/handlers.go` (bulk marker, republish route).
- Tracker: nothing new. `tracker.MarkedCommentWriter` and
  `JiraAdapter.UpsertMarkedComment` (#663) already take the marker and its
  value per call.
- Web, row 47: `web/src/components/RoadmapView.tsx` (status line under the
  framing), `web/src/context/AppContext.tsx` (republish call),
  `web/src/types/index.ts`, `web/src/locales/planning.ts` and the
  notification strings beside `todosRepublished`.
- Desktop: nothing of its own; it serves the same web build.
- Docs: `CHANGELOG.md`; ADR 0046 gains a short section saying the framing of
  a Jira epic is copied the same way (no new ADR: the decision is the same
  one, applied to a second field).

No new child process (NFR2). No change to story creation, to the todos copy
body, or to the GitHub milestone description write.

## Row 2 - Timeline search

```ts
// web/src/lib/taskQuery.ts
/** Views that filter their own rows on complete data, so the server search
 *  must not truncate what they read (#636). */
export const SELF_FILTERING_VIEWS: ReadonlySet<ViewMode> = new Set(['roadmap', 'timeline'])
export const sendsServerSearch = (view: ViewMode): boolean => !SELF_FILTERING_VIEWS.has(view)
```

In `buildTaskQuery`:

```ts
if (searchQuery && sendsServerSearch(activeView)) params.append('q', searchQuery)
```

- The parent filter line keeps `activeView !== 'roadmap'` (FR3, out of
  scope): only the search moves to the set.
- The French comment above the line is rewritten in English, since it is
  touched (AGENTS.md), and names both views.
- `activeView` is already a dependency of `buildTaskQuery`, so going from the
  Timeline back to a ticket view changes the query string and refetches with
  the search (FR2). Going between Roadmap and Timeline leaves the string equal
  and refetches nothing, which is right.
- `filteredTasks` applies no search client-side: nothing else to change.
  `Header.tsx` is unchanged (the field keeps its value and clear button).
- If `ViewMode` cannot be imported into `lib/` without a cycle, type the set
  as `ReadonlySet<string>`.

## Row 47 - Framing copy

### Generalising the copy machinery

`macrotodosmirror.go` copies one field. Rather than duplicating its ~650
lines, introduce a descriptor of the copied part and pass it through the
existing functions:

```go
// macroCopyPart is one field of a macro Sectile copies on its tracker, one
// way: the todos (#663) or the framing (#636).
type macroCopyPart struct {
	name    string        // "todos" | "framing", column prefix and timer key
	marker  string        // Jira comment property
	opKind  TrackerOpKind // queued write
	// github is true when a milestone description can carry the part.
	github  bool
	render  func(kind string, m *models.MacroMeta) string
	count   func(m *models.MacroMeta) int // 0 is "nothing to copy"
	written func(kind string, n int, key string) string
}

var (
	todosCopy   = macroCopyPart{name: "todos", marker: "sectile.macroTodos", opKind: TrackerOpEpicTodos, github: true, ...}
	framingCopy = macroCopyPart{name: "framing", marker: "sectile.macroFraming", opKind: TrackerOpEpicFraming, github: false, ...}
)
```

- `todosMirrorScope.eligibility(key)` becomes `eligibility(part, key)`. For a
  part with `github: false`, the `githubMilestoneMacros(proj)` case returns
  none with "un milestone GitHub ne prend pas de commentaire : le cadrage
  reste dans Sectile" (D1). GitLab, local, foreign (`isForeignMacro`,
  ADR 0043) and `M-<n>` keep their reasons, reworded per part where the
  sentence names the list ("la liste" / "le cadrage").
- `readTodosMirrorState`, `recordTodosMirrorSuccess`,
  `recordTodosMirrorFailure`, `todosMirrorStatus`, `scheduleTodosMirror`,
  `PushMacroTodosMirror` and `TodosMirrorRefusal` take the part and read or
  write `<part.name>_mirror_*` columns. Column names come from the two
  constant parts only, never from input, so the formatted SQL stays safe.
- The timer key becomes `projectID + "\x00" + key + "\x00" + part.name`, so a
  todos save does not cancel a pending framing write and the reverse.
- The exported names used outside the file (`PushMacroTodosMirror`,
  `TodosMirrorRefusal`) keep thin wrappers for the todos part, plus
  `PushMacroFramingMirror` and `FramingMirrorRefusal`, so callers and #663's
  tests do not churn.
- The todos behaviour is unchanged: same marker, same body, same columns, and
  #663's tests pass untouched. That is the regression gate of the refactor.

If the generalisation turns out to touch more than the functions listed, the
fallback is a separate `macroframingmirror.go` reusing `retryTransient`,
`todosMirrorHash` and `UpsertMarkedComment` directly; record the choice in
the commit message.

### Data

Next free migration number on `main` (44 at the time of writing; check
`origin/main` before numbering and renumber if one landed), in
`internal/db/migrations.go` only, never in the frozen baseline:

```sql
ALTER TABLE macros ADD COLUMN framing_mirror_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE macros ADD COLUMN framing_mirror_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE macros ADD COLUMN framing_mirror_error TEXT NOT NULL DEFAULT '';
ALTER TABLE macros ADD COLUMN framing_mirror_credential TEXT NOT NULL DEFAULT '';
ALTER TABLE macros ADD COLUMN framing_mirror_at TIMESTAMP NULL;
```

Same meaning as the `todos_mirror_*` columns of migration 43. Written only by
the copy job, never part of `saveMacroMetaKeys`' upsert.

Known fallout, same commit: the rewind helpers of the db tests and the
forget-and-replay fixtures must know the new columns (see #663's commit for
the exact helpers it touched); run the PostgreSQL suite against a throwaway
DSN.

`models.MacroMeta` gains, beside `TodosMirror`:

```go
// FramingMirror is where the framing is copied on the tracker and how that
// copy stands (#636). Same shape as TodosMirror; its kind is never
// MacroTodosMirrorGithubDescription.
FramingMirror *MacroTodosMirror `json:"framingMirror,omitempty"`
```

`fillTodosMirror` fills both, so `GetProjectMacros`, the PUT answer and
`GetMacro` (MCP `get_macro`) carry it with no further change.

### Rendering (FR8, FR9)

```go
// renderFramingMirror renders the body of the framing comment of an epic,
// within the size budget.
func renderFramingMirror(framing string) string
```

```markdown
### 🧭 [Sectile] Cadrage

<the framing Markdown as saved, trimmed>

_Cadrage tenu dans Sectile : une modification faite ici est remplacée à la prochaine mise à jour._
```

- Empty framing: heading plus "_Aucun cadrage pour l'instant._" (D6), only
  ever written over an existing comment (FR11).
- Budget: `todosMirrorBudget` (30,000 bytes). Over budget: cut at the last
  line break before the budget, minus room for the closing lines, and add
  "_… cadrage tronqué, texte complet dans Sectile._" before the footer.
- No date in the body: the hash must be stable across equal saves.
- `MarkdownToADF` already renders headings, lists, emphasis and code; check
  a framing with a fenced block and a table in a test and accept whatever
  degradation it has today (the stage comments go through it too).

### Scheduling (FR4, FR5)

- `UpdateMacro` calls `d.scheduleMacroCopy(ctx, framingCopy, projectID, key)`
  when `framingComment != nil` and the context is not a bulk edit.
- Bulk marker: `db.WithBulkMacroEdit(ctx)` / `bulkMacroEdit(ctx)`, a context
  value set by the macro PUT handler when `req.Bulk` is true. Today `bulk`
  only gates the roadmap-project axis writes in the handler; the framing
  scheduling lives in the db layer, so it needs the flag there. Prefer this to
  a new `UpdateMacro` parameter, which every caller would have to pass.
- `SaveMacroMeta` (used by `sddslicing.go`) passes `nil` framing and never
  schedules: unchanged.
- Ineligible macros schedule nothing (AC4).

### Queued write (FR7, FR10, FR11)

`TrackerOpEpicFraming TrackerOpKind = "epic_framing"` in `trackerops.go`,
with `EpicKey`, `TaskKey`, `ProjectID` and `Force`. Activity: action
`Cadrage de KEY ➔ tracker`, summary `Recopie du cadrage de KEY en file
d'attente`. Dispatch to `runEpicFramingOp`, mirroring `runEpicTodosOp`, which
calls `PushMacroFramingMirror(ctx, projectID, key, op.Force)`:

1. Re-check eligibility; ineligible returns the reason as an error.
2. Read the current framing (not the one at schedule time) and render.
3. Empty framing with no ref and no hash: "aucun cadrage à recopier sur KEY",
   no tracker call (FR11). This also covers the case where the marker search
   would find a comment: with nothing ever written, Sectile created none it
   remembers, and an empty framing is not a reason to look.
4. Hash equal to `framing_mirror_hash` with a stored ref and not forced:
   "cadrage de KEY déjà à jour", no call.
5. `retryTransient` around `UpsertMarkedComment` with `Marker:
   "sectile.macroFraming"`, `Value: {"macroKey": key}`, `CommentID: ref`.
   The adapter's search by property means the todos comment
   (`sectile.macroTodos`) is never matched (US2.6).
6. Success stores ref, hash, time, clears the error. Failure stores the error
   and the missing-token tracker, and returns "cadrage gardé dans Sectile
   mais pas recopié sur KEY : %w", keeping the credential chain for #645's
   offer.

### Route (FR13, FR14)

`POST /api/projects/{id}/macros/{key}/framing-mirror` in `handlers.go`,
beside `todos-mirror`, same shape: `FramingMirrorRefusal` non-empty answers
`400` with it; otherwise `EnqueueTrackerOp` with `Force: true`, `202` with the
activity. Same `epics` alias as the other sub-routes.

## Web, row 47

- Types: `framingMirror?: MacroTodosMirror | null` on the macro type.
- Extract the todos status line (`RoadmapView.tsx`, the block with
  `data-testid="todos-mirror"`) into a small component taking the status, a
  test id, the republish callback and its strings, and render it twice: under
  the todos as today, and under the framing editor with
  `data-testid="framing-mirror"`. `todosMirrorState` in `lib/roadmap.ts`
  already reads either status; rename it only if it stays readable.
- `republishMacroFraming(projectId, key)` in `AppContext.tsx`, beside
  `republishMacroTodos`, calling the new route, with its own toasts.
- The panel already replaces the macro with the PUT answer (pending status)
  and reloads it when a queued write ends, as for any queued write, so the
  line turns to **Recopié sur KEY** on its own (US4.7). Verify that the
  reload is not filtered on `epic_todos`.
- Strings in `web/src/locales/planning.ts`, French and English:
  `framingMirrorUpToDate` (Recopié sur {key} / Copied to {key}),
  `framingMirrorLocal` (Reste dans Sectile : {reason} / Stays in Sectile:
  {reason}); the pending, failed, republish and add-token strings of the
  todos line are reused. Notification strings `framingRepublished` and
  `framingRepublishRefused` beside the todos ones.

## Rejected alternatives

- **A second ADR**: the decision (one-way copy, comment property, debounce,
  queue) is ADR 0046's; a section there keeps one place to read it.
- **Footer signature as the marker** (round 1): ADF drops hidden markers and a
  visible signature can be edited away; #663 settled on comment properties.
- **Framing in the GitHub milestone block**: refused by the owner (D1).
- **Schedule the framing copy from the handler**: it would skip any future
  caller of `UpdateMacro`; the todos copy is scheduled in the db layer, the
  framing copy follows it.
- **One comment for framing and todos together**: #663 already owns its
  comment; merging would rewrite its body and its tests, and a ticked todo
  would rewrite the framing text.
- **Search on the Timeline**: out of scope; the backlog has its own.

## Test plan

Go (`go test ./internal/...`, sandbox off for httptest, GOCACHE under
`$TMPDIR`):

- Eligibility table for the framing part: Jira own epic (copied), Jira
  foreign epic with and without `RoadmapAxisWrites`, Jira `M-<n>`, GitHub
  milestone (local, "ne prend pas de commentaire"), GitLab, local, tracker
  without marked comments.
- Rendering: heading, body, footer; empty framing; truncation at the budget
  with the closing line; a fenced block survives.
- Scheduling: several framing saves within the delay queue one
  `epic_framing` op signed by the last saver; a todos save and a framing save
  each queue their own op; a save without `framingComment` queues no framing
  op; a bulk-marked context queues none; ineligible macros queue none.
- Job with a fake `MarkedCommentWriter`: create then update by id; stale id
  then found by marker; not found then created; a comment with the todos
  marker is never updated; empty framing with no ref writes nothing; emptied
  framing with a ref rewrites to "Aucun cadrage"; equal hash no-op; forced
  write; transient failure retried then success; permanent failure stored
  with the credential chain; success clears the error.
- #663's existing tests (`macrotodosmirror_test.go`, handler and MCP tests)
  unchanged and green: the regression gate of the refactor.
- Handler: `framing-mirror` route 202 / 400; PUT with `bulk: true` and a
  framing queues nothing.
- `get_macro` answer carries `framingMirror`.
- Migration on SQLite and on PostgreSQL with a throwaway DSN; rewind and
  replay fixtures updated.

Web:

- `web/tests/taskQuery.test.mjs`: `sendsServerSearch` false for `roadmap`
  and `timeline`, true for `board`, `list`, `triage` and the others.
- Browser component test (`*.browser.mjs`, Playwright from
  `desktop/node_modules`): the framing line in its four states and
  **Republier**; the todos line unchanged.
- `tsc` and `oxlint`, with the main checkout's `node_modules` linked when the
  worktree has none; `npx vite build`, restoring `webui/.gitkeep` afterwards.
