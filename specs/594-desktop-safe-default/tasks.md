# Tasks #594 - Five parallel executions by default

Ordered checklist. Each group is one commit and leaves the tree buildable.

## 1. Agent fallback (FR1, FR2, FR3, FR4)

- [x] T1.1 `internal/agentconfig/local.go`: add `DefaultParallelism = 5` next
  to `MaxParallelism`; `ExecutionLimit` falls back to it; update its doc
  comment.
- [x] T1.2 `TestExecutionLimitInheritance`: unset gives 5; explicit 1 in the
  defaults gives 1; explicit 1 in the project over unset defaults gives 1;
  no worktrees gives 1; existing cases kept.
- [x] T1.3 Fix every Go test that relied on 1 for an unset value.

## 2. Desktop display (FR5)

- [x] T2.1 `desktop/src/main.js`: `DEFAULT_PARALLELISM=5`; workstation panel
  readout and hint; project panel fallback.
- [x] T2.2 `desktop/tests/workstation-settings.ui.cjs`: with no parallelism in
  the defaults, the slider reads 5 and the hint "Default · 5 executions".

## 3. Documentation (FR6, FR7)

- [x] T3.1 `docs/contracts/server-agent-v1.md`: "one execution at a time"
  and "else 1" become the new default.
- [x] T3.2 `CHANGELOG.md`: one line under `Changed` in `[Unreleased]`.

## 4. Verification

- [x] T4.1 `go vet ./...`, `go test ./internal/agentconfig/... ./internal/agent/...`.
- [x] T4.2 `npx vite build` in `desktop/`, then the two UI tests.
