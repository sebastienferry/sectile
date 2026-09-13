## 0. Implementation preparation

- [ ] 0.1 Integrate the current default branch before implementation, preserving existing local skill/configuration edits. Recheck the execution metadata and Desktop toolbar integration points against the updated code.

## 1. Local comparison and identity

- [ ] 1.1 Add the local diff DTO and capture the verified assigned branch in execution metadata without changing server task storage.
- [ ] 1.2 Implement read-only checkout identity validation, deterministic local default-ref resolution, and unique common-ancestor resolution with explicit errors.
- [ ] 1.3 Implement the bounded ancestor-to-current-content comparison, deduplicated untracked/recreated paths, robust filenames, metadata changes, and consistent counts.
- [ ] 1.4 Add binary, symlink, submodule, unsupported-entry and size-limit handling, cancellation, bounded subprocess output, temporary-file cleanup, and concurrent-change detection.
- [ ] 1.5 Add real Git fixture tests: divergent default branch, nonstandard default, ref precedence, dangling default, shallow/unrelated history, multiple ancestors, empty repository, clean checkout, overlapping committed/staged/unstaged edits, reversions, untracked/ignored files, tracked ignored files, staged deletion with recreation, additions/deletions, renames, spaces/tabs/newlines/Unicode paths, executable bits, symlinks, binaries and submodule markers.
- [ ] 1.6 Prove inspection preserves file contents, index bytes, refs and branch and does not execute external diff/textconv/fsmonitor helpers; cover bounds, timeout/cancellation, unmerged entries and detected concurrent changes.

## 2. Local agent and Electron bridge

- [ ] 2.1 Add the authenticated run-ID-based GET endpoint and capability, copying metadata under lock before performing Git reads.
- [ ] 2.2 Test known running/stopped runs, reused main checkout, missing/unprepared/deleted runs, wrong branch/root, detached HEAD, credentials, Origin rejection, method handling, error mapping, and no caller-selected directory access.
- [ ] 2.3 Add the narrow preload/main IPC method and unsupported-agent handling without a server-source fallback.

## 3. Desktop viewer

- [ ] 3.1 Add accessible Console/Changes controls and a file-list/unified-patch viewer with baseline context, status, counts and non-text markers.
- [ ] 3.2 Implement load-on-open, Refresh, file selection retention, loading/empty/error/stale/partial states and response-generation guards.
- [ ] 3.3 Preserve PTY connection and execution state while switching views; restore terminal focus and size on return.
- [ ] 3.4 Add isolated Electron UI tests for selecting files, refresh and removed selection, delayed response after task switch, failure and retry, unsupported agents, stopped runs, limits, binary display, inert hostile-looking content and keyboard navigation.
- [ ] 3.5 Verify opening/closing Changes does not send input to, stop, detach or restart the console; verify narrow-window scrolling and focus visibility.

## 4. Documentation and validation

- [ ] 4.1 Update root and Desktop READMEs for the delivered feature, refresh semantics, baseline resolution and limitations; update UX documentation and local-agent API contract. Keep future behavior clearly separate until implemented.
- [ ] 4.2 Run focused tests while developing, then `go test ./...` and `go vet ./...` (includes `cmd/server`, omitted by the current Makefile test target).
- [ ] 4.3 In `desktop`, run `npm run build` and `npm run test:ui`; run `node --check` on changed CommonJS bridge files.
- [ ] 4.4 Run `make test` for existing backend/web checks and `make server-build` to verify the shared server/agent executable builds.
- [ ] 4.5 Run `openspec validate 81-desktop-worktree-diff --strict` and `git diff --check`, document actual outcomes, and review the implementation against every scenario before ready-for-review status.

These are implementation tasks and intentionally remain unchecked at specification stage. The specification is reviewed through a draft PR before code is implemented.
