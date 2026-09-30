# Plan #634 - Roadmap: create the stories of a slicing in one gesture

Implementation plan for `spec.md`. The behaviour lives there; this file says
where it goes and how.

## Stack and surfaces

- Server: Go, `internal/db/macros.go` (story creation from a slicing line,
  slicing save), `internal/handlers/handlers.go` (macro routes).
- Web: React and TypeScript, `web/src/components/RoadmapView.tsx`,
  `web/src/context/AppContext.tsx`, `web/src/lib/roadmap.ts`,
  `web/src/types/index.ts`, `web/src/locales/planning.ts`.
- Desktop: nothing of its own; it serves the same web build.
- No migration: the selection is not stored and the origin already is
  (`MacroTodo.SourceKind`, `SourceEntry`).
- No tracker adapter change: each line reuses `createStoryUnder`.
- No MCP change.

## Server

### Per-macro creation lock (FR8)

`DB` gains a keyed lock, one `*sync.Mutex` per `projectID + "\x00" + macroKey`,
held in a `sync.Map` (`macroStoryLocks`). It is taken for the whole of a
single-line creation and of a batch, and never while `d.mu` is held, since the
tracker call inside can be slow. It is released by `defer`.

### Shared per-line core (FR6)

Extract from `CreateStoryFromMacroTodo` everything after the macro lookup into:

```go
// createStoryFromLine checks one slicing line and creates its story. It
// returns errLineAttached, wrapping the key, when the line already carries
// one.
func (d *DB) createStoryFromLine(ctx context.Context, proj *models.Project, macroKey string, todo models.MacroTodo) (*models.Task, string, error)
```

It holds, in this order, the checks that exist today: roadmap-project key,
existing key, target project missing, other tracker instance; then
`createStoryUnder`. The two "already attached" checks return a sentinel
`*macroLineAttachedError{Key, Roadmap bool}` whose `Error()` is the current
French sentence, so the single-line path answers exactly as before (FR12)
while the batch reads it as `skipped`.

`CreateStoryFromMacroTodo` becomes: validate arguments, take the lock, read the
macro, find the line, call `createStoryFromLine`, then `recordLineStoryKey`.
Its signature and error strings are unchanged. A failed record keeps today's
behaviour (the story is returned with the meta read before).

### Recording one key (FR7, FR9)

```go
// recordLineStoryKey sets the story key of one slicing line and saves the
// slicing, re-read under the row lock, so an edit of another line made in the
// meantime is kept. It reports false when the line no longer exists.
func (d *DB) recordLineStoryKey(projectID, macroKey, todoID, storyKey string) (*models.MacroMeta, bool, error)
```

Implemented as an option of `saveMacroMetaFull` rather than a second copy of
its transaction: a new unexported `macroSave` struct is not worth it for one
field, so add a trailing parameter `storyKeys map[string]string` (line id to
key) applied to `current.Todos` after the read and before the write, and pass
`nil` from every existing caller. `recordLineStoryKey` calls it with
`todos == nil` and a one-entry map, then looks for the line in the result.

### Keys survive a stale save (FR7b)

In `saveMacroMetaFull`, when `todos != nil`, index the stored lines by id
before replacing them; for each incoming line with an empty `StoryKey` whose
stored line has one, keep the stored key. Removed lines stay removed.

### Batch (FR5-FR11)

```go
type MacroStoryOutcome struct {
    TodoID   string       `json:"todoId"`
    Status   string       `json:"status"` // created | skipped | failed
    StoryKey string       `json:"storyKey,omitempty"`
    Task     *models.Task `json:"task,omitempty"`
    Notice   string       `json:"notice,omitempty"`
    Error    string       `json:"error,omitempty"`
    Code     string       `json:"code,omitempty"`    // tracker_credential_missing
    Tracker  string       `json:"tracker,omitempty"` // with Code
}

type MacroStoryBatch struct {
    Macro   *models.MacroMeta   `json:"macro"`
    Results []MacroStoryOutcome `json:"results"`
    Created int                 `json:"created"`
    Skipped int                 `json:"skipped"`
    Failed  int                 `json:"failed"`
}

func (d *DB) CreateStoriesFromMacroTodos(ctx context.Context, projectID, macroKey string, todoIDs []string) (*MacroStoryBatch, error)
```

Steps:

1. Trim and de-duplicate `todoIDs`; refuse an empty list
   ("aucune ligne sélectionnée"). Refuse a missing project or macro with the
   single-line sentences. These are the only whole-request errors.
