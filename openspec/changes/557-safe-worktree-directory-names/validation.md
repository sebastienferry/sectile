# Implementation validation

## Change map

- `internal/agent/worktree_paths.go`: shared key validation, bounded safe basenames, unchanged effective branch fallback, branch-first lookup, and bounded occupied destination selection using `Lstat`.
- `internal/agent/agent_config.go`: primary task creation uses the shared naming and destination policy; existing checkout and remote branch reuse remain intact.
- `internal/agent/agent_operations.go`: workspace, editor, diff and cleanup consumers use the shared resolver; workspace information reports the effective fallback branch.
- `internal/agent/agent_desktop.go`: discussion launches resolve the task's primary repository and actual branch checkout.
- `internal/agent/agent_macro_worktree.go`: safe macro naming with original-key hashing, existing branch/base behavior, and symlink-safe occupied-path refusal.
- `internal/agent/worktree_paths_test.go`: collision/length/input coverage and real Git tests for occupied entries, exhaustion, safe failures, branch fallback, legacy/shared/main/detached lookup, macro naming, and explicit dirty/clean cleanup.
- `internal/agent/worktree_vite_test.go`: opt-in real Chrome acceptance check using production worktree preparation and dependencies installed locally.
- `internal/agent/agent_config_test.go`, `agent_operations_test.go`, `agent_macro_worktree_test.go`, `agent_macro_dispatch_test.go`, and `repositories_test.go`: creation expectations updated; existing base selection, reuse, provisioning and secondary/attached repository coverage retained.
- `internal/agent/agent_desktop_test.go`: discussion launch uses a shared branch checkout while retaining the tracker identity.
- `docs/ARCHITECTURE.md`, `docs/contracts/server-agent-v1.md`, `docs/REIMPLEMENTATION_GUIDE.md`, and `docs/UX_COMPONENTS.md`: describe safe paths, branch-first reuse, collision protection, and legacy/ancestor limitations.
- `CHANGELOG.md`: user-facing Fixed entry under Unreleased.

## Checks

- `go test ./...`: passed.
- `go test -race ./internal/agent ./internal/workspace`: `ok tasks/internal/agent 43.046s`; `ok tasks/internal/workspace (cached)`.
- `go test -race -run TestMacro ./internal/agent`: `ok tasks/internal/agent 6.169s` after final normalized macro-input validation.
- `go vet ./...`, `make fmt-check`, and `git diff --check`: passed without output.
- `make build-all`: `Built server, local agent and desktop app.` Existing frontend chunk-size warnings remain.
- Web `npm test`: 509 tests passed, zero failed. Desktop `npm test`: 145 tests passed, zero failed.
- `npx tsc --noEmit -p tsconfig.app.json`: passed. `npx oxlint src`: passed with existing frontend warnings.
- `npx --no-install openspec validate 557-safe-worktree-directory-names --strict`: `Change '557-safe-worktree-directory-names' is valid`.

Frontend tests ran in a temporary safe-path snapshot at `/private/tmp/sectile-557-checks-cxu49djx`, containing the working sources. The assigned legacy `#557` checkout was retained. The previously documented URL-fragment failure in an absolute frontend test import is outside the legacy-preservation scope; no legacy-path workaround was removed.

## Real Vite acceptance evidence

Command: `SECTILE_VITE_ACCEPTANCE=1 PLAYWRIGHT_MODULE=<absolute desktop/node_modules/playwright/index.mjs> go test -v -run TestGeneratedWorktreeLoadsViteInBrowser ./internal/agent`.

The production preparation function created task `#289` on `feat/289` at:

`/var/folders/5r/p55ynvrj6ylglwg75f6v8nf40000gn/T/TestGeneratedWorktreeLoadsViteInBrowser1502893672/001/.tasks/worktrees/issue-289`

Vite 8.2.0 was installed in that checkout's own `node_modules` directory. A real headless Chrome browser received HTTP 200 for `/src/main.js`, executed the module, rendered `safe-worktree-loaded`, and reported no page errors. Output: `PASS: source module HTTP 200, DOM safe-worktree-loaded`. The test passed in 6.70 seconds and closed the browser/server and removed its temporary fixture.

## Consumer audit and limits

Secondary/attached preparation delegates to primary task preparation. Skill discovery uses Git-enumerated paths. Multi-repository removal resolves actual branch locations, while workspace cleanup retains main-checkout and non-forced removal safeguards. Batch naming is unchanged; shared branch paths are reused. No infrastructure repository changes were needed.

No automatic migration or Vite repair is promised for retained legacy paths. Safe basenames cannot repair unsafe ancestor paths. The acceptance test remains opt-in because it requires npm registry/cache access and installed Chrome/Playwright; ordinary Go tests skip it.
