## Implementation Checklist
- [x] 1. Add pure batch selection/order/name/prompt helpers and tests for distinct full IDs, project membership, ordering, trimming, length boundaries, invalid paths and existing-work preservation.
- [x] 2. Extend Electron and local-agent launch serialization with optional ordered batchTaskIds, preserving single-ticket behavior, explicit project, local target device and autonomous pickup mode. Add relay tests and advertise batch-launch capability.
- [x] 3. Add accessible checkboxes, selected count, clear and prepare actions to Board and Backlog; preserve selection across refresh/filter/view changes within the active project and clear on workspace close/project change. Test both surfaces and navigation/drag/menu regressions.
- [x] 4. Add preparation dialog, reorder controls, suggested/editable name, validation and cancellation. Test keyboard use, exact launch order and first-member lead.
- [x] 5. Submit once through existing launch path; gate unsupported agents, suppress duplicate pending submissions, preserve all preparation state on refusal and clear on acceptance. Test busy/stale/mixed-project rejection and no force or per-member launches.
- [x] 6. Verify existing execution console behavior and server batch membership indicators; confirm the generated prompt requests sequential processing in one shared worktree and one combined PR without overwriting work.
- [x] 7. Add the user-facing CHANGELOG.md Unreleased Added entry. Run Desktop tests/build, relevant Desktop UI tests, Go agent and handler tests, and repository formatting checks. Record results and preserve artifacts for retry.

## Specification Validation
Run `openspec validate 826-desktop-batch --strict`. Verify every requirement below maps to the checklist, and retain `docs/clarifications/826.md` unchanged.

## Implementation Report

### Changes

- `desktop/src/batch-pickup.mjs`: pure membership, order, directory-name and prompt helpers; full IDs and explicit project membership are retained.
- `desktop/src/main.js`: project-scoped checkbox selection in Board and Backlog, count and clear action, preparation snapshot, keyboard reorder controls, editable name, capability gate, one autonomous launch, pending protection and retained preparation on refusal or transport failure.
- `desktop/src/style.css`: batch toolbar, member rows and checkbox layout using existing theme styles.
- `desktop/electron/preload.cjs`, `main.cjs`, `run-project.cjs`: optional ordered membership crosses the existing launch bridge; the main process also checks capability immediately before dispatch. Single-ticket request bodies remain compatible.
- `internal/agent/agent_desktop.go`: advertise `batch-launch`, reject malformed or forced batches, enforce autonomous pickup mode and forward ordered membership with the local device and active project. The existing server remains authoritative for every member's eligibility and busy state.
- `desktop/tests/batch-pickup.test.mjs`, `run-project.test.mjs`, `board.ui.cjs`, `internal/agent/agent_desktop_test.go`: helper boundaries, request compatibility, both selection surfaces, keyboard use, filtering, cancellation, project changes, exact lead/order, busy refusal, transport failure without replay, pending protection, missing members and unsupported agents.
- `CHANGELOG.md`: user-visible Desktop batch preparation and launch entry under Unreleased Added.

### Scope and compatibility

No new process launch, migration, batch persistence, parallel dispatch or execution-sidebar selection flow. Existing server batch membership and console dispatch behavior are reused. Desktop runtime text follows the existing English Desktop surface. The accepted clarification, proposal, design and behavior specification remain unchanged; only this implementation checklist and report are updated.

### Validation

- `cd desktop && npm test`: 284 tests, 284 passed, 0 failed.
- `cd desktop && npm run build`: 1,016 modules transformed; production build passed. Vite reports its existing large-chunk warning.
- `cd desktop && node --test --test-concurrency=1 tests/board.ui.cjs tests/project-open-tasks.ui.cjs`: 19 tests, 19 passed, 0 failed. The final added transport/pending assertions were also rerun with `--test-name-pattern=batch`: 3 passed, 0 failed.
- `GOCACHE=/private/tmp/sectile-826-go-cache go test ./internal/agent ./internal/handlers`: both packages passed. All Desktop agent tests and server Batch tests also passed separately.
- `go vet ./internal/agent ./internal/handlers` and `go build -o /private/tmp/sectile-826-agent ./cmd/agent` with the same writable cache: passed.
- `make fmt-check`, JavaScript syntax checks for the renderer and Electron bridge, and `git diff --check`: passed.
- `openspec validate 826-desktop-batch --strict`: passed.
- Backlog screenshot inspected: checkbox column and toolbar fit the established ticket layout.

Initial sandbox-only runs could not bind loopback test servers or use the default Go cache; checks were rerun with local-server access and a writable temporary cache. A relay fixture initially omitted its required skill directory and was corrected before the successful final checks. No remaining test failure. Accepted artifacts and existing published history were preserved.

## Adjustment Review

- Verified the existing PR #829 is open for `main <- feat/826` before editing. GitHub issue comments, PR comments, reviews and inline review comments were retrieved successfully. PR feedback lists were empty; there is no unaddressed reviewer feedback. Ticket clarification and stage reports were checked against the implementation and require no further change.
- Reviewed the complete branch diff against the accepted requirements, including project membership, ordered lead identity, unsupported agents, cancellation, pending submission, refusal/transport retention, single-ticket compatibility and authoritative server checks. No actionable production defect was found.
- Merged the current default branch without rewriting published history. Removed one trailing blank line from the clarification report so the complete PR diff passes whitespace checks; accepted report content is unchanged.
- Reviewer focus: the renderer preparation snapshot and pending controls, Electron capability check, and agent forwarding of ordered membership with explicit active project/local device. The server remains responsible for member eligibility and busy races.
- Replay checks: `cd desktop && npm test`; `cd desktop && node --test --test-concurrency=1 tests/board.ui.cjs tests/project-open-tasks.ui.cjs`; `cd desktop && npm run build`; `GOCACHE=/private/tmp/sectile-826-go-cache go test ./internal/agent ./internal/handlers`; `go vet ./internal/agent ./internal/handlers`; `go build ./cmd/agent`; `make fmt-check`; `openspec validate 826-desktop-batch --strict`; `git diff origin/main...HEAD --check`.
- Desktop unit tests passed (284/284), Board/Backlog UI tests passed (19/19), production build passed with the existing large-chunk warning, and static analysis, agent build, formatting and strict specification validation passed. The first restricted unit-test attempt could not bind loopback; its rerun with loopback access passed.
