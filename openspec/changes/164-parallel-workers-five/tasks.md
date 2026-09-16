# Tasks

## 1. Ceiling
- [x] 1.1 Export `models.MaxParallelism = 5` and use it in `NormalizeParallelism`.
- [x] 1.2 Clamp the server job limiter and `GetProjectParallelism` with the constant.
- [x] 1.3 Clamp `agentconfig.ExecutionLimit` and the desktop mapping validation.
- [x] 1.4 Render the web and desktop pickers from 1 to the ceiling.

## 2. Console exemption
- [x] 2.1 Skip console runs when counting active per-project executions.
- [x] 2.2 Admit console runs without the per-project capacity comparison.
- [x] 2.3 Restrict console checkout collision to non-isolated executions, and let
      two consoles run side by side.

## 3. Verification
- [x] 3.1 Unit tests: normalizer and `ExecutionLimit` bounds at 1, 5 and above.
- [x] 3.2 Admission tests: a saturated project still admits a console, a console
      does not consume a worker slot, and a console still waits on a shared checkout.
- [x] 3.3 `make build`, `go vet`, `go test ./...`, web build.
