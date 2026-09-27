# Tasks #558 - The adjustment contract carries guardrails only

## Code

- [x] T1 Rewrite `AdjustmentContract` in `internal/runner/adjustment.go` to
      the guardrails of FR1 (FR1, FR2).

## Tests

- [x] T2 Native launch in `TestNativeAdjustmentAliasesAndReconciliation`:
      guardrails present, prescriptions absent (AC1).
- [x] T3 Managed prompt with a configured `PromptAdjust`: contract guardrails
      present, prescriptions absent (AC2).
- [x] T4 Managed prompt from the built-in fallback: still requires build,
      lint, tests and push (AC3, FR4).

## Docs

- [x] T5 ADR 0038 and the amendment note in ADR 0004 (FR5, AC4).
- [x] T6 `Changed` line in `CHANGELOG.md` (FR6, AC5).

## Test plan

- `go build ./...`, `go vet ./internal/...`, `go test ./internal/runner/... ./internal/agent/...`,
  then `go test ./...`.
