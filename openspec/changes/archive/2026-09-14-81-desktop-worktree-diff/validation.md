# Validation — Desktop worktree diff (#81)

## Scope and implementation

The confirmed scope is a read-only Desktop comparison of the selected execution's
current checkout with the local default-branch common ancestor, including committed,
staged, unstaged, and non-ignored untracked net contents. No product decisions remain.
The existing draft PR is https://github.com/sebastienferry/sectile/pull/85 on `feat/81`.

- `internal/runner/worktree_diff.go`: explicit checkout/baseline validation; bounded
  subprocess collectors; temporary index/object storage; coherent rename, recreation,
  type-change, binary, symlink, submodule, and special-entry handling; resource limits;
  metadata and index consistency checks with one retry. No new dependencies.
- `cmd/server/agent.go` and `agent_desktop.go`: capture the verified execution branch,
  advertise `git-diff`, and route through existing credential and Origin checks.
- `cmd/server/agent_diff.go`: run-ID-only endpoint, copied run metadata, no-store
  responses, explicit error/status mapping, and no caller-selected checkout.
- `desktop/electron/main.cjs` and `preload.cjs`: narrow authenticated IPC and a clear
  upgrade/restart explanation for old agents.
- `desktop/src/gitDiff.js`, `main.js`, and `style.css`: accessible file/patch viewer,
  context/counts, explicit refresh and errors, stale-request guards, and uninterrupted
  PTY connection with focus restoration. The newly merged skill-result header remains.
- Root/Desktop READMEs, UX guide, local-agent contract, and design document describe
  the delivered behavior and bounds. No CHANGELOG or project memory file existed.

## Review findings resolved

1. Embedded `bytes.Buffer` exposed a `ReadFrom` fast path that bypassed the bounded
   writer. The collector now contains the buffer and records overflow even if Git
   exits through a broken pipe first.
2. Access times change when inspected. Concurrency fingerprints exclude access times
   and retain content-relevant timestamps, inode identity, mode, size and link text.
3. Git represents a regular-file/symlink type change as two patch sections under
   one NUL-delimited file identity. Both sections are grouped and subsequent file
   associations and counters are tested.
4. Git omits untracked FIFOs from `ls-files`. A bounded name-only filesystem walk
   finds special entries, prunes ignored directories and submodules, and never opens
   their contents. `check-ignore --stdin -z` requires omitting global literal-pathspec
   mode because that command already accepts literal names and rejects that option.
5. File/directory replacements remove old index entries before adding new ones in
   the private index; former children of a replaced directory are treated as absent.
6. Repository submodule-ignore settings cannot hide dirty submodules. Uninitialized
   submodules use their index OIDs; initialized submodule HEADs are rechecked.
7. The default branch advanced during implementation. Integrated project task browsing
   and the server-confirmed skill-result header through `origin/main` at `adc4a71`.
   Resolved toolbar, CSS, and documentation conflicts by retaining both features.

PR metadata, reviews, discussion comments and inline review comments were retrieved;
no actionable feedback was present. The complete feature diff was reviewed against
the behavioral specification. Existing local skill/configuration edits were preserved
separately and excluded from feature commits.

## Replayable checks

- [x] `go test ./...` — all packages pass, including `cmd/server` and `internal/runner`.
- [x] `go vet ./...` — exit 0, no diagnostics.
- [x] `make test` — Go internal tests pass; web tests report `tests 22`, `pass 22`,
  `fail 0`; TypeScript and oxlint exit 0. Existing web lint warnings remain in files
  unchanged by this branch.
- [x] `make server-build` — exit 0; output ends with `Done: bin/sectile`.
- [x] `cd desktop && npm run build` — exit 0, 13 modules transformed.
- [x] `node --check desktop/electron/main.cjs` and preload — exit 0.
- [x] `cd desktop && npm run test:ui` — `tests 15`, `pass 15`, `fail 0` (81.1 seconds), including the integrated header feature.
- [x] `openspec validate 81-desktop-worktree-diff --strict` — change is valid.
- [x] `git diff --check` — no whitespace errors.

Go fixtures cover divergent/default-ref selection, dangling/nonstandard refs, shallow
or unrelated/multiple-ancestor history, empty/clean checkouts, overlapping edits and
reversions, recreated deletions, renames and unusual paths, binary/mode/type changes,
symlink target isolation, submodules, ignored tracked paths, cancellation and concurrent
writes, unsupported entries, list and aggregate limits, and real-index/object preservation.
Endpoint tests cover authentication, Origin, methods, stopped/running/unprepared/missing
runs, wrong roots, detached branches, and caller-directory isolation.

The focused Electron scenario passed before and after the earlier base integration.
It exercises file selection, preserved and removed selections, refresh/retry, unsupported
agents, stopped runs, empty/partial/binary states, inert hostile-looking text, keyboard
activation and terminal focus. It asserts no console input, detach or extra attachment
when switching views and discards delayed results after switching executions. A narrow
window screenshot was inspected visually; content remains scrollable without overflow.
The final full-suite outcome is recorded above.

## Environment notes and limits

Tests use a writable `/tmp/sectile-81-go-cache`; HTTP/WebSocket/PTY and Electron tests
require local listener/app access outside the restricted sandbox. Locked npm installs
completed successfully. Vite warns about `#` in the assigned worktree name and large
web chunks; builds succeed. The assigned worktree was retained as requested.

Suggested knowledge-base entry: on macOS, Apple's Git launcher can create `xcrun_db`
under TMPDIR independently of the comparison. Cleanup checks should assert removal
of `sectile-diff-*` request storage, not an entirely empty system temporary directory.

Resource bounds and best-effort concurrency behavior are documented in the Desktop
README and contract. Historical runs inspect present files, and old runs without
branch metadata need a new execution after an agent upgrade. Editing, staging, merge,
network fetch, persistent source snapshots and web source relay remain outside scope.

## Publication

All implementation and review checks pass. Publication reuses PR #85; the workflow
report records the verified pushed commit and ready status. Human merge and ticket
handoff remain separate from this invocation.

## Conflict adjustment — 2026-09-13

PR #85 became conflicting after eight further commits landed on main. Integrated
`origin/main` at `ed4052d` through merge commit `a218048`. The only textual conflict
was in `docs/contracts/server-agent-v1.md`: both branches inserted a contract section
at the same location. Retained both the local worktree comparison and MCP naming
contracts. Reviewed automatic merges in the agent and Desktop task-status code;
Changes, terminal-header results and task-list status indicators remain available.
The previously reviewed comparison engine, endpoint, viewer and focused tests are
unchanged. PR discussion, reviews and inline comments were retrieved; none were present.

Repeated validation on the integrated branch:

- `go test ./...`: all packages pass, including the new MCP contract tests.
- `go vet ./...`: exit 0, no diagnostics.
- `make test`: exit 0; 22 web tests pass, TypeScript and lint succeed.
- `make server-build`: exit 0, `Done: bin/sectile`.
- Desktop build: exit 0, 13 modules transformed.
- Desktop UI suite: 15 passed, 0 failed (85.4 seconds), including Changes and the
  updated terminal/task-list skill-status scenarios.
- Electron main/preload syntax checks, strict OpenSpec validation and whitespace:
  all pass. Previously documented web/Vite warnings remain.

The existing ready PR is reused. Local configuration edits remain excluded from
feature commits and are restored after publication verification. Human merge and
handoff remain pending.
