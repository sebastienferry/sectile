# Windows task execution through an embedded pseudo-console

## Why
The agent cannot run a single task on Windows. `internal/terminal` starts every execution with
`creack/pty`, whose Windows build returns `ErrUnsupported`, so each dispatch dies at
`impossible de démarrer le terminal PTY: unsupported` before the coding CLI is ever reached.
Two more layers fail behind it: `startControlledCommand` refuses outright with "supervised
native execution is not supported on Windows", so a run could not be supervised even if a
console existed, and `quoteShell` emits POSIX quoting for a command line no Windows shell
parses.

Windows has had a pseudo-terminal since Windows 10 1809 — the Pseudo Console API, ConPTY. What
is missing is not the platform capability but a Go binding: `creack/pty` never implemented one
(issue #95, open since 2020, two stalled pull requests), because ConPTY requires the child to be
created already attached to the pseudo-console through `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`,
which `os/exec` cannot express. `github.com/aymanbagabas/go-pty` solves exactly that: one
cross-platform `Pty` interface, `creack/pty` underneath on Unix, ConPTY on Windows, and a
`Cmd` type that starts the child attached.

## What Changes
- `internal/terminal` moves from `creack/pty` to `github.com/aymanbagabas/go-pty`. A console
  session is now created the same way on every platform: open the pty, then ask it for the
  command. Windows gets the embedded console the other platforms already had.
- A Windows session runs the user's own shell — `pwsh`, then Windows PowerShell, and `cmd.exe`
  only when neither exists — rather than a shell chosen for them.
- A line typed into a session ends the way the host console expects. A Windows console reads
  Enter as a carriage return: with a bare newline the shell sits on its continuation prompt and
  the command is echoed but never runs.
- The session environment stops assuming POSIX: `PATH` is joined with the host separator, and
  the POSIX locale variables are set only where they mean something.
- `startControlledCommand` / `stopControlledCommand` gain a real Windows implementation built on
  a Job Object, so a run is supervised there as it is on POSIX: the board sees it start and
  finish, and cancelling it kills the whole process tree.
- The supervised command line is quoted for the shell that will read it, which on Windows is
  PowerShell rather than a POSIX shell.
- The dead external-terminal launcher is removed: `runner.OpenExternalTerminal` and its script
  renderers had no production caller — `/api/open-terminal` dispatches an `open_terminal`
  action that the agent runs in the console session like any other command.

## Impact
`internal/terminal` (pty backend, shell and environment per platform, line endings),
`internal/runner` (host shell detection and quoting kept, launcher removed), `cmd/agent`
(process control, one execution surface again) and their tests. One new direct dependency,
`github.com/aymanbagabas/go-pty`; `creack/pty` stays in the module as its Unix backend. No
schema change, no protocol change, no new endpoint. Behaviour on macOS and Linux is unchanged:
the same session, the same console, the same supervision.
