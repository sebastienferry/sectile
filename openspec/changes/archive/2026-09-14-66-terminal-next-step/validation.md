# Implementation and review

## Changes
- `desktop/src/workflow.mjs`: resolve workflow labels and status aliases and identify the next configured skill.
- `desktop/src/main.js`: show the task footer, refresh metadata on selection and every 15 seconds, recheck before dispatch, block active/duplicate submissions, and contain errors within the footer.
- `desktop/src/style.css`: wrap the footer, refit the console when footer height changes, and contain narrow-window overflow.
- `desktop/tests/workflow.ui.cjs` and `desktop/tests/next-step.ui.cjs`: exercise workflow states, configured skills, launch errors, stale state, duplicate clicks, historical console guards, selection races, and narrow-window positioning.
- `README.md` and `desktop/README.md`: explain the next-step control and manual merge boundary.

## Verification
- `openspec validate 66-terminal-next-step --strict`: Change '66-terminal-next-step' is valid.
- `npm --prefix desktop run build`: 9 modules transformed; build passed.
- `npm --prefix desktop run test:ui`: 4 tests passed, 0 failed.
- `npm --prefix web run build`: TypeScript and Vite passed; 2095 modules transformed.
- `npm --prefix web run lint`: exit 0; existing warnings in unchanged React files remain.
- `npm --prefix web test`: 19 tests passed, 0 failed.
- `web/node_modules/.bin/oxlint desktop/src/main.js desktop/src/workflow.mjs desktop/tests/next-step.ui.cjs desktop/tests/workflow.ui.cjs`: exit 0, no diagnostics.
- `go test ./internal/...` and `go test ./cmd/server`: all tested packages passed.
- `go vet ./...` and `go build -o /tmp/sectile-66 ./cmd/server`: exit 0.
- `git diff --check`: no whitespace errors.

Vite reports the assigned worktree's `#` character and the existing web bundle size as warnings. Initial sandbox attempts could not bind local test servers or access the Go cache; the authorized reruns passed.

## Review findings resolved
Launch errors originally used the global overlay, which covered the new action; they now appear in the footer with retry available. Submission tracking clears on any new execution, including one that fails before a console opens. Metadata request generations prevent stale selection responses from replacing the current task. A footer ResizeObserver keeps terminal dimensions current; narrow-window overflow is contained.

The branch was compared with fetched `origin/main` at d87ad676ea649db21bf260a3299c9ddd4dbdabe0, with no missing base commits. Existing generated workflow skill edits are preserved locally and excluded from this change. No new architecture or API is introduced. Merge and task closure remain human actions.
