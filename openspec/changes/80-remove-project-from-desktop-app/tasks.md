## 1. Persist and enforce disconnection

- [ ] 1.1 Add workstation-only disconnected project state with backward-compatible reads and atomic writes; preserve unrelated and connection settings.
- [ ] 1.2 Reject disconnected projects before all repository resolution fallbacks; test explicit, inferred, and legacy configurations.
- [ ] 1.3 Make successful validated re-add clear disconnection atomically; cover invalid paths and write failures without restoring deleted overrides.

## 2. Coordinate agent removal and admission

- [ ] 2.1 Implement authenticated per-project DELETE, idempotence, input errors, and persistence failure handling without server or filesystem deletion.
- [ ] 2.2 Serialize admission registration and removal using the documented lock order; reject removal until all matching executions have confirmed exit.
- [ ] 2.3 Expose capability and authoritative disconnection state in status/discovery without deleting finished history.
- [ ] 2.4 Add deterministic tests for queued/preparing/running, terminal-but-not-exited, unrelated projects, history-only projects, repeated removal, and both concurrent orderings.

## 3. Implement desktop interaction

- [ ] 3.1 Add Electron/preload removal bridge with capability check and actionable old-agent error.
- [ ] 3.2 Add Local settings action and confirmation, cancellation, pending state, and clear failure/retry feedback.
- [ ] 3.3 Filter disconnected projects across history-driven rendering, auto-selection, and launch actions; clear selected console only when affected.
- [ ] 3.4 Refresh disconnection state independently of run changes; allow explicit re-add despite retained history and preserve task archive behavior.
- [ ] 3.5 Extend UI tests for success/cancel/conflict/failure, reload, polling changes, selection, re-add, and compatibility.

## 4. Verify and document implementation

- [ ] 4.1 Update desktop settings/behavior documentation and relevant local API/architecture docs; add a changelog entry if present.
- [ ] 4.2 Run `go test ./internal/agentconfig ./cmd/server` and `go test -race ./internal/agentconfig ./cmd/server`.
- [ ] 4.3 Run `go vet ./internal/agentconfig ./cmd/server` and `go build ./cmd/server`.
- [ ] 4.4 Run `npm --prefix desktop run build` and `npm --prefix desktop run test:ui`.
- [ ] 4.5 Run `openspec validate 80-remove-project-from-desktop-app --strict` and `git diff --check`; review the final change against every scenario.

These boxes describe future implementation work and remain unchecked at specification stage.
