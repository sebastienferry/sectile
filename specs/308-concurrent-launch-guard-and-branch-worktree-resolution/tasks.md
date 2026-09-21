# Implementation checklist - #308

Behaviour in [`spec.md`](spec.md), technical choices in [`plan.md`](plan.md). The order matters:
each block leaves the tree building and the suite green.

## Block A - Server-side duplicate guard

- [x] **A1** Add `ActiveRunOnTask(taskID string) (*models.TaskActivity, error)` to
      `internal/db/skillresult.go`. `status = 'running'`, `skill_id = 'remote_run'` or one of the
      nine workflow ids, the `agent_launch` action expression excluded as in
      `managedStageRunningUnsafe`, ordered by `started_at` ascending, first row or nil. Leave
      `managedStageRunningUnsafe` untouched.
- [x] **A2** Add `Force bool \`json:"force,omitempty"\`` to `models.RunSkillRequest`
      (`internal/models/models.go:946`) with a comment stating it skips the duplicate check and
      nothing else.
- [x] **A3** In the `run-skill` sub-action (`internal/handlers/handlers.go`), after `GetTaskByID`
      and before the dispatcher lookup and the `agent_launch` activity: call `ActiveRunOnTask`.
      No active run, continue. Active run and no `force`, answer
      `409` with `{"error": …, "activeRunId": …}` naming the skill and the run's start time.
      Active run and `force`, call `h.requireOwnerOrAdmin(w, r, active.UserID)` and return when it
      is not ok; otherwise continue without touching the active run.
- [x] **A4** Tests in `internal/handlers/` (new `run_skill_guard_test.go`, following
      `agent_launch_test.go` for the fixture shape):
      - a running `remote_run` refuses the launch with `409` and records no activity;
      - a run with `waiting_since` set refuses identically;
      - a completed run, and an `agent_launch` alone, do not refuse;
      - a running workflow-skill activity refuses;
      - `force` from the owner passes and leaves the active run `running`;
      - `force` from an admin on another user's run passes;
      - `force` from a third party answers `403`;
      - `force` with an invalid mode still answers `400`.

## Block B - Branch-based worktree resolution

- [x] **B1** Add `worktreeForBranch(ctx, root, branch string) (string, error)` to
      `internal/agent/agent_config.go`: run `git worktree list --porcelain`, parse the
      blank-line-separated records, match only `branch refs/heads/<name>`, skip `detached` and
      `bare`, return the matching `worktree <path>` or `""`.
- [x] **B2** Rewrite the resolution in `ensureLocalWorktree` (`agent_config.go:288-316`): keep
      branch derivation and `check-ref-format` as they are, then call `worktreeForBranch` and
      return its result when non-empty. Delete the `os.Stat(target)` refusal and the separate
      main-checkout fallback, which this subsumes.
- [x] **B3** When the branch is nowhere: target `.tasks/worktrees/<key>` if free; if it exists on
      another branch, `log.Printf` a warning naming the stale path and its branch, and fall back
      to `.tasks/worktrees/<key>-<SanitizeBranchName(branch)>`. The sanitiser moved from `db` to
      `models`: `cmd/server/runtime_boundary_test.go` forbids the agent binary from linking the
      database package, so the plan's `db.SanitizeBranchName` call could not stand. `db` keeps its
      exported name and delegates, so there is still one implementation. Then `git worktree add` with the
      existing local / `origin/` / `HEAD` base selection, unchanged.
- [x] **B4** Rewrite `TestLocalWorktreeCreationAndBranchGuard`
      (`internal/agent/agent_config_test.go:46-61`): the mismatched-branch case must now succeed
      by resolving elsewhere or creating a new worktree, not fail. Keep the `../../escape` key
      assertion exactly as it is.
- [x] **B5** Keep `TestLocalWorktreeReusesAssignedMainCheckout` and
      `TestLocalWorktreeReusesMainCheckoutForDerivedBranch` passing unchanged: they are the
      regression net for the fallback the rewrite absorbs.
- [x] **B6** New tests in `agent_config_test.go`:
      - the assigned branch lives in a linked worktree at an unrelated path: it is reused, no new
        worktree is created;
      - `.tasks/worktrees/<key>` sits on another branch while the assigned branch lives in a third
        worktree: the third one is used and the launch succeeds (the `#296` failure);
      - `.tasks/worktrees/<key>` sits on another branch and the assigned branch is nowhere: a
        worktree is created at a non-colliding path;
      - a detached worktree is not matched as carrying a branch.

