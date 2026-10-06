# Tasks #755 - Desktop: archiving a task removes its worktree

Order follows the flow from the workstation outwards; each step leaves the tree
building and its tests green.

## Protocol and model

- [ ] T1 Add `archive_workspace` to `agentprotocol.Operations` and the
      `DeleteBranch` field to `Operation`.
- [ ] T2 Add `models.WorkspaceArchive`, `WorkspaceArchiveEntry` and
      `Archivable()`.

## Agent

- [ ] T3 `internal/agent/archive_workspace.go`: code worktrees (single and
      multi repository), specifications worktree, outcomes removed / absent /
      disabled / failed.
- [ ] T4 Branch clean-up guarded by `DeleteBranch`, existence, checkout,
      upstream and unpushed commits; `git branch -D`.
- [ ] T5 Dispatch `archive_workspace` in `executeOperation`.
- [ ] T6 Tests for T3-T5 on real Git repositories.
- [ ] T7 `POST /desktop/tasks/archive-workspace` relay and the
      `archive-workspace` capability, with a test.

## Server

- [ ] T8 `db.ArchiveTaskWorkspace`: shared branch, `DeleteBranch` from pull
      request states, repositories, caller's agent, empty answer.
- [ ] T9 `POST /api/tasks/{id}/archive-workspace` in the handlers.
- [ ] T10 Tests for T8-T9.

## Desktop

- [ ] T11 `desktop/src/archive-workspace.mjs` (`archiveRefusal`) and its
      `node --test` file.
- [ ] T12 Preload `archiveWorkspace` and the IPC handler with the capability
      check.
- [ ] T13 `main.js`: call before hiding runs for ticket tasks, refusal dialog,
      button disabled while running, tooltips and dialog text.
- [ ] T14 UI tests: fakes answer the route and capability; one refusal case.

## Wrap-up

- [ ] T15 `CHANGELOG.md` line under `## [Unreleased]` / `Changed`.
- [ ] T16 `go build ./...`, `go vet ./...`, `go test` on the touched packages,
      Desktop unit and UI tests; quote the output.

## Test plan (reviewer)

- [ ] Archive a launched ticket with a clean worktree: the folder under
      `.tasks/worktrees/` is gone, the task leaves the sidebar.
- [ ] Add an untracked file in the worktree, archive: "Task not archived"
      names the repository; the task stays; remove the file, archive: done.
- [ ] Stop the agent, archive a ticket: refused with "start or update the
      agent".
- [ ] Archive a ticket whose pull request is merged and branch pushed: the
      local `feat/<n>` is deleted; with an unpushed commit it is kept.
- [ ] Archive a free console: archived as before.
