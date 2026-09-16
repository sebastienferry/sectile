# Tasks

## 1. Ceiling
- [ ] 1.1 Export `models.MaxParallelism = 5` and use it in `NormalizeParallelism`.
- [ ] 1.2 Clamp the server job limiter and `GetProjectParallelism` with the constant.
- [ ] 1.3 Clamp `agentconfig.ExecutionLimit` and the desktop mapping validation.
- [ ] 1.4 Render the web and desktop pickers from 1 to the ceiling.

## 2. Console exemption
- [ ] 2.1 Skip console runs when counting active per-project executions.
- [ ] 2.2 Admit console runs without the per-project capacity comparison.
- [ ] 2.3 Keep shared-checkout blocking for consoles.

## 3. Verification
- [ ] 3.1 Unit tests: normalizer and `ExecutionLimit` bounds at 1, 5 and above.
- [ ] 3.2 Admission tests: a saturated project still admits a console, a console
      does not consume a worker slot, and a console still waits on a shared checkout.
- [ ] 3.3 `make build`, `go vet`, `go test ./...`, web build.
