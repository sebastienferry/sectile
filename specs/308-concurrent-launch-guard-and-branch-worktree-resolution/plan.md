# Implementation plan - #308

Behaviour and acceptance criteria are in [`spec.md`](spec.md). This file records the technical
choices, the data contracts and the files to touch. The ordered checklist is in
[`tasks.md`](tasks.md).

## Stack

Go 1.x server (`internal/handlers`, `internal/db`, `internal/agent`), SQLite through
`internal/db`, Electron desktop (`desktop/electron`, `desktop/src`, plain ESM, no framework),
tests with `go test ./...` and the Node `.cjs` / `.mjs` suites under `desktop/tests`.

## Architecture

Three independent changes, in three layers, plus the desktop surfacing.

### 1. The duplicate guard: a dedicated DB helper and a 409 in `run-skill`

A **new** helper next to `managedStageRunningUnsafe` in `internal/db/skillresult.go`, not an
extension of it. That helper is the transition and postback guard
(`internal/db/stage.go:35`, `internal/db/stage.go:131`, `internal/db/postback.go:104`), and its
skill list deliberately omits `remote_run`; adding it there would silently change when
transitions are blocked. The new one has a single caller and its own semantics.

```go
// ActiveRunOnTask reports the run that makes a task busy for a new launch, or
// nil. A run waiting for user input is active: the session that owns it is
// still there. An agent_launch record is a launch, not a run, and is excluded
// the same way managedStageRunningUnsafe excludes it.
func (d *DB) ActiveRunOnTask(taskID string) (*models.TaskActivity, error)
```

Query shape: `status = 'running'` on `task_activities` for `task_id = ?`, with
`skill_id = 'remote_run' OR skill_id IN (<the nine workflow ids>)`, excluding the
`agent_launch` action expression already used in `managedStageRunningUnsafe`, ordered by
`started_at` so the message names the oldest one. `waiting_since` needs no clause: a waiting run
keeps `status = 'running'` (`internal/db/remoterun.go:297` only sets the column).

The check goes into the `run-skill` sub-action in `internal/handlers/handlers.go`, after the
task is loaded (`handlers.go:1800`, `GetTaskByID`) and **before** `h.agentDispatcher.Lookup` and
the `agent_launch` activity is built (`handlers.go:1817`). Placing it there satisfies NFR1: no
activity exists yet to clean up.

- No active run: fall through unchanged.
- Active run, no `force`: `writeError(w, http.StatusConflict, …)` naming the skill and the start
  time of the active run.
- Active run, `force`: `h.requireOwnerOrAdmin(w, r, active.UserID)` (`internal/handlers/authz.go:102`),
  which already writes the `403` itself; on `ok` fall through. This reuses the rule `cancel-run`
  applies (`internal/handlers/remote_run.go:56`), including its treatment of a run with no
  recorded owner as an admin's to clear.

`force` is a new field on `models.RunSkillRequest` (`internal/models/models.go:946`):

```go
// Force skips the duplicate-launch refusal, and only that. Mode, model and
// workspace checks still apply. Reserved to the active run's owner or an admin.
Force bool `json:"force,omitempty"`
```

The forced path must not close the run it steps over: leaving it alone is what makes `force`
distinguishable from `cancel-run`, and closing it would hand the workflow back for a process
still running.

### 2. Branch-based worktree resolution

`ensureLocalWorktree` (`internal/agent/agent_config.go:262`) keeps its signature
`(ctx, root, task, useWorktrees) (workDir, branch, error)` and its non-worktree path. What
changes is everything after the branch is resolved and validated with `check-ref-format`.

A new unexported helper in the same file:

```go
// worktreeForBranch returns the path of the worktree checked out on branch,
// among every worktree of the repository at root, including the main checkout.
// It returns "" when no worktree carries it.
func worktreeForBranch(ctx context.Context, root, branch string) (string, error)
```

