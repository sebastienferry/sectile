# Windows task execution through the host terminal

## Why
The agent cannot run a single task on Windows. `internal/terminal` starts every execution with
`creack/pty`, whose Windows build returns `ErrUnsupported`, so each dispatch dies at
`impossible de démarrer le terminal PTY: unsupported` before the coding CLI is ever reached.
Two more layers fail behind it: `startControlledCommand` refuses outright with "supervised
native execution is not supported on Windows", so a run could not be supervised even if a
console existed, and `quoteShell` emits POSIX quoting for a command line no Windows shell
parses.

The host-terminal path that would sidestep the PTY already exists — the server dispatches
`open_terminal`, the agent resolves it, and `OpenExternalTerminal` even has a `case "windows"`
— but it is unreachable: the script it writes is `#!/bin/bash` with `export` and
`exec "$SHELL" -l`, written to a `.command` file, and `dispatchTerminal`, the function that
chooses between a PTY and a host terminal, has no callers at all.

## What Changes
- On Windows a task runs in the host terminal (Windows Terminal when present, otherwise
  `cmd.exe`) instead of an embedded PTY. macOS and Linux keep the embedded PTY unchanged.
- The external terminal script is rendered for the shell that will read it: a PowerShell
  script on a Windows host that has PowerShell, a `.cmd` batch file on one that does not, and
  the existing bash script everywhere else. The terminal tab runs the user's own shell.
- `startControlledCommand` / `stopControlledCommand` gain a real Windows implementation built
  on a Job Object, so an externally launched run is still supervised: the board sees it start
  and finish, and cancelling it kills the whole process tree.
- `dispatchTerminal` becomes reachable and decides the execution mode; `detectDefaultTerminal`
  stops claiming `pty` on Windows.
- The command line handed to a host terminal is quoted for that host's shell.

## Impact
`internal/runner` (script rendering and launcher), `internal/terminal` (Windows guard),
`cmd/agent` (routing, quoting, process control) and their tests. No schema change, no protocol
change, no new endpoint. Behaviour on macOS and Linux is unchanged: the same PTY session, the
same console, the same supervision. Restoring an embedded ConPTY console on Windows is
explicitly out of scope — this change makes the platform usable, not identical.
