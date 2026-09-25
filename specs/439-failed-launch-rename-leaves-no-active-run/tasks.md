# #439: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

- [ ] T1 `closeLaunch` with the status-only variant and the bounded retry (FR1-FR3).
- [ ] T2 `processSkillJob` uses it, passing whether the rename succeeded.
- [ ] T3 Test forcing the rename to fail with a trigger; a new launch is accepted.

## Test plan

- `go vet ./internal/db/`, `gofmt -l internal/db`.
- `go test ./internal/db/ -run 'Launch|ActiveRun|SkillJob'`, then the full package.