It runs `gitLocal(ctx, root, "worktree", "list", "--porcelain")` and parses the record format:
blank-line separated records of `worktree <path>`, `HEAD <sha>`, then `branch refs/heads/<name>`,
`detached`, or `bare`. Only `branch refs/heads/<name>` records match; a detached or bare worktree
carries no branch. This is the first `git worktree list` call site in the Go code, so there is no
existing parser to reuse.

The new order in `ensureLocalWorktree`:

1. Resolve and validate the branch, as today (lines 263-287, unchanged).
2. `worktreeForBranch`. Non-empty: return that path and the branch. This single step replaces the
   `os.Stat(target)` probe (lines 288-300) **and** the main-checkout fallback (lines 305-316),
   which becomes a special case of the general rule rather than its own branch of code.
3. Branch nowhere: determine the target path. `.tasks/worktrees/<key>` if it is free; if it is
   occupied by another branch, log a warning naming the path and the branch it carries, and use a
   non-colliding sibling path (`.tasks/worktrees/<key>-<sanitised-branch>`) so the launch still
   proceeds (spec US3, last-but-one criterion).
4. `git worktree add`, with the existing local / `origin/` / `HEAD` base selection
   (lines 317-327, unchanged).

The stale-path warning is `log.Printf("[Agent] …")`, matching the logging already in
`prepareDispatch` and `bootstrapLocalMCP`.

The key-safety guard (`agent_config.go:271-274`) stays exactly where it is: it protects the path
join and is unaffected.

`sameDirectory` is kept - `worktreeForBranch` compares git-reported paths, which may differ from
`root` by symlink resolution on macOS, the same reason `sameDirectory` exists.

### 3. Persisting the derived branch

The write belongs to the dispatch caller that owns the task record, not to the git helper, which
stays free of DB and HTTP access (NFR3). `ensureLocalWorktree` already returns the resolved
branch as its second value, which is all the caller needs.

In `internal/agent/agent.go`, right after `prepareDispatch` returns (line 888): when
`config.UseWorktrees` is true, the returned branch is non-empty, and `task.BranchName` is nil or
blank, `PATCH /api/tasks/{taskRef}` with `{"branchName": "<branch>"}`. The exact mechanism
already exists twenty lines below for `prUrl` (`agent.go:922-940`): same helper shape, same
`agenthttp.Client(d.link.token)`, same non-2xx handling - except that here a failure is logged
and the dispatch continues (spec US4), because recording the branch is durability, not a
precondition. Factor the two into one small `patchTask(ctx, taskRef string, fields map[string]string) error`
rather than copying the block.

`internal/db/db.go:2526` already accepts `branchName` on a task update, so no server change is
needed.

`prepareDispatch` is also called from `internal/agent/agent_operations.go:253`
(`prepare_workspace`); that path already persists the branch through
`DB.EnsureTaskWorktree` (`internal/db/db.go:1893-1903`), so it needs no change and must not be
made to write twice.

### 4. The desktop

The refusal already reaches the renderer intact: `desktopRunSkill`
(`internal/agent/agent_desktop.go:779-781`) copies the server's status code and body verbatim,
and `api()` in `desktop/electron/main.cjs:31-37` attaches `status` and the raw body to the thrown
error. So the work is presentation plus one field passed through.

- `desktop/electron/preload.cjs:15` and the `launch-server-task` IPC handler
  (`desktop/electron/main.cjs:178`): carry an optional `force` into the `/desktop/tasks` payload.
- `internal/agent/agent_desktop.go:729-736` (`input` struct) and line 767 (`mustJSON`): accept
  `Force` and forward it to `run-skill`. The agent does not interpret it, exactly as it does not
  interpret `Mode`.