2. Take the per-macro lock.
3. Order: the ids found in the saved slicing, in slicing order, then the ids
   not found, in request order; the latter are `failed`
   "ligne de TODO introuvable".
4. For each found id: re-read the macro (`GetProjectMacros`) and the line, so
   a key recorded by an earlier line or another path is seen; call
   `createStoryFromLine`.
   - `*macroLineAttachedError` → `skipped` with its key.
   - other error → `failed`, `Error` = the message as the single-line path
     words it (`erreur création de story: …` for a tracker failure); when
     `trackerapi.MissingCredentialTracker(err) != ""` set `Code` to
     `handlers.TrackerCredentialMissingCode`'s value and `Tracker`. The code
     constant lives in `handlers`; move it to `trackerapi` (or duplicate the
     string with a comment) so `db` does not import `handlers`.
     `errors.Is(err, trackerapi.ErrNoActingUser)` is the one error that stops
     the batch: it means the request names nobody to write as, which holds
     for every line alike. The batch returns it as a whole error and the
     handler answers `403` as today; no line has been created at that point,
     since it is raised before the first tracker write.
   - success → `recordLineStoryKey`; `created` with key, task and notice. If
     the record fails or the line is gone, append to the notice
     "clé non enregistrée sur la ligne : …" (FR9).
5. Return the batch with the macro read at the end and the three counts.

No activity is recorded (FR11).

### Route (FR5, FR10, FR12)

In `internal/handlers/handlers.go`, beside the `/story` sub-action, add
`parts[3] == "stories"` for `POST` on `macros` and `epics`:

- decode `{ "todoIds": [] }`; `400` on a bad payload;
- call `CreateStoriesFromMacroTodos(h.actingContext(r), …)`;
- a returned error goes through `writeTrackerError(w, 400, err)`;
- otherwise `200` with the batch.

The `/story` block is left untouched.

## Web

### Types and context

- `web/src/types/index.ts`: `MacroStoryOutcome` and `MacroStoryBatch`
  mirroring the Go JSON.
- `AppContext.tsx`: `createStoriesFromMacroTodos(projectId, macroKey,
  todoIds): Promise<MacroStoryBatch | null>`, exposed in `AppContextType`.
  It posts to `/stories`; a non-OK answer throws `trackerError` and is shown
  with `refusalToast` as the single-line call does. On success it calls
  `fetchTasks()`, then shows one toast with the summary: `success` when
  `failed === 0`, `warning` otherwise; when a failed outcome carries
  `code === 'tracker_credential_missing'`, show `tokenOfferToast(tracker)`
  instead of the plain warning (US2.3).

### Pure helpers (`web/src/lib/roadmap.ts`)

So the panel logic is testable with `node --test`:

- `selectableTodoIds(todos)`: ids of unattached lines, in order.
- `pruneTodoSelection(selection, todos)`: drops ids that are attached or gone.
- `batchSummary(batch, copy)`: the "N créées, M passées, K en échec" string,
  with singular and plural forms.
- `todoOrigin(todo)`: `{ kind: 'tasks' | 'spec' | 'stories' | 'manual' |
  'unknown', raw: string, entry: string }`.

### Panel (`RoadmapView.tsx`)

- State: `selectedTodoIds: Set<string>`, `batchRunning: boolean`,
  `batchReport: { macroKey: string; batch: MacroStoryBatch } | null`. All three
  reset when `selected.key` changes. The selection is pruned with
  `pruneTodoSelection` whenever the macro's todos change.
- Each unattached line gains a selection box before the `done` checkbox, with
  a distinct look (square outline with accent tick, `aria-label`
  "Sélectionner « {todo} » pour la création groupée"), so the two are not
  confused.
- The checklist heading row gains **Tout sélectionner** / **Tout
  désélectionner** (hidden with no unattached line) and the button
  **Créer les stories ({count})**, disabled at zero or while running, reading
  **Création…** while running.
- While `batchRunning`, every control of the checklist is `disabled`, and
  `persist` is not called for this macro (FR4).
- On completion: `setMacroMeta` with `batch.macro`, clear the selection, store
  the report. The summary is shown above the checklist; each line whose id is
  in the report shows, beside its content: nothing extra for `created` (its
  key badge already shows), "déjà rattachée à {key}" for `skipped`, the error
  in rose for `failed`, the notice in amber when present.
- Origin badge on every line, after the text: a small muted pill with the
  label of `todoOrigin(todo).kind` (or the raw kind for `unknown`) and
  `title` = entry, or the origin label when no entry.