## Block C - Persisting the derived branch

- [x] **C1** Extract the `prUrl` PATCH block (`internal/agent/agent.go:922-940`) into
      `patchTask(ctx context.Context, taskRef string, fields map[string]string) error`, and make
      the `adjust` path call it. Behaviour unchanged.
- [x] **C2** After `prepareDispatch` returns (`agent.go:888`): when `config.UseWorktrees`, the
      returned branch is non-empty and `task.BranchName` is nil or blank, call
      `patchTask(ctx, taskRef, map[string]string{"branchName": branch})`. Log a failure and carry
      on; it must never abort the dispatch.
- [x] **C3** Verify `internal/agent/agent_operations.go:253` (`prepare_workspace`) is unaffected:
      it persists through `DB.EnsureTaskWorktree` and must not write the branch twice.
- [x] **C4** Test: a dispatch on a task with no `branchName` issues one `PATCH` carrying the
      derived name; a dispatch on a task that already has one issues none. Extend the existing
      `prepareDispatch` fixture at `agent_config_test.go:276`, whose stub server already serves
      `/api/tasks/…`.

## Block D - Desktop

- [x] **D1** `internal/agent/agent_desktop.go`: add `Force bool` to the `desktopRunSkill` input
      struct (l.729) and to the `mustJSON` payload sent to `run-skill` (l.767). The agent does not
      interpret it.
- [x] **D2** `desktop/electron/preload.cjs:15` and the `launch-server-task` handler
      (`desktop/electron/main.cjs:178`): carry an optional trailing `force` into the
      `/desktop/tasks` body. Existing call sites that omit it keep working.
- [x] **D3** `desktop/src/main.js`, `#next-step` click handler (l.1699-1718): on a caught error,
      parse the body as JSON; when it carries `activeRunId`, store its `error` text in
      `nextStepErrors` and record that a forced retry is offered for this task key.
- [x] **D4** `renderNextStep` (l.1655-1676): when a forced retry is offered, show a "Launch
      anyway" control next to the status, whose click re-sends the same launch with
      `force: true` and renders the outcome in place. Leave the local `busy` guard (l.1666)
      unchanged.
- [x] **D5** UI test `desktop/tests/concurrent-launch.ui.cjs`, modelled on
      `next-step.ui.cjs`: a `409` carrying `activeRunId` shows the server message and the
      "Launch anyway" control; clicking it re-sends with `force: true`; a `409` without
      `activeRunId` (finished task, agent disconnected) shows the message and **no** control.

## Block E - Verification and documentation

- [x] **E1** `go build ./... && go vet ./... && go test ./...` from the worktree root.
- [x] **E2** `npx vite build` in `desktop/`, then the desktop suites (the `.ui.cjs` tests load
      `dist/index.html`, so the build comes first), then `node --test` over
      `desktop/tests`.
- [x] **E3** The `#296` residue is reproduced as a test rather than checked by hand:
      `TestLocalWorktreeResolvesBranchWhereverItLives` builds the exact shape - the assigned
      branch in an unrelated worktree, `.tasks/worktrees/#296` on `feat/296` - and asserts the
      launch now resolves the right tree and leaves the stale one alone.
- [x] **E4** The repository carries no `CHANGELOG.md`, so nothing to update there. The `409`
      body and `force` are documented on the `run-skill` row of `docs/API_AND_DATA_SPEC.md`.

## Test plan summary

| Level | Where | What it pins |
| --- | --- | --- |
| Unit | `internal/db` | `ActiveRunOnTask` counts `remote_run`, workflow skills and waiting runs, and never `agent_launch` |
| Handler | `internal/handlers/run_skill_guard_test.go` | the `409` / `force` / `403` matrix, and that a refusal records nothing |
| Unit | `internal/agent/agent_config_test.go` | worktree resolution by branch, stale path warning, detached worktrees, key safety |
| Integration | `internal/agent/agent_config_test.go` | the derived branch is PATCHed once, and only when absent |
| UI | `desktop/tests/concurrent-launch.ui.cjs` | the refusal message and the "Launch anyway" gesture |

## Out of scope reminders

Fix 4 (caller attribution on `agent_launch`), `sharesCheckout`, the full-chain runner, orphan-run
reaping, and the manual cleanup of `.tasks/worktrees/#296`, `feat/296` and
`claude/clarify-issue-workflow-bbb85f`.
