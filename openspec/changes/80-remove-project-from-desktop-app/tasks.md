## 1. Persist and enforce disconnection

- [x] 1.1 Add workstation-only disconnected project state with backward-compatible reads and atomic writes; preserve unrelated and connection settings.
- [x] 1.2 Reject disconnected projects before all repository resolution fallbacks; test explicit, inferred, and legacy configurations.
- [x] 1.3 Make successful validated re-add clear disconnection atomically; cover invalid paths and write failures without restoring deleted overrides.

## 2. Coordinate agent removal and admission

- [x] 2.1 Implement authenticated per-project DELETE, idempotence, input errors, and persistence failure handling without server or filesystem deletion.
- [x] 2.2 Serialize admission registration and removal using the documented lock order; reject removal until all matching executions have confirmed exit.
- [x] 2.3 Expose capability and authoritative disconnection state in status/discovery without deleting finished history.
- [x] 2.4 Add deterministic tests for queued/preparing/running, terminal-but-not-exited, unrelated projects, history-only projects, repeated removal, and both concurrent orderings.

## 3. Implement desktop interaction

- [x] 3.1 Add Electron/preload removal bridge with capability check and actionable old-agent error.
- [x] 3.2 Add Local settings action and confirmation, cancellation, pending state, and clear failure/retry feedback.
- [x] 3.3 Filter disconnected projects across history-driven rendering, auto-selection, and launch actions; clear selected console only when affected.
- [x] 3.4 Refresh disconnection state independently of run changes; allow explicit re-add despite retained history and preserve task archive behavior.
- [x] 3.5 Extend UI tests for success/cancel/conflict/failure, reload, polling changes, selection, re-add, and compatibility.

## 4. Verify and document implementation

- [x] 4.1 Update desktop settings/behavior documentation and relevant local API/architecture docs; add a changelog entry if present.
- [x] 4.2 Run `go test ./internal/agentconfig ./cmd/server` and `go test -race ./internal/agentconfig ./cmd/server`.
- [x] 4.3 Run `go vet ./internal/agentconfig ./cmd/server` and `go build ./cmd/server`.
- [x] 4.4 Run `npm --prefix desktop run build` and `npm --prefix desktop run test:ui`.
- [x] 4.5 Run `openspec validate 80-remove-project-from-desktop-app --strict` and `git diff --check`; review the final change against every scenario.

Implementation completed and validated. The existing PR remains draft pending the separate review stage.
