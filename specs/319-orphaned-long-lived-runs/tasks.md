# Task checklist - #319

Ordered so the build stays green at every step. Built after #318 on the same
branch, since both change `sessions.go`.

## 1. Server

- [x] T1 - Rewrite predicate matches the note anywhere; tests for a silenced then
      closed task run and macro run. *(FR5)*
- [x] T2 - `NewSessionRegistryBounded`, abandon bound, SDK session kept and closed; unit
      tests (abandon closes, below bound keeps, clamp). *(FR1)*
- [x] T3 - `mcpAbandonAfter` and its wiring; override test. *(FR2)*
- [x] T4 - `handleCancelRemoteRun` client branch; handler tests (owner, admin,
      stranger, ownerless member). *(FR4)*
- [x] T5 - Activities cancel of a client run; handler tests. *(FR6)*

## 2. Web

- [x] T6 - `silent` run state and precedence; `runStates` and
      `remoteRunIndicator` tests. *(FR3)*
- [x] T7 - `closableRunIds` and the Close button; indicator tests. *(FR4)*
- [x] T8 - Labels, classes and translations; `cancelActivity` error message.

## 3. Documentation

- [x] T9 - ADR 0007 second amendment, contract, README, `.env.sample`, CHANGELOG.

## Test plan

`go test ./...`, `gofmt -l`, `go vet ./...`, web `npm test` + `tsc` + `oxlint`,
desktop `npm test`.
