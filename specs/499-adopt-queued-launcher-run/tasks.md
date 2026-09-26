# Tasks #499 - A queued launcher run can be adopted and finished

Ordered checklist. One commit, leaving the tree buildable.

## 1. Task runs (US1-US3, FR1-FR4)

- [ ] T1.1 `startRemoteRun`: adopt `queued` (plan decision 1) and the two messages (decision 2).
- [ ] T1.2 `finishRemoteRun`: close `queued` too (decision 3).
- [ ] T1.3 `SyncRemoteRunStatusFor`: no demotion of `running` (decision 4).
- [ ] T1.4 DB tests: queued adopted and returned running; ended and foreign runs refused with their messages; queued finished directly; queued report after adoption keeps running; first queued report still inserted queued.

## 2. Macro runs (US4)

- [ ] T2.1 `startMacroRun` and `FinishMacroRunAs`: same rules.
- [ ] T2.2 DB test: queued macro run adopted then finished.

## 3. End to end and changelog

- [ ] T3.1 MCP test: launcher run queued by an agent report → `start_run(runId)` → `finish_run(completed)`.
- [ ] T3.2 `CHANGELOG.md` `Fixed` line (FR5).

## 4. Verification

- [ ] T4.1 `go test ./internal/db/ ./internal/taskmcp/ ./internal/handlers/ -count=1`, `go vet`.
