# Tasks #870 - Jira refuses a comment whose Markdown has inline code inside bold text

Order matters: each step leaves the tree buildable and the tests green.

## 1. Converter

- [ ] T1.1 `parseInline` keeps only `link` marks next to `code` (US1, FR1).
- Tests: `adf_test.go` cases for bold, italic (both forms), strike, link,
  bold link; a walk asserting no `code` node carries a non-`link` mark (US1.5).

## 2. Stage job outcome

- [ ] T2.1 `DB.AwaitTrackerOp` (plan §2).
- Tests: completed, failed and still-queued activities; canceled context.

## 3. `transition_stage`

- [ ] T3.1 Wait and map the outcome into `trackerSync` / `trackerError`
      (US2.1 to US2.3, FR2 to FR4).
- [ ] T3.2 Tool description names the three values (US2.4).
- Tests: Jira project whose comment endpoint refuses with 400: `failed`, error
  carried, stage applied, no MCP error; accepting endpoint: `done`.

## 4. Skill contract

- [ ] T4.1 `contracts/transition.md` tells the agents to read `trackerSync`
      (US2.5); golden files regenerated.

## 5. Documentation

- [ ] T5.1 `CHANGELOG.md` `Fixed` lines under `[Unreleased]`.

## Test plan

`go build ./...`, `go vet ./...`, `go test ./internal/trackerapi/
./internal/db/ ./internal/taskmcp/ ./internal/skills/`, then `make test`.
