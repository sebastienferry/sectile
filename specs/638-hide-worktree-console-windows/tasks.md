# #638 — Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Worktree dependency install (D1, D2, FR1, FR3)

- [ ] T1.1 Extract `npmCommand(ctx, dir)` from `npmInstall` in `internal/agent/provision.go`,
      building the command through `agentexec.Hidden`; `npmInstall` runs it.
- [ ] T1.2 Add `internal/agent/provision_windows_test.go` asserting `CREATE_NO_WINDOW` on
      `npmCommand`'s command, and that its `Dir` and `WaitDelay` are kept.

## 2. Macro specification reader (D1, D2, D3, FR2)

- [ ] T2.1 Extract `gitCommand(ctx, repo, args...)` from `gitOutput` in
      `internal/sddfiles/sddfiles.go`, building the command through `agentexec.Hidden`.
- [ ] T2.2 Add `internal/sddfiles/sddfiles_windows_test.go` asserting `CREATE_NO_WINDOW` on
      `gitCommand`'s command.

## 3. Convention (D4)

- [ ] T3.1 Add the "Child processes never open a console window on Windows" section to
      `AGENTS.md`.

## 4. Verification (FR1–FR6)

- [ ] T4.1 Run the two new Windows tests.
- [ ] T4.2 Run the existing provisioning and `sddfiles` tests.
- [ ] T4.3 `go build ./...`, `go vet` on the changed packages, `gofmt -l` clean.

## Test plan

- Automated: the two Windows tests (FR6) and the existing provisioning and `sddfiles`
  tests (FR3). The full suite runs in CI on the pull request.
- Manual, on Windows with Sectile Desktop: launch a skill on a task without a worktree, then
  check out a task branch from the board, then open a macro's specification files; no
  console window opens in any of the three (US1, US2).
