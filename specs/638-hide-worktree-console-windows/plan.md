# #638 — Plan

Spec: [`spec.md`](spec.md). Checklist: [`tasks.md`](tasks.md).

## Stack and target files

- `internal/agent/provision.go` — `npmInstall`, the `npm ci` run by `provisionWorktree`.
- `internal/sddfiles/sddfiles.go` — `gitOutput`, used by the macro specification reader
  (`internal/agent/agent_macro_dispatch.go`).
- New Windows-only tests: `internal/agent/provision_windows_test.go`,
  `internal/sddfiles/sddfiles_windows_test.go`.
- `AGENTS.md` — the convention every new child process follows.
- Read, not changed: `internal/agentexec/process_windows.go` (`Hidden`),
  `internal/agentexec/process_unix.go` (no-op `Hidden`),
  `internal/agentexec/process_windows_test.go` (test pattern), `internal/agent/agent_config.go`
  (`gitLocal`, already hidden).

## Decisions

- **D1 — Wrap with `agentexec.Hidden`.** Both commands are built as
  `agentexec.Hidden(exec.CommandContext(...))`. `CREATE_NO_WINDOW` is inherited by
  `cmd.exe`, `node` and the install scripts `npm.cmd` starts, so the whole tree stays
  windowless (FR1, FR2). `Hidden` is a no-op outside Windows (FR5).
  - Rejected: a new helper or a change to `Hidden`. The existing one has the right
    semantics and is already tested.
- **D2 — Split construction from execution.** `npmCommand(ctx, dir)` builds the `npm ci`
  command (`Dir`, `WaitDelay`, `Hidden`) and `npmInstall` runs it; likewise
  `gitCommand(ctx, repo, args...)` in `sddfiles` builds what `gitOutput` runs. A test can
  then inspect the command without running npm or git (FR6). `npmInstall` stays a variable
  so the existing provisioning tests keep replacing it (FR3).
- **D3 — `sddfiles` imports `agentexec`.** `sddfiles` is a leaf package so that the agent
  and the server never share filesystem helpers; `agentexec` imports no Sectile package, so
  the dependency keeps that property.
- **D4 — Record the rule in `AGENTS.md`.** A section states that every command the agent or
  the server runs for itself goes through `agentexec.Hidden`, lists the visible exceptions,
  and asks for a Windows test with each fix, so the next launch does not reopen the gap.

## Side effects

None outside Windows. On Windows the two children no longer get a console, which neither
needs: their output is read through pipes.

## Verification

1. On Windows: `go test ./internal/agent -run TestNpmInstallRunsWithoutAConsoleWindow` and
   `go test ./internal/sddfiles -run TestGitOutputRunsWithoutAConsoleWindow`.
2. `go test ./internal/agent -run Provision` and `go test ./internal/sddfiles` (behaviour
   unchanged, FR3).
3. `go build ./...`, `go vet` on the changed packages, `gofmt -l`.
4. Manual, on Windows with the desktop-launched agent: launch a skill on a task without a
   worktree and check no console window opens during worktree creation and `npm ci`.
