# Implementation checklist - #232

Behaviour in [`spec.md`](spec.md), technical choices in [`plan.md`](plan.md). Each block leaves the
tree building and the suite green.

## Block A - Provisioning step

- [x] **A1** Create `internal/agent/provision.go` with `provisionWorktree`, `packageFolders`,
      `installReason`, the stamp writer, the injectable `npmInstall` and the two timeouts.
- [x] **A2** Add a per-directory mutex so two preparations never install in one worktree at once.

## Block B - Wiring

- [x] **B1** Split `prepareDispatch` into `prepareDispatchLocked` (under `prepareMu`, returning
      the root too) and a wrapper that runs `provisionWorktree` after the lock is released.
- [x] **B2** Remove the redundant `prepareMu` lock in the `prepare_workspace` case of
      `internal/agent/agent_operations.go` (existing self-deadlock).
- [x] **B3** Give `prepare_workspace` a 17 minute budget in `operationTimeout`
      (`internal/db/agentoperations.go`), and update `agentoperations_test.go`.

## Block C - Tests

- [x] **C1** `internal/agent/provision_test.go` covering every case listed in `plan.md`.

## Block D - Documentation

- [x] **D1** Correct `docs/REIMPLEMENTATION_GUIDE.md` step 2.
- [x] **D2** Add a `CHANGELOG.md` line under `[Unreleased]`.

## Test plan

- `gofmt -l internal/` prints nothing.
- `go vet ./internal/agent/ ./internal/db/`.
- `go test ./internal/agent/ ./internal/db/`.
- Manual: launch a task on this repository, then run `npm run build --prefix web` and
  `npm run lint --prefix web` in the worktree.
