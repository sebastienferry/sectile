# Tasks #627 - Roadmap: give each epic its own priority and quarter

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. Storage (FR1)

- [ ] T1.1 Migration 33 `macros.priority_quarter` in
  `internal/db/migrations.go`: the two `ADD COLUMN` statements.
- [ ] T1.2 `internal/db/migrations_test.go`: the rewind helpers drop
  `macros.priority` and `macros.quarter` wherever they rewind before 33.
- [ ] T1.3 `models.MacroMeta`: `Priority`, `Quarter`, `LabelsWritable`.
- [ ] T1.4 Every macro read in `internal/db/macros.go` selects and scans the
  two columns.
- [ ] T1.5 `saveMacroAxes` and the two new pointers of `UpdateMacro`.
- [ ] T1.6 Tests: save and read back a priority and a quarter, clear them,
  leave the other macro fields untouched; SQLite and PostgreSQL.

## 2. Axis vocabulary (FR4, FR8)

- [ ] T2.1 `internal/db/macroaxes.go`: prefixes, `NormalizeEpicPriority`,
  `NormalizeQuarter`, `PriorityLabel`, `QuarterLabel`, `PriorityFromLabels`,
  `QuarterFromLabels`, `isQuarterLabel`.
- [ ] T2.2 `internal/db/macroaxes_test.go`, table-driven: every accepted form
  of each normalizer and the refused ones (`Q4`, `2026-Q5`, `P4`, `bientôt`);
  case and `#` ignored; prefixed quarter wins over bare; invalid labels read
  as absent (US7.2, US7.3, US7.5).

## 3. Tracker writes (FR5, FR6, FR7, FR9)

- [ ] T3.1 Extract `pushMacroLabels` from `PushMacroHorizonLabel`; horizon
  tests unchanged and green.
- [ ] T3.2 `PushMacroPriorityLabel` and `PushMacroQuarterLabel` (the latter
  reads the epic with `GetIssue` and strips both quarter forms).
- [ ] T3.3 `macroLabelsWritable`; `GetProjectMacros` sets `LabelsWritable`.
- [ ] T3.4 `TrackerOpEpicPriority`, `TrackerOpEpicQuarter`, their `TrackerOp`
  fields, activity texts and runners, with the "set in Sectile but not on the
  ticket" failure wording.
- [ ] T3.5 Widen `PendingHorizonPushes` and `PushPendingHorizons` to the three
  axes; failures named with their key and axis.
- [ ] T3.6 Tests with a fake tracker: priority set replaces the other
  `priority:` labels; clear removes all four; quarter set removes
  `quarter:2026-q3` and bare `2026-Q3` and adds `quarter:2026-q4`; no field
  other than labels is sent (AC3); milestone, foreign epic and non-epic
  tracker are never pushed nor pending (FR6); a pending macro with only a
  differing priority is pushed on that axis only.

## 4. Read-back (FR8, US7)

- [ ] T4.1 `ImportMacroHorizons` stores the priority and quarter read from the
  labels when present, keeps the local value otherwise, extends its summary.
- [ ] T4.2 Tests: tracker value replaces a different local one; no label keeps
  the local one and makes it pending; bare and prefixed quarter handling; the
  import sends no write.

## 5. API (FR3, FR4, FR6)

- [ ] T5.1 Macro handler accepts `priority` and `quarter`, normalizes (400 on
  invalid), saves, enqueues only when writable, sets `labelNote`.
- [ ] T5.2 Handler tests: valid values queued on a Jira project; invalid value
  refused with nothing saved; GitHub milestone saved with no op queued and the
  "kept in Sectile" note.

## 6. Web logic (FR2, FR10, FR12)

- [ ] T6.1 `web/src/types/index.ts`: `EpicPriority`, the three `MacroMeta`
  fields.
- [ ] T6.2 `web/src/lib/epicAxes.ts`: constants, `normalizeQuarter`,
  `titleAxes`, `seedProposals`, `sortByPriority`, `matchesPriority`.
- [ ] T6.3 `web/src/lib/roadmap.ts`: row priority and quarter from the meta;
  the child-derived loop removed.
- [ ] T6.4 `web/tests/epicAxes.test.mjs`: title parsing
  (`2026.Q4 [P2] - Platform AI`, `2026.Q4 P0  - Grafana`, `PEP1`,
  `X2026-Q4`, `1.2026Q4`, first candidate wins), quarter normalization,
  proposals skip axes already set and epics with nothing to propose, stable
  sort with "no priority" last in both orders, filter incl. `none`.
- [ ] T6.5 Update `roadmapProjects` / `roadmapDisplayMode` tests if they
  assert the derived priority; add one asserting an epic with high-priority
  children and no own priority shows none (US1.2).

## 7. Web UI (US1 to US6, FR13)

- [ ] T7.1 Row badges: `P0`..`P3` with the mapped colours, muted "no
  priority".
- [ ] T7.2 Panel: priority control, quarter field with inline validation and
  clear, "kept in Sectile" line when `labelsWritable` is false;
  `saveMacroMeta` patch type widened.
- [ ] T7.3 Toolbar: priority filter with its chip, sort select, applied before
  the tab counts and lists.
- [ ] T7.4 Seeding button and modal: ticked proposals, confirm, cancel, empty
  state, result toast naming failures.
- [ ] T7.5 `web/src/locales/planning.ts`: every new string in fr and en;
  run the translation check (`docs/web-translation-checks.md`).

## 8. Documentation (FR14)

- [ ] T8.1 `CHANGELOG.md` under `[Unreleased]`: `Added` (epic priority and
  quarter, set from the panel, written as Jira labels, seeded from titles,
  priority filter and sort) and `Changed` (the roadmap priority is the epic's
  own, no longer the highest of its tickets). Reference #627.
- [ ] T8.2 `docs/USER_GUIDE.md` roadmap section and
  `docs/API_AND_DATA_SPEC.md` (macro payload fields, `macros` columns, the
  label axes).

## Test plan

- `go test ./internal/db/... ./internal/handlers/...` on SQLite, then with
  `SECTILE_TEST_POSTGRES_DSN` pointing at a throwaway database.
- `node --test web/tests/*.test.mjs`, the web typecheck and lint.
- Manual, against a copy of the dev database and without a tracker token
  first: set, clear and seed on a GitHub project (values stay local, no
  activity queued); then on a Jira sandbox epic: labels added and removed as
  in US2 to US4, read back after editing a label on Jira (US7).
