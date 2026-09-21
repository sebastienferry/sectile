# #308 - A concurrent skill launch is not detected, and the worktree is resolved by task key

## Context

On task `#296`, two skill launches started on the local agent while a Claude Code session was
already working the same stage. The first one created a branch and a worktree from nothing and
started an autonomous agent in it; it was stopped by chance, 10.3 seconds in, by a `cancel-run`.
The second one failed with `existing worktree does not use assigned branch
claude/clarify-issue-workflow-bbb85f`, because the worktree it needed existed - under another
path - and was never looked for.

Two independent defects produce this:

1. **The duplicate guard is blind to runs declared outside the local daemon.** The desktop
   "Next: …" button only inspects the daemon's own queue, so a run a Claude Code session
   declared through MCP `start_run` leaves the button active. No server-side barrier exists, and
   the server is the only place that sees every run.
2. **`ensureLocalWorktree` resolves a workspace by task key, not by branch.** The path is always
   `.tasks/worktrees/<key>`; when that path exists on a different branch the launch is refused,
   even though the assigned branch lives, intact, in another linked worktree. The refusal is
   factually true and functionally wrong.

A third, smaller defect feeds the second: when `task.BranchName` is empty, a branch is derived
from the key (`#296` → `feat/296`) and used to create a worktree, but the derived name is never
written back onto the task. The next launch, once a real branch has been assigned elsewhere,
resolves a different branch than the one the first launch created.

The clarification for this ticket is recorded in
[`docs/clarifications/308.md`](../../docs/clarifications/308.md); every product question it
raised was answered by the owner and no question remains open.

This document states behaviour and acceptance criteria only. Architecture, data contracts and
target files are in [`plan.md`](plan.md); the ordered implementation checklist is in
[`tasks.md`](tasks.md).

---

## Scope

**In scope**

- A server-side refusal (`409 Conflict`) of a `run-skill` launch on a task that already carries
  an active run, overridable by an explicit `force`.
- Branch-based worktree resolution through `git worktree list --porcelain`, including a warning
  when `.tasks/worktrees/<key>` exists on an unrelated branch.
- Persistence of the derived `feat/<key>` branch name onto the task at the moment the branch is
  created.
- The desktop rendering of the refusal, and its "launch anyway" gesture.
- The rewrite of the two tests that assert the behaviour being changed.

**Out of scope**

- Caller attribution on the `agent_launch` activity (fix 4 of the ticket) - split into its own
  ticket; it needs a schema column and is observability, not safety.
- `sharesCheckout` and the queue admission logic (`awaitRunSlot`).
- The full-chain runner, and any change to how stages or transitions are decided.
- Reaping or cancelling orphan runs.
- Removing the `#296` residue (`.tasks/worktrees/#296`, `feat/296`, and the orphan branch
  `claude/clarify-issue-workflow-bbb85f`): a manual operation for the owner. Fix 2 makes the
  residue non-blocking, so it carries no urgency.
- The MCP `start_run` path is **not** made to refuse. A CLI session declaring its own run is the
  authority on itself, and refusing it would break the very sessions the guard protects.

---

## Decisions being specified

1. **What counts as an active run.** A task is busy when it carries a `remote_run` activity in
   status `running`, or a workflow-skill activity in status `running`. A run whose
   `waiting_since` is set - waiting for user input - **counts as active**. The `agent_launch`
   activity is not a run and is never counted.
2. **The refusal is a `409 Conflict`**, returned before any activity is recorded, so a rejected
   launch leaves no trace. `run-skill` already reserves `400` for client mistakes, so the status
   code carries the distinction.
3. **`force` is explicit, authorised, and narrow.** An explicit `force` in the payload skips the
   duplicate check **and nothing else**: mode validation, model validation and
   `ensureLocalWorktree` still apply. It is allowed to the active run's owner or to an admin,
   the same rule `cancel-run` enforces through `ErrRunNotYours`. It is never a default.
4. **No staleness threshold, no timer.** A run stuck `running` is cleared by its owner, through
   `force` or through `cancel-run`, not by an expiry the server invents.
5. **The worktree is resolved by branch.** `git worktree list --porcelain` reports every linked
   worktree and the main checkout in one call; the worktree that carries the assigned branch is
   reused wherever it sits. A new worktree is created only when the branch is nowhere.
