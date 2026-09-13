# Design: Desktop worktree diff

## Context and decisions

The user confirmed a common-ancestor baseline and inclusion of committed, staged, unstaged, and non-ignored untracked changes. ADR 0003 assigns local Git controls to Desktop. The server's `GetTaskGitDiff` uses server paths; `Runner.GetGitDiff` combines patches and has incompatible fallback semantics. Add a dedicated local comparison service instead of calling or silently changing this compatibility API.

The viewer is a current filesystem inspection, not a saved execution snapshot. A historical run still points to the current contents of its recorded checkout. Index-only intermediate edits canceled by unstaged edits do not appear in the net result. This interpretation follows the confirmed clarification.

## Execution identity and routing

- Add `Branch` to local `desktopRun` metadata and populate it with the verified branch returned by `prepareDispatch` in `cmd/server/agent.go`. Keep it in the existing in-memory run record; no database migration or server task update is needed.
- Add authenticated `GET /desktop/git-diff?id=<runID>` in the local agent, routed through `desktopHandler` after its credential and Origin checks. Advertise `git-diff` in `/desktop/status` capabilities.
- Under `runsMu`, look up the run and copy its project ID, task ID, directory, assigned branch, and repository root. Release the lock before Git or filesystem operations. Use recorded metadata, not caller-provided directories or a freshly prepared worktree.
- Verify the directory is an existing Git checkout, its canonical top-level path is the recorded execution directory, its common Git directory matches the recorded repository root, and its current branch matches the recorded branch. Reject missing branch metadata, detached HEAD, unprepared runs, deleted directories, and branch mismatches explicitly.
- Never call `ensureLocalWorktree` on this read path: it can create or prepare repository state. Existing main-checkout reuse remains supported.
- Expose `localAgent.gitDiff(runID)` through `desktop/electron/preload.cjs` and a matching IPC handler in `main.cjs`. Electron's main process retains the token and calls the local endpoint. No server relay is involved. Old agents without the capability produce an update/restart explanation rather than falling back to server data.

## Baseline resolution

Resolve only existing local commit refs, with this deterministic precedence:

1. Resolve symbolic `refs/remotes/origin/HEAD` to its remote-tracking ref. If present but dangling or invalid, report an unavailable baseline rather than substituting another branch.
2. If that symbolic ref is absent, prefer existing `refs/remotes/origin/main`, then `refs/remotes/origin/master`.
3. If neither exists, prefer `refs/heads/main`, then `refs/heads/master`.
4. Otherwise report `baseline_unavailable`. Nonstandard defaults are supported through `origin/HEAD`; arbitrary remote/ref selection is outside this version.

Resolve the selected ref and the task HEAD to commit OIDs. Run `git merge-base --all <baseOID> <headOID>` and require exactly one ancestor. Missing/shallow/unrelated history and multiple best ancestors are explicit errors; there is no two-dot or last-commit fallback. Return the full resolved default ref, its OID, and the ancestor OID. These are local refs as of the read, with no claim to represent the latest remote server state.

## Net comparison service

Add a dedicated module such as `internal/runner/worktree_diff.go` with context-aware helpers and fixture tests. Prefer Git's own patch generation and rename detection; do not introduce a diff library. Use argument arrays, literal path handling, `--` path separators, `--no-ext-diff`, `--no-textconv`, no color, and `GIT_OPTIONAL_LOCKS=0`. Disable filesystem-monitor hooks for inspection. Use bounded stdout/stderr collectors and a request context; do not capture unbounded output and truncate afterward.

1. Capture HEAD, branch, selected default ref OID, and index identity. Read status and path metadata with NUL-delimited formats. Refuse unmerged index entries with `unmerged_index` rather than mislabeling conflict-marker files as an ordinary complete comparison.
2. Use the ancestor-to-working-tree comparison for tracked paths, including staged additions and deletions, mode changes and renames. Generate name/status, numerical statistics, and patch data with consistent options and resolved ancestor OID. Never concatenate an ancestor-to-HEAD patch with a HEAD-to-worktree patch.
3. Enumerate non-ignored untracked files with `git ls-files --others --exclude-standard -z`. Merge by exact path. For an untracked path absent from the ancestor, compare against an empty file. For a recreated path present in the ancestor (for example, a staged deletion), replace any tracked deletion result with a comparison of the ancestor blob against the actual current file. This avoids deletion-plus-addition duplication. Reconcile any rename record consuming that path so every baseline/current path participates only once.
4. Use bounded temporary files outside the checkout where Git needs materialized baseline blobs or no-index comparisons. Treat Git diff exit code 1 as “differences”, not an execution failure. Rewrite temporary patch headers to repository-relative paths. Do not write the real index, object store, refs, or worktree; do not run `git add` or stash operations. Remove temporary files when the request ends.
5. Read symbolic-link text with `lstat`/`readlink`, never follow link targets for file contents. Do not descend into ignored directories or submodules. Describe submodule OID/dirty-state changes with a marker and null text counts. Binary detection uses Git's binary classification for tracked paths and an equivalent bounded check for untracked paths. Unsupported non-regular filesystem entries get an explicit marker.
6. Parse NUL-delimited file identities; do not derive filenames by splitting patch headers on whitespace. Preserve exact paths in JSON, use separate old/new paths for detected renames, and deterministically sort the final list by path. Rename detection is Git's 50% similarity heuristic; when limited, a rename may appear as a deletion plus an addition with a warning instead of a fabricated rename. Count mode-only changes as changed files with zero text changes.
7. Derive file and aggregate counters from the final deduplicated comparison. Binary/submodule/omitted content has null per-file text counts. Aggregate numeric text counts sum known text files only; mark them partial if any text content was omitted. Empty is true only for a complete comparison with no changed entries.
8. Recheck HEAD, branch, default ref, index identity, and the inspected files' metadata after collection. Retry once if changed and time remains; otherwise return `checkout_changed`. A read while an agent writes is best effort, not an atomic filesystem snapshot. Never pause or stop the agent to obtain a snapshot.