- `desktop/src/main.js`, the `#next-step` click handler (line 1699-1718): on a caught error with
  `status === 409`, parse the JSON body for `error` and store it in `nextStepErrors` as the
  message, plus a flag that makes `renderNextStep` (line 1655-1676) show a "Launch anyway"
  control. That control re-runs the same launch with `force: true`. The local `busy` guard
  (line 1666) is untouched.

The `409` the desktop already produces for a finished task
(`agent_desktop.go:748`) and the `409` for "Connect the local agent"
(`handlers.go:1894`) are not forceable; the renderer must key the "Launch anyway" control on a
marker in the body, not on the status code alone. Simplest: the server's refusal body carries a
distinguishable payload, `{"error": "…", "activeRunId": "<id>"}`, and the desktop offers `force`
only when `activeRunId` is present.

## Data contracts

| Contract | Change |
| --- | --- |
| `POST /api/tasks/{id}/run-skill` request | new optional `force` boolean |
| `POST /api/tasks/{id}/run-skill` response | new `409` with `{"error": string, "activeRunId": string}`; new `403` when `force` comes from neither owner nor admin |
| `POST /desktop/tasks` (agent) request | new optional `force` boolean, forwarded untouched |
| `launchServerTask` IPC | new trailing optional `force` argument |
| Task record | `branchName` written by the agent after a derived-branch worktree creation |
| Database schema | **unchanged** - no migration, no new column |

## Target files

| File | Change |
| --- | --- |
| `internal/db/skillresult.go` | add `ActiveRunOnTask` |
| `internal/models/models.go` | `RunSkillRequest.Force` |
| `internal/handlers/handlers.go` | the 409 / force / 403 branch in `run-skill`, before the activity |
| `internal/agent/agent_config.go` | `worktreeForBranch`, rewritten resolution in `ensureLocalWorktree`, stale-path warning |
| `internal/agent/agent.go` | `patchTask` helper, derived-branch persistence after `prepareDispatch`, `prUrl` write refactored onto it |
| `internal/agent/agent_desktop.go` | `Force` on the desktop input, forwarded to `run-skill` |
| `desktop/electron/main.cjs`, `desktop/electron/preload.cjs` | `force` through the IPC bridge |
| `desktop/src/main.js` | render the 409 message, "Launch anyway" control |
| `internal/agent/agent_config_test.go` | rewrite `TestLocalWorktreeCreationAndBranchGuard` (l.46-61), keep `TestLocalWorktreeReusesMainCheckoutForDerivedBranch` (l.105) |

## Rejected alternatives

- **Extending `managedStageRunningUnsafe`.** Its four callers decide when transitions and
  postbacks are blocked; adding `remote_run` to its list would change that behaviour silently.
- **Probing each candidate worktree directory with `os.Stat` + `branch --show-current`.** It
  cannot see a worktree at a path nobody thought to probe, which is precisely the `#296` failure.
- **Refusing the launch when `task.BranchName` is empty (fix 3, strict reading).** It breaks the
  first launch on every new task, the normal path, which
  `agent_config_test.go:105` encodes deliberately.
- **A staleness threshold on the active run.** It invents a timer, and a wrong threshold either
  reintroduces the collision or blocks a legitimate relaunch. `force` covers the case with no
  clock involved.
- **Making MCP `start_run` refuse a second declaration.** A CLI session is the authority on its
  own run; refusing it breaks the sessions the guard exists to protect.

## Risks

- **Worktree parsing on an unusual repository.** A bare or detached worktree must be skipped, not
  matched. Covered by a dedicated test.
- **Path collision when the key path is occupied.** The sibling-path fallback must produce a
  valid, non-colliding directory name from an arbitrary branch name; reuse
  `db.SanitizeBranchName` (`internal/db/db.go:1875`) rather than writing a third sanitiser.
- **Residual risk the ticket names is narrowed, not closed.** `sharesCheckout`
  (`internal/agent/agent_run.go:236-255`) still only knows the agent queue, so two agents can
  still write the same tree without seeing each other. Out of scope here, and stated as such.
