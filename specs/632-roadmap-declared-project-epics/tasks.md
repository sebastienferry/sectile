# Tasks #632 - Roadmap: show the epics of declared tracker projects

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green. The Go tests run on SQLite and on PostgreSQL
(`SECTILE_TEST_POSTGRES_DSN` pointed at a throwaway database, never at dev).

## 1. Storage (FR14)

- [ ] T1.1 Migration 37 `projects.roadmap_axis_writes` in
  `internal/db/migrations.go` only (renumber if `main` took 37).
- [ ] T1.2 `internal/db/migrations_test.go`: the rewind helpers and the
  forget-and-replay fixtures drop the new column.
- [ ] T1.3 `models.Project.RoadmapAxisWrites`, and `*bool` on the create and
  update requests.
- [ ] T1.4 `internal/db/db.go`: both project `SELECT`s, the `INSERT` and the
  `UPDATE` carry the column; the value is forced to `false` on a non-Jira
  project or an empty declaration.
- [ ] T1.5 `models.MacroTodo.TargetTrackerProject`, exclusive with
  `TargetProjectID` on save.
- [ ] T1.6 Tests: an existing project reads `false` after the migration; the
  setting round-trips; emptying the declaration closes it; a line keeps one
  target only.

## 2. Origins (FR3, FR8)

- [ ] T2.1 `internal/db/roadmapprojects.go`: `macroOrigin`, `isForeignMacro`,
  `isDeclaredRoadmapProject`.
- [ ] T2.2 `models.MacroMeta.Origin`, `Foreign`, `AxesWritable`, computed and
  never stored; filled by `GetProjectMacros` and by every handler returning a
  macro.
- [ ] T2.3 Tests, table-driven: own key, declared key, former key, lowercase
  key, milestone, local `M-<n>`, key without a dash, GitHub and local
  projects (never foreign).

## 3. Reading the declared keys (FR1, FR2; US1)

- [ ] T3.1 `remoteMacros`: own key, then one `ListEpics` per declared key on a
  project copy, each under its own timeout; failures collected.
- [ ] T3.2 `ImportMacroHorizons`: foreign epics stored like own ones; the
  summary names every failing declared key.
- [ ] T3.3 Tests with a fake tracker: two declared keys, one failing (US1.1,
  US1.3); own key failing (US1.4); no declaration (US1.5); axes read from a
  foreign epic's labels (US1.6); removed key keeps its stored epics (US1.7);
  no `SyncIssues` call carries a declared key (US1.2).

## 4. Writes on foreign epics (FR9 to FR13; US3, US5)

- [ ] T4.1 `macroTracker` takes the axis; foreign epics refused except priority
  and quarter with the opt-in open and the origin still declared, on the
  project copy.
- [ ] T4.2 `pushMacroLabels`, the three `PushMacro*Label`,
  `ValidateMacroLabelEdit` and `PushMacroLabels` pass their axis.
- [ ] T4.3 `MacroLabelsWritable` gains the axis and `bulk` inputs; `AxesWritable`
  computed from it.
- [ ] T4.4 `handlers.go` macro `PUT`: `bulk` read; horizon enqueued only when
  writable, otherwise the "conservé dans Sectile" note;
  `enqueueMacroAxes` reads `AxesWritable`.
- [ ] T4.5 Pending pushes unchanged; a test proves a foreign epic is never
  listed, opt-in open or closed (US3.5).
- [ ] T4.6 Tests: horizon of a foreign epic queues nothing and fails nothing
  (US3.4); free label edit refused (US3.3); opt-in closed keeps priority and
  quarter local (US5.2); opt-in open queues exactly one label-only operation
  per axis edit (US5.3, US5.4, AC4); `bulk` never queues on a foreign epic
  (US5.6); key undeclared or opt-in closed between enqueue and run refuses in
  the activity; tracker refusal leaves the value set (US5.7).

## 5. Story creation in a declared key (FR16, FR17; US4)

- [ ] T5.1 `internal/db/remotetarget.go`: `createStoryInRoadmapProject`, no local
  task recorded.
- [ ] T5.2 `CreateStoryFromMacroTodo`: branch on `TargetTrackerProject`, recheck
  the declaration, refuse on a non-Jira project, record the key, notice says the
  story stays in Jira.
- [ ] T5.3 Tests with a fake Jira adapter: created with title, story type and
  parent (US4.2); key recorded and second creation refused (US4.3); no task
  row, board and counters unchanged (US4.4); undeclared key refused before any
  call (US4.5); creation refused records nothing and names the fields (US4.6);
  parent refused keeps the key (US4.7); opt-in closed changes nothing (US4.8).

## 6. Web: origins (FR4 to FR7; US2)

- [ ] T6.1 `web/src/lib/roadmapOrigins.ts` and `roadmapOrigins.test.ts`:
  offered origins and their order, counts, normalization (empty, unknown,
  removed key), storage read and write tolerating an unavailable storage.
- [ ] T6.2 `RoadmapView.tsx`: selection state per project, origin filter in the
  row pipeline, counts without the origin filter, toolbar multi-select, active
  filter chip, hidden when FR4 says so.
- [ ] T6.3 Browser component test (`*.browser.mjs`) of the toolbar: default
  selection, ticking an origin, empty selection falling back, restored after a
  remount.

## 7. Web: foreign epics, targets and settings (FR8, FR9, FR13, FR15, FR18; US3 to US5)

- [ ] T7.1 Row origin badge; panel read-only mark; `EpicLabelEditor`, framing
  post and epic edit hidden on a foreign epic; axes note reads `axesWritable`
  with the opt-in sentence.
- [ ] T7.2 `runSeed` sends `bulk: true`.
- [ ] T7.3 `lookups.ts` `roadmapTargetOptions`; the target picker's option group;
  `targetTrackerProject` saved; the creation result without a task id does not
  navigate.
- [ ] T7.4 `ProjectModal.tsx` opt-in checkbox and its `false` fallbacks.
- [ ] T7.5 `types/index.ts`, `locales/planning.ts`, `locales/projectSettings.ts`
  in French and English, new `roadmapProjectsHelp`.
- [ ] T7.6 Unit tests of `roadmapTargetOptions` and of the picker value mapping.

## 8. Documentation (FR20, FR21)

- [ ] T8.1 `CHANGELOG.md`: three `Added` lines under `[Unreleased]`.
- [ ] T8.2 ADR `docs/adrs/0042-*.md` restating the read-only rule of declared
  projects.
- [ ] T8.3 Any page of `docs/` describing the roadmap settings follows the new
  help text.

## Test plan

- `go test ./internal/db/... ./internal/handlers/...` on SQLite, then with
  `SECTILE_TEST_POSTGRES_DSN` on a throwaway database. Outside the sandbox, with
  `GOCACHE` under `$TMPDIR`.
- `cd web && node --test` for the `lib` tests, the browser component tests with
  Playwright from `desktop/node_modules`.
- `cd web && npx tsc --noEmit` and the linter.
- Manual check on a copy of the database, never on the dev `tasks.db`, and with
  the tracker token withheld until the write scenarios, since writes reach the
  real Jira: declare two keys, one unreachable; import; select origins; classify
  a foreign epic with the opt-in closed then open; create a story in a declared
  key.
