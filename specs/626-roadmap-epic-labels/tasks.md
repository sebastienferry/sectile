# Tasks #626 - Roadmap: keep and show an epic's labels

Ordered checklist. Each group is one commit and leaves the tree buildable.

## 1. Storage and sync (US1, FR1-FR4)

- [x] T1.1 Migration 35 `macros.labels` in `internal/db/migrations.go`;
  `dropCredentialAccountColumn` in `migrations_test.go` drops the column.
- [x] T1.2 `MacroMeta.Labels` in `internal/models/models.go`, always an array.
- [x] T1.3 `parseMacroLabels`; select and parse `labels` in every macro read
  of `internal/db/macros.go` listed in the plan.
- [x] T1.4 `saveMacroMetaFull` gains `labels *[]string`; existing callers pass
  `nil`; the upsert writes the column.
- [x] T1.5 The macro move copies `labels`.
- [x] T1.6 `ImportMacroHorizons` passes a non-nil copy of `epic.Labels`.
- [x] T1.7 Tests: import keeps labels in order with the horizon label; re-sync
  drops a removed label and clears an emptied list; horizon and framing saves
  keep labels; the move keeps labels; PostgreSQL round trip.

## 2. Axis prefixes and label edit on the server (US4, FR6, FR9-FR11)

- [x] T2.1 `macroAxisPrefixes` and `IsMacroAxisLabel` in `internal/db/macros.go`.
- [x] T2.2 `internal/db/macrolabels.go`: `ValidateMacroLabelEdit` and
  `PushMacroLabels`.
- [x] T2.3 `TrackerOpEpicLabels`, `TrackerOp.Labels` / `RemovedLabels`, its
  enqueue description and `runEpicLabelsOp` in `internal/db/trackerops.go`.
- [x] T2.4 `POST /api/projects/{id}/macros/{key}/labels` in
  `internal/handlers/handlers.go`: `202` with the activity, `400` with the
  sentence.
- [x] T2.5 Tests `macrolabels_test.go`: every refusal queues nothing; the
  push sends only the delta and updates the stored list; a failing tracker
  leaves it unchanged and records nothing pending. Handler test: `202` and one
  `epic_labels` activity; `400` and none for `roadmap:later`.

## 3. Roadmap helpers (FR5-FR7)

- [x] T3.1 `labels: string[]` on `MacroMeta` in `web/src/types/index.ts`.
- [x] T3.2 `EPIC_AXIS_LABEL_PREFIXES`, `isEpicAxisLabel`, `freeEpicLabels`,
  `epicLabelInventory`, `matchesEpicLabels`, `pruneSelectedLabels`,
  `canEditEpicLabels` in `web/src/lib/roadmap.ts`.
- [x] T3.3 `web/tests/roadmapEpicLabels.test.mjs` covering each helper,
  including case-insensitive merge, `#roadmap:now` treated as an axis label,
  OR semantics and pruning.

## 4. Row badges and toolbar filter (US2, US3)

- [x] T4.1 Badges in `renderMacroRow`: all in the unfolded shape, two plus
  `+n` with a tooltip in the condensed shape, none without free labels.
- [x] T4.2 `selectedLabels` state; inventory and filter in the `rows` memo;
  pruning effect.
- [x] T4.3 Toolbar "Labels" popover with counts, hidden on an empty
  inventory; selected labels in `activeFilterChips`.
- [x] T4.4 Strings in `web/src/locales/planning.ts` (fr, en) and their types.

## 5. Panel editor (US4)

- [x] T5.1 `editMacroLabels` in `AppContext.tsx`, with client-side refusals
  and the server sentence in the error toast.
- [x] T5.2 Panel "Labels" block: removable chips and an input with
  suggestions when `canEditEpicLabels`; read-only chips and the refusal
  sentence otherwise.
- [x] T5.3 Strings for the block and the refusals (fr, en) and their types.

## 6. Changelog and checks (FR13, FR14, AC4-AC6)

- [x] T6.1 `CHANGELOG.md`: the `Added` line under `Unreleased`.
- [x] T6.2 `go test ./...` (with the sandbox off for `httptest`), web type
  check, lint, `node --test web/tests`.
- [x] T6.4 Browser regression `web/tests/roadmap-epic-labels.browser.mjs`
  (added at review): badges, filter, editor refusals and calls, read-only epic.
- [ ] T6.3 Manual check on a Jira project: badges, filter, add and remove, a
  refused axis label, a read-only milestone; a GitHub project's roadmap
  unchanged.
