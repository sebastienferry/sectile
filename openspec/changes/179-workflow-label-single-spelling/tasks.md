# Tasks

## 1. Normalise the workflow label at the sink
- [ ] 1.1 In `SetWorkflowLabel` (`internal/db/db.go:1870`), build the appended workflow label as
      `"#" + strings.ToLower(cleanTarget)` whatever the caller passed, and drop the branch on
      `strings.HasPrefix(targetLabel, "#")`.
- [ ] 1.2 Leave the non-workflow branch (`db.go:1886-1891`) untouched: those labels keep their
      casing and their prefix or absence of one.
- [ ] 1.3 State the contract in a comment above the function: the caller supplies a stage name
      with or without `#`, the function decides the spelling.

## 2. Align the call sites that pass the bare form
- [ ] 2.1 `CreateTask` (`db.go:1980`): pass `"#new"`.
- [ ] 2.2 Clone path (`db.go:2139`): pass `"#new"`.
- [ ] 2.3 `db.go:4676`: pass the prefixed form of `GetStageLabelForStatus(task.Status)`.
- [ ] 2.4 Leave `GetStageLabelForStatus` returning the bare stage name, and leave its
      comparisons (`db.go:2218`, `db.go:2598-2605`, `board.go:280`) unchanged.

## 3. Remove the dead duplicate
- [ ] 3.1 Delete `var WorkflowLabels` (`db.go:1806`) after confirming with a repository-wide
      search that nothing references it.

## 4. Align the add form
- [ ] 4.1 `web/src/components/QuickAddModal.tsx:77`: seed the default label with `'#new'`.
- [ ] 4.2 `QuickAddModal.tsx:344-352`: compare the badge label with the `#` stripped as well as
      lowercased, so `#new` and `new` get the same badge.

## 5. Pin the spelling with tests
- [ ] 5.1 Update `internal/db/labels_test.go` expectations from `new` to `#new`
      (lines 85, 115, 142, 195, 235, 273), keeping the custom labels `CustomerCase` and
      `#TeamTag` asserted verbatim.
- [ ] 5.2 Add a test calling `SetWorkflowLabel` directly with a bare `targetLabel`, a prefixed
      one, and a mixed-case one, asserting `#new` in all three cases.
- [ ] 5.3 Add a test asserting that a task carrying a legacy bare `new` label still resolves to
      the `new` stage through `StageOfTask` and is cleaned at the next transition.

## 6. Verify
- [ ] 6.1 Run the Go test suite, including `chain_test.go`, `headless_test.go`, and
      `terminalrun_test.go`.
- [ ] 6.2 Run the front-end build and lint.
- [ ] 6.3 Create a task through the API and confirm its sole workflow label is `#new`, locally
      and on the tracker.
