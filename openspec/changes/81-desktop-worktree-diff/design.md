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

1. Capture HEAD, branch, default-ref OID and an index-content digest. Read tree and index identities with NUL-delimited formats; reject unmerged entries. Capture file metadata without following symlink contents.
2. Ask Git for ancestor-to-working-tree changed paths and enumerate non-ignored untracked files. Deduplicate exact paths. This includes recreated staged deletions, whose actual current content replaces the baseline entry once.
3. Initialize a **temporary** index from the ancestor with `read-tree`, and a temporary object directory with the real objects available as read-only alternates. Materialize changed regular-file or symlink contents as Git loose blobs in that temporary directory, update the temporary index with NUL-delimited `update-index --index-info`, and write its temporary tree. No `git add`, real-index update, source staging, or repository-object write occurs. Temporary files are removed on every return.
4. Compare the ancestor and temporary tree with Git's own rename detection at 50% similarity (`-l1000`). Collect name/status, numstat, and patches in separate bounded processes. Patch segments follow Git's file order; their headers do not determine file identities. One process per output kind avoids subprocess costs growing with the file count. Binary/submodule markers have null counts; mode-only changes have zero text counts.
5. Keep symlink text separate from target contents and represent unsupported entries explicitly. Submodule entries use recorded index OIDs when uninitialized and checked-out OIDs when initialized, with dirty markers; do not traverse their source files into the comparison. Read regular files only after checking the opened file identity. Non-UTF-8 names produce a named error; non-UTF-8 contents are marked non-text.
6. Sort the final records by exact path and apply the response/list limits. Omitted content has null text counts; numeric aggregates sum displayed known text changes only. Use 8 MiB bounds for metadata and individual content reads, plus a 64 MiB temporary-content budget, in addition to the specified display limits. These internal resource bounds can produce explicit incomplete results or a metadata-limit error.
7. Recheck Git identity, index digest, changed-path and included-path sets, inspected file metadata (excluding access times), and inspected submodule HEADs. Retry once on change within the 10-second deadline. This is best-effort live inspection, not an atomic snapshot.

The temporary Git storage replaces the initially proposed per-file no-index comparisons. It preserves the behavioral contract while letting Git reconcile recreated paths and renames coherently, and avoids invoking a subprocess for each patch. Loose-object encoding uses the repository's SHA-1 or SHA-256 object format and standard zlib; fixture tests prove objects remain outside the real repository.

## Bounded response contract

Success uses HTTP 200 with a dedicated DTO rather than overloading the legacy server result:

- `runId`, `taskId`, `projectId`, `directory`, `branch`.
- `baseRef`, `baseCommit`, `mergeBase`, `headCommit`, `generatedAt` (UTC).
- `isClean`, `complete`, `countsPartial`, `filesChanged`, `additions`, `deletions`, and `warnings` (code plus human-readable message).
- `files`: `path`, optional `oldPath`, `status` (`added`, `modified`, `deleted`, `renamed`, `type-changed`), `kind` (`text`, `binary`, `symlink`, `submodule`, `unsupported`), nullable `additions`/`deletions`, `patch`, and optional `omittedReason`.

Use a 10-second request deadline including Git and file reads. Initial fixed limits: 1,000 changed entries, 256 KiB of UTF-8 patch data per file, and 4 MiB of serialized response data including metadata. A per-file limit leaves the file entry with `omittedReason`; an aggregate/list limit returns a deterministic prefix with `complete=false` and a warning. Do not split UTF-8 characters. `filesChanged` is the returned entry count when limited and the UI labels it as partial. If a bounded metadata collector cannot establish a valid prefix, return `limit_exceeded` rather than malformed or apparently clean results. These fixed limits require no project configuration changes. The additional internal snapshot and metadata bounds above also produce explicit omissions or errors.

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