## Bounded response contract

Success uses HTTP 200 with a dedicated DTO rather than overloading the legacy server result:

- `runId`, `taskId`, `projectId`, `directory`, `branch`.
- `baseRef`, `baseCommit`, `mergeBase`, `headCommit`, `generatedAt` (UTC).
- `isClean`, `complete`, `countsPartial`, `filesChanged`, `additions`, `deletions`, and `warnings` (code plus human-readable message).
- `files`: `path`, optional `oldPath`, `status` (`added`, `modified`, `deleted`, `renamed`, `type-changed`), `kind` (`text`, `binary`, `symlink`, `submodule`, `unsupported`), nullable `additions`/`deletions`, `patch`, and optional `omittedReason`.

Use a 10-second request deadline including Git and file reads. Initial fixed limits: 1,000 changed entries, 256 KiB of UTF-8 patch data per file, and 4 MiB of serialized response data including metadata. A per-file limit leaves the file entry with `omittedReason`; an aggregate/list limit returns a deterministic prefix with `complete=false` and a warning. Do not split UTF-8 characters. `filesChanged` is the returned entry count when limited and the UI labels it as partial. If a bounded metadata collector cannot establish a valid prefix, return `limit_exceeded` rather than malformed or apparently clean results. These fixed limits require no project configuration changes.

Errors are JSON `{ "error": { "code": "...", "message": "..." } }` for this endpoint: 404 for unknown run; 409 for unprepared/missing/mismatched checkout, unavailable baseline, unmerged index, or detected concurrent change; 413 for a limit preventing a usable result; 504 for timeout; 500 for other Git/read failures. Existing authentication behavior is preserved. Unsupported methods return 405. Messages describe recovery without dumping file contents, credentials, or unbounded subprocess output.

## Desktop presentation

Add Console and Changes controls beside the selected execution toolbar. Put the viewer in a separate module, for example `desktop/src/gitDiff.js`, to keep existing console event handling intact. Mount a file list and patch pane in the article area; hide the terminal visually when Changes is selected without detaching its PTY. Return focus to the terminal and refit it on returning to Console.

Opening Changes fetches once; Refresh triggers another request. Display a loading indicator, context header, summary, and first changed file by default. Keep file selection on refresh when the path remains present. Render paths and patches using text nodes or `textContent`, with line-level styling for additions/deletions/context and horizontal scrolling. No syntax-highlighting dependency is necessary.

Use a monotonically increasing request generation plus selected run ID to discard late results after refresh, selection, closure, or reconnect. Abort obsolete requests where supported. Clear results on task changes. On a failed refresh, either clear the old result or retain it behind a visible stale notice; never leave it looking current. Show named controls, keyboard-operable file buttons, visible focus, a loading status region, and an error alert. No automatic polling or source-content logging/persistence.

## Target files and documentation

- `cmd/server/agent.go`, `cmd/server/agent_desktop.go`, and a focused `cmd/server/agent_diff.go`: branch capture, capability and endpoint wiring, identity checks.
- `internal/runner/worktree_diff.go` and tests: local net comparison, path handling, limits.
- `internal/models/models.go` or a dedicated model file: local diff DTO.
- `desktop/electron/main.cjs`, `desktop/electron/preload.cjs`: bounded IPC access.
- `desktop/src/main.js`, `desktop/src/gitDiff.js`, `desktop/src/style.css`: controls and viewer.
- `cmd/server/agent_desktop_test.go` or a focused diff test file; `desktop/tests/git-diff.ui.cjs`: endpoint and UI coverage.
- During implementation update `desktop/README.md`, root `README.md` (major feature), `docs/UX_COMPONENTS.md`, and `docs/contracts/server-agent-v1.md` or its linked local-desktop contract. No CHANGELOG exists in this checkout. ADR 0003 already covers the boundary; no new architectural decision is introduced.

## Rejected alternatives

- Server diff endpoint: reads the wrong machine for remote execution and has incompatible fallback semantics.
- Latest default tip or selectable baseline: differs from the confirmed common-ancestor scope.
- Committed-only patches or separate concatenated layers: omits ongoing work or duplicates changes.
- Git staging/stashing or worktree preparation to build the diff: changes repository state during an inspection.
- Web access or source relay: changes the accepted local-control boundary unnecessarily.
- New diff/highlighting dependency, file watchers, and persistent snapshots: not required for this initial viewer.

## Risks and verification

Git path encoding, rename pairing, recreated files, and concurrent filesystem reads are the main correctness risks. Cover them using real temporary repositories and compare repository status, index bytes, refs, and file contents before/after requests. Cover authenticated routing and wrong-checkout rejection separately from Git computations. Electron tests use the existing isolated mock-agent pattern and exercise delayed responses, console continuity, accessibility, text escaping, and all result states. Full implementation checks are listed in `tasks.md`; this specification stage runs only OpenSpec validation and documentation whitespace checks.

## Base-branch review

At specification time, the assigned checkout was 30 commits behind `origin/main` and had no branch-only commits. Read-only review of the current default branch confirmed the relevant local run and authentication boundaries remain, with additions to execution timing and Desktop workflow controls. Preserve those newer controls when adding Changes. Integrating the base is an explicit implementation prerequisite; existing unrelated local skill/configuration edits remain untouched during specification.

## Open questions

None. Remaining implementation details may refine helper structure while preserving the behavior and bounds above.
