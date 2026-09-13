# Validation and review

## Outcome
Implemented on `feat/72` for https://github.com/sebastienferry/taskflow/pull/75.
The branch includes remote default `origin/main` at `5df919f` (verified by fetch).

## Changes
- `cmd/server/agent.go`: retain first successful PTY command submission time under the run mutex.
- `cmd/server/agent_desktop.go`: expose optional `startedAt`, omitting zero values.
- `cmd/server/agent_desktop_test.go`: test actual launch time, failed launch omission, unchanged submission time, and completion/API retention.
- `desktop/src/task-order.mjs`: rank task representatives and rows by state, parsed time, and identity without mutating histories.
- `desktop/src/main.js`: apply ordering after archive filtering, retaining selection by run ID.
- `desktop/tests/task-order.ui.cjs`: cover state priority, queue delays, timestamp fallbacks, timezones, identity ties, and unchanged inputs.
- `desktop/tests/task-order-render.ui.cjs`: verify rendered order, refresh stability, selected historical execution, alphabetic projects, and archived-run exclusion.
- `desktop/README.md` and `docs/contracts/server-agent-v1.md`: document display behavior and timestamp semantics.

## Checks
- `openspec validate 72-project-tasks-list --strict`: `Change '72-project-tasks-list' is valid`.
- `go test ./...`: all packages passed, including `tasks/cmd/server 3.623s` and `tasks/internal/terminal 23.546s`.
- `go build -o /tmp/taskflow-72-server ./cmd/server`: exit 0.
- `go vet ./...`: exit 0.
- `npm run build --prefix desktop`: 10 modules transformed; build passed.
- `npm run test:ui` in desktop: 7 tests, 7 passed, 0 failed (22.67s).
- `npm run build` in web: TypeScript and Vite build passed.
- `npm test` in web: 22 tests, 22 passed, 0 failed.
- `npx tsc --noEmit -p tsconfig.app.json` in web: exit 0.
- `npx oxlint src` in web: exit 0; existing React hook/effect warnings in unchanged web source.
- `node --check` for desktop main and ordering module: exit 0.
- `git diff --cached --check`: no whitespace errors.

Build warnings remain for the assigned worktree's `#` character and existing web bundle size. No web source changes are included.

## Review findings and corrections
The first timestamp test assumed missing directories fail launch. The terminal manager creates them, so the test now uses a regular file as its working directory to exercise a real launch failure. The first UI test used a wait shorter than the refresh interval; it now polls for an observed API request. The complete desktop suite passed after these corrections. Archive coverage includes another finished task so an accidentally visible archived timestamp would change the expected order.

The full ticket diff was reviewed against the requirements. No production defects remain. PR #75 was verified open on the expected branch and base; discussion comments, reviews, and inline review comments were retrieved successfully and empty. No remote checks were configured/reported. Existing generated skill edits were preserved outside the ticket commits.

## Replay checklist
- [x] Validate OpenSpec and run the Go build, vet, and full suite.
- [x] Build desktop and run all desktop UI tests.
- [x] Build web and run its tests, typecheck, and lint.
- [x] Verify row ordering and unchanged history/selection in the rendered desktop regression.
- [x] Verify the published implementation head and ready state of the existing PR.

PR #75 was verified open and ready with implementation commit `10076244e565ceee429a3026cb5cf712f343512d`. The final documentation commit records this verification; no production changes followed validation. Human review and merge remain pending.

Suggested project-memory entry: the terminal manager creates missing working directories, so launch-failure tests should use a regular file as the working directory; refresh tests should observe polling instead of assuming an interval.
