# Verification

## Changes and decisions
- `internal/agentconfig/config.go` and `internal/db/agentconfig.go`: optional effective GitHub repository and tracker metadata without credentials or server paths; project-over-global precedence.
- `cmd/server/agent_config.go` and `cmd/server/agent.go`: propagate the task fetched during preparation and resolved local directory/branch through shared task dispatch.
- `cmd/server/agent_template.go`: single-pass argument interpolation for eight tokens, empty/repeated/embedded values and ordinary shell quoting. Protect interactive history expansion by enclosing inserted values in single quotes even within a double-quoted argument.
- Go regression tests: shell round trips in sh/bash/zsh, interactive bash/zsh history, current task context, metadata fallback and precedence, configuration secrecy, local worktree paths and launch routes.
- Desktop help/UI test, root README, desktop README and server-agent contract: document available parameters and local execution semantics.

## Reproducible checks
- [x] `openspec validate 63-desktop-prompt-placeholders --strict`: `Change '63-desktop-prompt-placeholders' is valid`.
- [x] `go test ./...`: all packages passed; `ok tasks/cmd/server 2.572s`, other tested packages passed from cache on final run.
- [x] `go build ./...`: exit 0.
- [x] `go vet ./...`: exit 0.
- [x] `npm run build --prefix web`: TypeScript and Vite build passed.
- [x] `npm test --prefix web`: 22 passed, 0 failed.
- [x] `npm run lint --prefix web`: exit 0 with existing React warnings also present on the untouched baseline.
- [x] `npm run build --prefix desktop`: Vite build passed.
- [x] `npm run test:ui --prefix desktop`: 4 passed, 0 failed, including all eight help tokens and settings inheritance/reset/refresh flows.
- [x] `git diff --check`: no whitespace errors.

Baseline checks passed before implementation. Vite reports the existing worktree `#` path warning and the existing web bundle-size warning; builds succeed. Electron tests use local mock servers and do not launch real coding clients.

## Review
Reviewed the complete task diff against fetched `origin/main` (`bfc68e5`), requirements, changed tests, and existing PR feedback. PR #74 had no reviews, comments, inline comments or configured checks at review time. No unresolved code findings. The interactive history edge case was fixed and regression-tested. PR #74 was updated and verified open and ready for review with implementation commit `0c83ddc`: https://github.com/sebastienferry/sectile/pull/74. No remote CI checks are configured; local check evidence is recorded above.

Server execution redesign, arbitrary shell-program interpolation, raw terminal commands and interactive launches without a skill remain outside the requested scope. The template itself is trusted shell code; placeholders are ordinary CLI argument data. No new dependency or migration is required. Existing unrelated local workflow changes remain preserved outside this PR.
