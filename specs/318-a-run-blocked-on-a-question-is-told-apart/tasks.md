# Task checklist - #318

Ordered so the build stays green at every step.

## 1. Store

- [ ] T1 - `SetRemoteRunWaiting`: first mark wins (`COALESCE`). *(FR3)*
- [ ] T2 - `ReportRemoteRunWaitingAs`: run check, owner rule, headless no-op. *(FR2, FR4)*
- [ ] T3 - Tests in `internal/db/waitingrun_test.go`: first mark wins, stranger
      refused, admin and ownerless allowed, headless not applied, wrong task and
      finished run refused.

## 2. Session registry and tool

- [ ] T4 - `RunWaiter`, per-session waiting set, `MarkWaiting`, `ForgetWaiting`,
      `Resume`, `ReleaseRun`, clearing in `Close`. *(FR5)*
- [ ] T5 - Middleware: `Resume` on `tools/call` other than `report_waiting`. *(FR5)*
- [ ] T6 - `report_waiting` tool; bridge catalog (twelve tools); naming contract. *(FR1)*
- [ ] T7 - Tests: registry unit tests (resume, pings keep, other session keeps,
      close clears), an MCP end-to-end test over HTTP (mark, other call clears).

## 3. Desktop banner

- [ ] T8 - Post-back listener dispatches `run_waiting` to the owner's agent. *(FR6)*
- [ ] T9 - Agent: `desktopRun.WaitingSince`, `run_waiting` case. *(FR6)*
- [ ] T10 - Tests: agent handler (set, clear, headless, unknown run); handler
      test that the owner's agent receives the message.

## 4. Skills and documentation

- [ ] T11 - Contract fragment rule; `UPDATE_GOLDEN=1 go test ./internal/skills/`. *(FR9)*
- [ ] T12 - ADR 0012 revision, contract, CAPABILITIES, README, CHANGELOG, MEMORY. *(FR7, FR10)*

## Test plan

`go test ./...`, `gofmt -l`, `go vet ./...`, web `npm test`, desktop `npm test`.