6. **A stale key path is a warning, not a refusal.** When `.tasks/worktrees/<key>` exists on a
   different branch while the assigned branch lives elsewhere, the resolved worktree is used and
   the stale path is logged as a warning naming it. The current refusal is the bug and goes away.
7. **The derived branch is persisted.** `feat/<key>` keeps being derived when `task.BranchName`
   is empty - launches on a branchless task are not refused - but the derived name is written
   onto the task at the moment the branch is created, so every later launch resolves the same
   branch.
8. **The desktop keeps its local guard and gains an honest failure.** The client guard remains an
   ergonomic nicety; the server one is the barrier. A `409` is rendered as a message naming the
   active run, with a gesture to launch anyway.

---

## User stories

### US1 - A second launch on a busy task is refused (P1)

**As a** developer whose Claude Code session is already running a skill on a task,
**I want** the server to refuse a second launch on that task,
**So that** no autonomous agent starts in parallel on the same work.

- **Given** a task carrying a `remote_run` activity in status `running`
- **When** a `POST /api/tasks/{id}/run-skill` arrives without `force`
- **Then** the response is `409 Conflict`, its body names the skill and the start time of the
  active run, **and** no `agent_launch` activity and no `remote_run` activity are created.

- **Given** a task carrying a run whose `waiting_since` is set
- **When** a launch arrives without `force`
- **Then** it is refused exactly as above: waiting for user input is being active.

- **Given** a task carrying only a completed, failed or canceled run, or only an `agent_launch`
  activity
- **When** a launch arrives
- **Then** it proceeds normally.

- **Given** a task carrying a `running` activity for a workflow skill (`clarify`, `specify`,
  `implement`, `adjust`, `create_pr`, `review`, `pickup`, `pick`, `handoff`)
- **When** a launch arrives without `force`
- **Then** it is refused as above.

---

### US2 - The owner can launch anyway (P1)

**As a** developer whose previous run is stuck because its session died,
**I want** to launch again explicitly,
**So that** a task is never blocked for good by a run nobody will close.

- **Given** a task with an active run owned by the caller
- **When** a launch arrives with `force` set
- **Then** the duplicate check is skipped, the launch proceeds, and the previous run is left
  exactly as it was - not cancelled, not finished.

- **Given** a task with an active run owned by another user, and a caller who is an admin
- **When** a launch arrives with `force` set
- **Then** it proceeds as above.

- **Given** a task with an active run owned by another user, and a caller who is not an admin
- **When** a launch arrives with `force` set
- **Then** the response is `403 Forbidden` and nothing is launched.

- **Given** a launch with `force` set and an invalid mode, an invalid model, or a workspace
  `ensureLocalWorktree` cannot prepare
- **When** the launch is processed
- **Then** it fails on that ground with its usual status: `force` waives the duplicate check
  alone.

---

### US3 - The worktree carrying the branch is found wherever it is (P1)

**As a** developer whose task branch lives in a worktree that does not match the task key,
**I want** the launch to use that worktree,
**So that** the work continues in the tree that actually holds the branch.

- **Given** the assigned branch is checked out in a linked worktree at a path unrelated to
  `.tasks/worktrees/<key>`
- **When** a skill is launched on the task in worktree mode
- **Then** that worktree is used as the working directory, no new worktree is created, and the
  launch succeeds.

- **Given** the assigned branch is checked out in the main checkout
- **When** a skill is launched
- **Then** the main checkout is used, with its uncommitted work untouched, and no
  `.tasks/worktrees/<key>` directory is created.

- **Given** the assigned branch is checked out nowhere
- **When** a skill is launched
- **Then** a worktree is created at `.tasks/worktrees/<key>` on that branch, from the branch if
  it exists locally, from `origin/<branch>` if it exists on the remote, from `HEAD` otherwise.

- **Given** `.tasks/worktrees/<key>` exists and sits on a branch other than the assigned one,
  while the assigned branch is checked out in another worktree
- **When** a skill is launched
- **Then** the resolved worktree is used, the launch succeeds, and a warning is logged naming
  the stale path and the branch it carries. This is the case that previously failed with
  `existing worktree does not use assigned branch`.