- The line layout is getting crowded: the badge and the report text go on a
  second row under the text, inside the text column, so the controls row is
  unchanged in width.

### Strings (`web/src/locales/planning.ts`, `framing`, French and English)

`selectTodo`, `selectAll`, `deselectAll`, `createStories` (`{count}`),
`creatingStories`, `batchSummary` pieces (`created_one`, `created_other`,
`skipped_one`, `skipped_other`, `failed_one`, `failed_other`, or the locale's
existing plural convention if it has one), `batchSkipped` (`{key}`),
`originTasks`, `originSpec`, `originStories`, `originManual`,
`originTitle`. Notification copy under `operations.notifications.macros`:
`storiesCreated`, `storiesPartial`, `storiesFailed`.

## Tests

### Go (`internal/db/macros_test.go`, local tracker projects)

- Batch creates every selected unattached line, in slicing order regardless of
  request order, each key recorded, counts right.
- An attached line and a roadmap-project line are `skipped` with their key;
  nothing created for them.
- A line whose target project was deleted is `failed` with the single-line
  sentence; the lines around it are `created`.
- An unknown id is `failed` "ligne de TODO introuvable"; duplicates processed
  once.
- Empty list, unknown project and macro without shaping are whole errors with
  nothing created.
- Keys are recorded line by line: a fake target whose creation fails on the
  third line leaves the first two keys stored (read back from the DB).
- A slicing save made between two lines (simulate through a hook or by saving
  in the creation stub) keeps both the edit and the keys.
- FR7b: saving a stale list without keys keeps the stored keys; a removed line
  is removed.
- Relaunching the same batch creates nothing and reports every line
  `skipped`.
- The single-line path still refuses an attached line with
  "cette ligne a déjà produit …" and records its key (existing test kept).
- Concurrency: two goroutines launching the same batch create each line once
  (run under `-race`).

### Handlers (`internal/handlers`)

- `POST …/macros/M-1/stories` answers `200` with outcomes and counts, also
  when every line failed.
- Bad payload and empty list answer `400`.
- A missing personal credential on a target is reported on the line with
  `code` and `tracker`, the answer still `200`.
- `POST …/macros/M-1/story` answers exactly as before (existing tests pass
  unchanged).

### Web (`web/tests`)

- `roadmap.test.mjs`: `selectableTodoIds`, `pruneTodoSelection`,
  `batchSummary` (singular, plural, zero counts omitted or shown as the
  strings decide), `todoOrigin` for the four kinds, an unknown kind and an
  empty entry.
- `roadmap-view.browser.mjs` (or a new `roadmap-batch-stories.browser.mjs`):
  select all skips attached lines; the button counts; a held answer shows
  every control disabled (hold the answer until released, see project memory
  on held answers); the report and summary render per line; switching macro
  clears selection and report; the origin badges and their tooltips.

## Documentation

- `CHANGELOG.md`, `## [Unreleased]` → `### Added`: one line, for example
  "**Create a slicing's stories in one go.** In an epic's panel on the
  roadmap, tick the slicing lines and create all their stories at once; each
  lands in its line's target project, lines that already have a story are
  skipped, a failing line does not stop the others, and the panel reports
  each line. Every slicing line also shows where it came from. (#634)"
- No README or ADR change: no configuration, no new architecture choice.

## Risks

- Crowded line layout on narrow panels: mitigated by the second row.
- A batch on many lines against a slow tracker holds the request open; there
  is no server write timeout, and the panel shows the running state.
- Multi-replica deployments keep the pre-existing duplicate risk (FR8).

## Implementation notes

Deviations from the plan above, made while implementing; the behaviour of
`spec.md` is unchanged.

- `saveMacroMetaFull` keeps its signature. The line-key option lives in a new
  `saveMacroMetaKeys`, which `saveMacroMetaFull` calls with `nil`, rather than
  adding a trailing parameter to its ten callers, tests included.
- The credential code moved to `trackerapi.CredentialMissingCode`;
  `handlers.TrackerCredentialMissingCode` is now an alias of it.
- The selection is not pruned by an effect: the panel derives the selectable,
  selected lines at render time and prunes the selection when it sends the
  batch, which gives the same result without a cascading render.
- The selection box is a native checkbox, so it never reads as the custom
  `done` box beside it. An attached line keeps an empty slot of the same width,
  so the texts stay aligned.
- The browser test is `web/tests/roadmap-batch-stories.browser.mjs`.
