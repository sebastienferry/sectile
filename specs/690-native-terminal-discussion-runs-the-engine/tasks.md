# Tasks #690 - "Discussion in native terminal" runs the ticket discussion

Ordered checklist. Each group leaves the tree buildable.

## 1. Test first

- [ ] T1.1 Add the test of `plan.md` §3 and confirm it fails on the current
  code (nothing runs in the console).

## 2. Behaviour

- [ ] T2.1 Resolve the task's configuration and build the discussion line
  (FR1, FR2).
- [ ] T2.2 Register the run `preparing` with its engine, wrap the line, type
  it with `runInPty`, move the run to `running` (FR3 to FR5).
- [ ] T2.3 Release the run on every failure after registration (FR6).

## 3. Changelog

- [ ] T3.1 One `Fixed` line under `## [Unreleased]` (FR8).

## 4. Verification

- [ ] T4.1 `go build ./...`, `go vet ./internal/agent/`.
- [ ] T4.2 `go test ./internal/agent/...`.