- **Given** `.tasks/worktrees/<key>` exists and sits on a branch other than the assigned one,
  while the assigned branch is checked out nowhere
- **When** a skill is launched
- **Then** a worktree for the assigned branch is created at a path that does not collide with
  the occupied one, the launch succeeds, and the stale path is reported as a warning.

- **Given** a task key that is empty, `.`, `..`, or contains a path separator
- **When** a skill is launched in worktree mode
- **Then** the launch is refused, as today: the key must not escape the worktree directory.

---

### US4 - A derived branch name is recorded on the task (P2)

**As a** developer launching the first skill on a task that has no assigned branch,
**I want** the derived branch to be written onto the task,
**So that** every later launch resolves the same branch instead of diverging.

- **Given** a task whose `branchName` is empty or absent
- **When** a skill is launched in worktree mode and the branch `feat/<key-slug>` is resolved
- **Then** the launch proceeds as it does today, **and** `branchName` on the task is set to that
  derived name.

- **Given** a task whose `branchName` is already set
- **When** a skill is launched
- **Then** the task record is not rewritten.

- **Given** the persistence write fails
- **When** the launch is otherwise sound
- **Then** the launch still proceeds and the failure is logged: recording the branch is a
  durability improvement, not a precondition for running.

- **Given** a task whose `branchName` is empty and a project not using worktrees
- **When** a skill is launched
- **Then** nothing is persisted: the branch is whatever the checkout sits on, and it is not the
  task's.

---

### US5 - The desktop tells the truth about a refused launch (P2)

**As a** desktop user pressing "Next: …" on a task another session is already working,
**I want** to see why the launch was refused and be offered an explicit way through,
**So that** a silent failure never reads as nothing having happened.

- **Given** a launch refused with `409` by the server
- **When** the desktop receives the response
- **Then** the toolbar status shows the server's message, naming the run already active, instead
  of a generic launch failure.

- **Given** that refusal is displayed
- **When** the user chooses the offered "Launch anyway" gesture
- **Then** the same launch is re-sent with `force` set, and the outcome - success, `403`, or
  another failure - is rendered in place.

- **Given** the desktop's own local guard already sees an active run in the daemon queue
- **When** the user looks at the toolbar
- **Then** the button stays disabled as it does today: the client guard is unchanged.

---

## Non-functional requirements

- **NFR1 - No trace for a rejected launch.** A `409` refusal creates no `agent_launch` and no
  `remote_run` activity. The activity list of a task must not fill with rejections.
- **NFR2 - One git invocation for resolution.** Resolving the worktree issues a single
  `git worktree list --porcelain`, not one probe per candidate directory.
- **NFR3 - The git helper stays free of database access.** Worktree resolution reads the task it
  is given and returns the resolved branch; writing that branch back belongs to the dispatch
  caller that owns the task record.
- **NFR4 - The existing transition guard is unchanged.** `managedStageRunningUnsafe` and its four
  call sites (`stage.go:35`, `stage.go:131`, `postback.go:104`) keep their current semantics; the
  duplicate check is a separate query.

---

## Acceptance criteria

The ticket is done when all of the following hold.

- **AC1** A launch on a task with an active `remote_run` or workflow-skill run is refused with
  `409`, with no activity recorded, and a run in `waiting_since` triggers that refusal.
- **AC2** `force` from the run's owner or an admin skips the duplicate check and only that check;
  `force` from anyone else is `403`.
- **AC3** A launch resolves the worktree carrying the assigned branch wherever it is - linked
  worktree, main checkout, or nowhere, in which case it is created.
- **AC4** A stale `.tasks/worktrees/<key>` on another branch no longer refuses a launch and is
  reported as a warning.
- **AC5** A derived `feat/<key>` branch is persisted onto the task when the branch is resolved in
  worktree mode and the task carried none.
- **AC6** The desktop renders the `409` message and offers a "Launch anyway" gesture that
  re-sends with `force`.
- **AC7** `internal/agent/agent_config_test.go` no longer asserts the removed refusal, still
  covers the derived-branch path, and the whole Go suite plus the desktop test suite pass.

---

## Open requirements

**None.** The clarification closed every product question; no decision was deferred to
implementation.
