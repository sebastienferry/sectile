# Proposal: Set a project's parallelism from the agent configuration

## Problem
Execution parallelism is workstation-owned and defaults to one execution per project
(`agentconfig.ExecutionLimit`). The only surface that stores a value is the "Parallel executions"
slider in the desktop project dialog (`desktop/src/main.js`). A workstation that runs
`sectile-agent` without the desktop app therefore has no way to raise the limit: the value lives in
`~/.config/sectile/settings.json`, which the user must hand-edit, and the server may not carry it.
Issue #268 reports the visible consequence — a second ticket sits in `queued` and never starts.

## Proposed Solution
Add a `config` role to the workstation executable that reads and writes the same settings file the
daemon already obeys:

- `sectile-agent config` prints every project the workstation knows, its repository mapping, its
  worktree preference and its stored parallelism, marking an absent value as the default of one.
- `sectile-agent config --project <id> --parallelism <n>` stores the value, enforcing the same
  1..`MaxParallelism` bounds as the desktop mapping endpoint and refusing an out-of-range value
  rather than clamping it.

Admission re-reads the settings (`localProjectRoot`), so a running agent applies the new limit to
the next submission without a restart.

## In Scope
- `internal/agent/configure.go` and its tests.
- The `config` subcommand in `cmd/agent/main.go`.
- The README and server–agent contract passages that named the desktop app as the only surface.

## Out of Scope
- Changing the default of one execution per project (the open decision on #268).
- Any server-side storage or transport of parallelism: the value stays workstation-owned.
- A parallelism control on the web board.
- The queue itself: `awaitRunSlot` and `sharesCheckout` are unchanged.
