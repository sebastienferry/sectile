# Tasks #586 - Reset skill status

## Agent

- [ ] T1 `skillSuccessor(own, activities, held, exitedAt)` in
      `agent_desktop.go` (FR1).
- [ ] T2 `desktopRunResult`: copy held ids and exit time under the lock, set a
      missing `finishedAt`, encode `successor` (FR1).
- [ ] T3 Go tests: `skillSuccessor` table, handler with and without a
      successor.

## Desktop renderer

- [ ] T4 `workflowSkill` and the successor branch of `skillResult` (FR2, FR3).
- [ ] T5 `skillRun` in `orderedTaskGroups` (FR4).
- [ ] T6 Row badge bound to `skillRun.id` in `main.js` (FR4, FR7).
- [ ] T7 `ended` in the refresh stamp, due at every poll while live (FR6);
      pass it from `refreshSkillResult`.

## Tests

- [ ] T8 `skill-result.ui.cjs`: successor states, name mapping, console and
      discussion, unchanged single-skill cases.
- [ ] T9 `task-order.test.mjs`: `skillRun` selection.
- [ ] T10 `skill-result-refresh.test.mjs`: `ended` cadence.
- [ ] T11 UI test for path B: row badge follows the new execution, old
      console's header keeps `✓`.

## Wrap-up

- [ ] T12 `CHANGELOG.md`: one `Fixed` line under `[Unreleased]`.
- [ ] T13 `go build ./...`, `go vet ./internal/agent/`, `go test
      ./internal/agent/`, desktop `node --test` suites, `npx vite build` then
      the touched UI tests.
