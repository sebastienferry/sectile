# Plan #755 - Desktop: archiving a task removes its worktree

## Stack

Go agent (`internal/agent`), Go server (`internal/db`, `internal/handlers`),
shared protocol (`internal/agentprotocol`, `internal/models`), Electron
Desktop (`desktop/electron`, `desktop/src/main.js`). No migration, no new
stored data: the archive state stays in the Desktop's `localStorage`.

## Flow

```
Desktop archiveTask
  -> IPC archive-workspace {projectId, taskId}          (desktop/electron/main.cjs)
  -> agent POST /desktop/tasks/archive-workspace         (relay, like /desktop/tasks/transition)
  -> server POST /api/tasks/{id}/archive-workspace       (decides scope and branch policy)
  -> agent operation archive_workspace                   (Git work on the workstation)
  <- models.WorkspaceArchive                             (per-repository outcome)
Desktop: archivable -> hide runs; otherwise dialog "Task not archived"
```

The server owns the decision because it holds what the agent does not: the
repositories the ticket changed (`TaskPrimaryRepository`,
`taskChangedRepositories`), the pull request states (`PrLinks[].State`), and
the other tasks of the project (shared branch). The agent owns the Git work,
since the checkouts are on its machine.

## Data contracts

### Operation (`internal/agentprotocol`)

New action `archive_workspace`, added to `Operations` so the server names an
agent that predates it (`UnsupportedOperationError`, "update the agent").
Fields used:

- `TaskID`, `ProjectID`, `UserID` (the caller, so the operation reaches the
  agent that asked).
- `Repositories`: the primary repository then each changed one, as computed by
  `RemoveTaskWorktree` today; empty for a single-repository task, which keeps
  the operation on the task's root (`primaryRoot`).
- New `DeleteBranch bool`: the server found at least one pull request on the
  task and all of them merged.

### Result (`internal/models/repository.go`)

```go
type WorkspaceArchive struct {
    Repositories []WorkspaceArchiveEntry `json:"repositories"`
}
type WorkspaceArchiveEntry struct {
    Repository string `json:"repository"`          // identity, "" for the project checkout
    Role       string `json:"role"`                // "code" or "specifications"
    Path       string `json:"path,omitempty"`      // the worktree, when one was found
    Outcome    string `json:"outcome"`             // removed | absent | shared | disabled | failed
    Error      string `json:"error,omitempty"`     // Git's reason, for failed
    Branch     string `json:"branch,omitempty"`
    BranchOutcome string `json:"branchOutcome,omitempty"` // deleted | kept
    BranchReason  string `json:"branchReason,omitempty"`  // no-merged-pr | no-upstream | unpushed | checked-out | missing | error text
}
func (a WorkspaceArchive) Archivable() bool // no entry failed
```

The server answers `{"archivable": bool, "repositories": [...]}`.

## Agent (`internal/agent`)

New file `archive_workspace.go`, dispatched from `executeOperation` next to
`remove_workspace`, after the task is read:

1. `config.UseWorktrees` false: one entry `disabled`, nothing touched.
2. Branch: `task.BranchName`, else `taskWorktreeBranch(task, format)`. None:
   every entry `absent`.
3. Code worktrees: for each repository in `op.Repositories`, its folder via
   `repositoryFolder` (not found: `failed`, "not found on this workstation");
   without repositories, the task root via `primaryRoot`. Then
   `worktreeForBranch(root, branch)`: none, or the main checkout itself:
   `absent`; a worktree whose folder was deleted by hand is pruned
   (`git worktree prune`) and `absent`; else `git worktree remove <path>`
   through `gitLocal`: `removed` or `failed` with Git's message.
4. Specifications worktree: when `overrides.IssueSpecPath(projectID)` names a
   distinct Issue folder (`localIssueSpecRepo`), the same lookup and removal
   there, role `specifications`.
5. Branch, per entry `removed` or `absent`, only when `op.DeleteBranch`:
   `refs/heads/<branch>` exists (else `missing`), no worktree holds it (else
   `checked-out`), `git rev-parse --abbrev-ref <branch>@{upstream}` succeeds
   (else `no-upstream`), `git rev-list --count <branch>@{upstream}..<branch>`
   is 0 (else `unpushed`), then `git branch -D <branch>`. A failure of any of
   these keeps the branch and records the reason; it never turns the entry
   into `failed`. Without `DeleteBranch`: `kept`, `no-merged-pr`.

Every Git call goes through `gitLocal` (`agentexec.Hidden` on Windows).

### Desktop route

`POST /desktop/tasks/archive-workspace?projectId=` with `{taskId}`, in
`agent_desktop.go`, modelled on `desktopTaskTransition`: checks the project
with `fetchConfig`, relays to the server with the device token, copies the
status and body back. Status capability `archive-workspace`.

