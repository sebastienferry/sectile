## Why

Users cannot inspect an agent's current local changes inside TaskFlow Desktop. The legacy server diff endpoint reads the server's filesystem and can concatenate overlapping patches or display an unrelated last commit when a comparison is empty. It therefore cannot represent the agent's actual worktree reliably.

Issue: https://github.com/sebastienferry/taskflow/issues/81. The confirmed clarification is recorded in `docs/clarifications/81.md`.

## What Changes

- Add a read-only Desktop diff viewer for the selected local execution, available while it runs and after it stops while its checkout remains available.
- Compare the current worktree contents with the common ancestor of the task branch and the repository default branch. Include committed, staged, unstaged, and non-ignored untracked changes as one net comparison.
- Show the task, actual directory and branch, resolved default ref and ancestor commit, changed files, text additions/deletions, and a selectable unified diff.
- Refresh on opening and on explicit request; distinguish clean, loading, unavailable, binary, and limited-result states.
- Use the authenticated local agent and existing Electron IPC boundary. Keep local source contents local.

## Capabilities

### New Capabilities

- `desktop-worktree-diff`: Inspect the current contents of a task checkout against its default-branch common ancestor through TaskFlow Desktop.

### Modified Capabilities

None. This extends the accepted Desktop/local-agent architecture without changing web responsibilities.

## Non-goals

Editing, staging, committing, discarding, merging, arbitrary baseline selection, history snapshots, continuous polling, side-by-side rendering, network fetching, recursive submodule inspection, and a web diff viewer are outside this change. The existing server diff API remains compatible and is not the source for the new viewer.

## Impact

- Local agent: capture assigned branch metadata and expose bounded diff reads for an existing execution.
- Git computation: add a dedicated local comparison service with fixture-based tests; do not reuse the legacy fallback semantics.
- Desktop: preload/main IPC, viewer, styles, and Electron UI tests.
- Documentation: update Desktop usage, local-agent contract, and historical UX documentation when implemented.
- No new service, database migration, or runtime dependency is required.
