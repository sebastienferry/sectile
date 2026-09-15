# Design

## Why not a Windows PTY
`creack/pty` v1.1.24 ships `start_windows.go` returning `ErrUnsupported`; there is no ConPTY
implementation to enable. Getting the embedded console back on Windows means adopting a second
PTY library and carrying two console implementations. The host terminal is already a supported
execution surface in this codebase — the server dispatches `open_terminal`, the agent resolves
it, and the launcher has a Windows branch — so routing Windows through it costs one renderer and
a process-control implementation rather than a new dependency.

The trade is visible and accepted: on Windows the task output appears in the user's own terminal
window, not in the desktop or web console. The board still tracks the run, because supervision
is preserved.

## Selecting the mode
`dispatchTerminal` already encodes the precedence — explicit `--terminal` flag, then the
dispatch override, then the project's `externalTerminalCommand`, then the agent default — and is
simply never called. Routing calls it and treats `pty` as "embedded console", anything else as
"host terminal". `detectDefaultTerminal` returning `pty` on Windows is the bug that makes the
default unreachable; it returns the host terminal there instead.

## Supervision on Windows
A supervised run is `taskflow-agent agent-exec --url <loopback control> --token <t> --command
<cmd>`. That indirection is what lets the board see a start and an exit, and it works the same
whether the wrapper runs in a PTY or in a host terminal — the wrapper reports over the loopback
API either way. What it needs on Windows is a way to start a child it can later kill as a tree.

A Job Object gives that: the child starts in a new process group, is assigned to the job, and a
forced stop terminates the job, taking every descendant with it. A polite stop sends
`CTRL_BREAK_EVENT` to the group, which is the closest Windows equivalent of the `SIGINT` the
POSIX path sends. `CREATE_NEW_PROCESS_GROUP` is required for the break to be deliverable.

## Script rendering
Rendering is selected by an explicit target parameter rather than by `runtime.GOOS` at the call
site, so the Windows renderer is testable from macOS and Linux — otherwise the only machine that
can test the Windows path is a Windows machine, which is what let this rot in the first place.

The POSIX script deletes itself on its first line, which works because the shell keeps its file
descriptor. `cmd.exe` reads a batch file line by line, so the same trick corrupts execution. The
Windows script deletes itself at the end with `(goto) 2>nul & del "%~f0"`, which closes the batch
context before removing the file. The token is therefore on disk for the duration of the run, in
the user's own temp directory; this is a real difference from POSIX and is why the script is not
left behind.