## Server

- `internal/db`: `ArchiveTaskWorkspace(ctx, userID, taskID) (*models.WorkspaceArchive, error)`:
  - reads the task and its project;
  - shared: another task of the project records the same non-empty
    `BranchName`: returns one `shared` entry per repository with the branch
    kept, without calling the agent (nothing to verify);
  - `DeleteBranch`: `PrLinks` non-empty and every `State == "merged"`;
  - repositories as in `RemoveTaskWorktree` when `multiRepoTask`;
  - `callAgentContext` with `UserID`; an empty answer from an agent
    (no entry) is an error "update the agent".
- `internal/handlers`: `POST /api/tasks/{id}/archive-workspace`, next to the
  `worktree` sub-action; user from `webSessionUser(r)`; 404 for an unknown
  task, 502 with the message when the agent cannot be reached or refuses,
  200 with `{archivable, repositories}` otherwise.

## Desktop

- `desktop/electron/preload.cjs`: `archiveWorkspace(projectId, taskId)`.
- `desktop/electron/main.cjs`: IPC `archive-workspace`; refuses with "The
  running local agent cannot remove a task's worktree. Update and restart the
  agent before archiving." when the status lacks `archive-workspace`. The
  existing 60 s timeout of `POST /desktop/tasks*` applies.
- `desktop/src/main.js`:
  - `archiveTask(run)`: after the active-run check and before hiding, unless
    `freeConsole(run)` or `macroRun(run)`, calls `api.archiveWorkspace`; a
    refusal throws an error whose message lists the blocking repositories.
  - The text is built by a pure helper in a new module
    `desktop/src/archive-workspace.mjs` (`archiveRefusal(result)`), unit
    tested with `node --test`: one line per failed entry, `<repository or
    "Project checkout"> (<role>): <error>`, then the advice "Commit or discard
    the changes, then archive again." A transport error is prefixed with
    "Not archived: " and followed by "Start or update the local agent, then
    archive again." when the agent is unreachable or too old.
  - `requestArchive`: the confirmed "Stop and archive" path already shows the
    error in its dialog and re-enables the button. The direct path opens a
    dialog "Task not archived" with the message instead of a passing notice.
    The archive button is disabled while the call runs.
  - Tooltip and aria-label: "Archive <task> and remove its worktree" /
    "Stop, archive <task> and remove its worktree" for ticket tasks; unchanged
    for free consoles and macro runs. The active-execution dialog reads "They
    must stop before it can be archived and its worktree removed."

## Tests

- Agent (real Git repositories in `t.TempDir()`, the `worktree_paths_test.go`
  style): clean worktree removed; dirty worktree failed and kept; absent;
  worktrees off; branch deleted when merged and pushed; kept with no upstream,
  unpushed commits, or `DeleteBranch` false; multi-repository with one
  repository not found; distinct specifications worktree removed.
- Agent desktop route: relays to the server and copies its status; capability
  announced. `agentprotocol` operation list tests already iterate
  `Operations`.
- DB: `ArchiveTaskWorkspace` sends `DeleteBranch` only when every recorded pull
  request is merged; shared branch answers without calling the agent;
  repositories for a multi-repository task; empty agent answer is an error.
- Handler: route answers 200 with the result, 404 for an unknown task.
- Desktop: `archive-workspace.test.mjs` for the message; UI tests that archive
  (`console.ui.cjs`, `task-rename.ui.cjs`, `pr-display.ui.cjs`, the shared
  `fake-agent.cjs`) answer the new route and capability, and one UI case
  shows a refusal that keeps the task visible.

## Target files

- `internal/agentprotocol/operations.go`
- `internal/models/repository.go`
- `internal/agent/archive_workspace.go` (new), `agent_operations.go`,
  `agent_desktop.go`
- `internal/db/` (new `archive_workspace.go`), `internal/handlers/handlers.go`
- `desktop/electron/preload.cjs`, `desktop/electron/main.cjs`,
  `desktop/src/main.js`, `desktop/src/archive-workspace.mjs` (new)
- `CHANGELOG.md`

## Rejected alternatives

- **Desktop calls the existing `DELETE /api/tasks/{id}/worktree`.** It
  answers only success or a joined error string, cannot tell "absent" from
  "removed", ignores the specifications worktree, and errors when worktrees
  are off. Changing it would change a route the web may call later; a new
  route keeps it as it is.
- **The agent decides alone.** It would have to fetch every task of the
  project to find a shared branch and re-derive the changed repositories the
  server already computes.
- **`git branch -d`.** Squash merges leave the branch unmerged for Git; the
  owner settled on the pull request state instead.
