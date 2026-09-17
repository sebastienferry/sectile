# Tasks

## 1. Normalise the workflow label at the sink
- [x] 1.1 In `SetWorkflowLabel` (`internal/db/db.go:1870`), build the appended workflow label as
      `"#" + strings.ToLower(cleanTarget)` whatever the caller passed, and drop the branch on
      `strings.HasPrefix(targetLabel, "#")`.
- [x] 1.2 Leave the non-workflow branch (`db.go:1886-1891`) untouched: those labels keep their
      casing and their prefix or absence of one.
- [x] 1.3 State the contract in a comment above the function: the caller supplies a stage name
      with or without `#`, the function decides the spelling.

## 2. Align the call sites that pass the bare form
- [x] 2.1 `CreateTask` (`db.go:1980`): pass `"#new"`.
- [x] 2.2 Clone path (`db.go:2139`): pass `"#new"`.
- [x] 2.3 `db.go:4676`: pass the prefixed form of `GetStageLabelForStatus(task.Status)`.
- [x] 2.4 Leave `GetStageLabelForStatus` returning the bare stage name, and leave its
      comparisons (`db.go:2218`, `db.go:2598-2605`, `board.go:280`) unchanged.

## 3. Remove the dead duplicate
- [x] 3.1 Delete `var WorkflowLabels` (`db.go:1806`) after confirming with a repository-wide
      search that nothing references it.

## 4. Align the add form
- [x] 4.1 `web/src/components/QuickAddModal.tsx:77`: seed the default label with `'#new'`.
- [x] 4.2 `QuickAddModal.tsx:344-352`: compare the badge label with the `#` stripped as well as
      lowercased, so `#new` and `new` get the same badge.

## 5. Pin the spelling with tests
- [x] 5.1 Update `internal/db/labels_test.go` expectations from `new` to `#new`
      (lines 85, 115, 142, 195, 235, 273), keeping the custom labels `CustomerCase` and
      `#TeamTag` asserted verbatim.
- [x] 5.2 Add a test calling `SetWorkflowLabel` directly with a bare `targetLabel`, a prefixed
      one, and a mixed-case one, asserting `#new` in all three cases.
- [x] 5.3 Add a test asserting that a task carrying a legacy bare `new` label still resolves to
      the `new` stage through `StageOfTask` and is cleaned at the next transition.

## 6. Verify
- [x] 6.1 Run the Go test suite, including `chain_test.go`, `headless_test.go`, and
      `terminalrun_test.go`.
- [x] 6.2 Run the front-end build and lint.
- [x] 6.3 Couvert par test plutôt que par exécution live : `TestCreateTaskLowercaseDefault
      WorkflowLabel` exerce `CreateTask`, le chemin commun à `POST /api/tasks` et au tool MCP
      `create_task`, et assied `#new` en base comme en retour. La pose côté tracker passe par
      le même tableau de labels.
