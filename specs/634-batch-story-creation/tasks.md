# Tasks #634 - Roadmap: create the stories of a slicing in one gesture

Ordered checklist. Each group is one commit and leaves the tree buildable.
References point at `spec.md` (US, FR) and `plan.md`.

## 1. Shared per-line core and key record (FR6, FR7, FR7b, FR8, FR12)

- [ ] T1.1 `macroStoryLocks` keyed lock on `DB` in `internal/db/macros.go`.
- [ ] T1.2 `macroLineAttachedError` carrying the key and the roadmap flag; its
  `Error()` returns today's two French sentences.
- [ ] T1.3 `createStoryFromLine` extracted from `CreateStoryFromMacroTodo`
  (checks in today's order, then `createStoryUnder`).
- [ ] T1.4 `saveMacroMetaFull` gains `storyKeys map[string]string`; every
  existing caller passes `nil`; `recordLineStoryKey` built on it.
- [ ] T1.5 FR7b in `saveMacroMetaFull`: an incoming line without a key keeps
  its stored key.
- [ ] T1.6 `CreateStoryFromMacroTodo` rewritten on T1.1-T1.4, signature and
  refusals unchanged.
- [ ] T1.7 Tests: existing `TestCreateStoryFromMacroTodoReturnsTheTask` and
  the other single-line tests pass unchanged; stale save keeps keys and
  removes omitted lines; a save between creation and record keeps both.

## 2. Batch in the database layer (FR5, FR6, FR9, FR10, FR11)

- [ ] T2.1 `MacroStoryOutcome`, `MacroStoryBatch`,
  `CreateStoriesFromMacroTodos` in `internal/db/macros.go`.
- [ ] T2.2 Move or mirror `TrackerCredentialMissingCode` so `db` can set
  `Code` without importing `handlers`.
- [ ] T2.3 Tests in `internal/db/macros_test.go`: slicing order; skipped
  attached and roadmap lines; failed missing target between two created lines;
  unknown and duplicate ids; whole-request refusals create nothing; keys
  stored line by line when a later line fails; relaunch reports all skipped;
  two concurrent batches create each line once (`-race`).

## 3. Route (FR5, FR10, FR12)

- [ ] T3.1 `POST /api/projects/{id}/macros/{key}/stories` (and `epics`) in
  `internal/handlers/handlers.go`, beside `/story`.
- [ ] T3.2 Handler tests: `200` with outcomes and counts, also all failed;
  `400` on bad payload and empty list; credential failure on a line carries
  `code` and `tracker`; `/story` tests unchanged.

## 4. Web helpers and context (FR1-FR3, FR13, FR14)

- [ ] T4.1 `MacroStoryOutcome`, `MacroStoryBatch` in `web/src/types/index.ts`.
- [ ] T4.2 `selectableTodoIds`, `pruneTodoSelection`, `batchSummary`,
  `todoOrigin` in `web/src/lib/roadmap.ts`.
- [ ] T4.3 Strings in `web/src/locales/planning.ts`, French and English
  (`framing` and `operations.notifications.macros`).
- [ ] T4.4 `createStoriesFromMacroTodos` in `AppContext.tsx`: `fetchTasks`,
  summary toast, token offer on a credential failure.
- [ ] T4.5 `web/tests/roadmap.test.mjs`: the four helpers.

## 5. Panel (US1-US5, FR1-FR4, FR13)

- [ ] T5.1 Selection state, reset on macro change, pruned on todo change.
- [ ] T5.2 Selection box on unattached lines, select all / deselect all, batch
  button with count and running label.
- [ ] T5.3 Every checklist control disabled while running; no `persist` for
  the macro meanwhile; `setMacroMeta` with the returned macro at the end.
- [ ] T5.4 Summary above the checklist and per-line outcome under the text.
- [ ] T5.5 Origin badge with tooltip on every line.
- [ ] T5.6 Browser test (`web/tests/roadmap-batch-stories.browser.mjs` or
  inside `roadmap-view.browser.mjs`): select all skips attached lines; count;
  held answer disables every control; report and summary; macro switch clears
  them; badges and tooltips.

## 6. Documentation (FR15)

- [ ] T6.1 `CHANGELOG.md` line under `## [Unreleased]` → `### Added` (#634).

## Test plan

Run before the implemented transition:

- `go vet ./...` and `go test ./internal/db/... ./internal/handlers/...`
  (outside the sandbox for `httptest`; `-race` on the concurrency test).
- PostgreSQL run of `internal/db` when a throwaway DSN is available: the key
  record and FR7b go through `forUpdate()`.
- `node --test web/tests/roadmap.test.mjs`.
- `npx tsc --noEmit` and `npx oxlint` in `web/`.
- `npx vite build`, then the browser test.
- Manual check on a copy of the dev database, started without a tracker token
  so nothing reaches the real tracker: select three lines, one with a deleted
  target project, and read the report.
