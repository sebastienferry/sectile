# Design

## Which pseudo-terminal
`creack/pty` v1.1.24 ships `start_windows.go` returning `ErrUnsupported`, and it is not an
oversight that can be patched locally: ConPTY requires the child to be created with
`PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE` in its `STARTUPINFOEX`, which `os/exec` has no way to
express. That is why creack's two Windows pull requests stalled — any working implementation
has to break `Start(cmd *exec.Cmd)`, so no library can be a drop-in replacement.

`github.com/aymanbagabas/go-pty` accepts that break and builds the right shape around it: the
pty is opened first and the command comes from it (`pty.Command(...)`), so the child can be
created attached. It keeps `creack/pty` as the Unix backend, so the POSIX side of this codebase
is the same code it was running before, reached through a different interface.

The alternative considered and rejected was running the task in the user's own terminal window
(Windows Terminal or `cmd.exe`) through a generated launcher script. It works, but it costs the
console: the output lands in a window the agent does not own, so the desktop and the web board
show a run with no console, and the launcher has to write the agent token to a temporary file
on disk. ConPTY costs one dependency and gives Windows the same console every other platform
has.

## Which shell the session runs
A console session runs an interactive shell and the agent types command lines into it. On POSIX
that is `$SHELL -l`. On Windows there is no such convention, so the session uses what the user
actually works in: `pwsh.exe`, then `powershell.exe`, then `cmd.exe`. `DetectHostLauncher`
already answered that question for the launcher script; the launcher is gone and the answer is
now used to start the session and to quote what is typed into it.

## Enter is a carriage return
This is the part that does not show up in a type signature. A Windows console reads Enter as
`\r`; a bare `\n` is a line continuation. Typing `echo X` followed by `\n` into a PowerShell
session leaves the shell on its `>>` prompt with the command echoed and never run — which looks
exactly like a CLI that started and hung. `SendInput` therefore normalises the line ending for
the host console. Bytes coming from a viewer's keyboard are left alone: a terminal emulator
already sends `\r` for Enter.

## Supervision on Windows
A supervised run is `sectile-agent agent-exec --url <loopback control> --token <t> --command
<cmd>`. That indirection is what lets the board see a start and an exit. What it needs on
Windows is a way to start a child it can later kill as a tree.

A Job Object gives that: the child starts in a new process group, is assigned to the job, and a
forced stop terminates the job, taking every descendant with it. A polite stop sends
`CTRL_BREAK_EVENT` to the group, which is the closest Windows equivalent of the `SIGINT` the
POSIX path sends. `CREATE_NEW_PROCESS_GROUP` is required for the break to be deliverable — and
for it to be *confined*: a child left in the agent's own group takes the whole console down
with it, including whatever started the agent.

## What is left POSIX-only
`RunCommandInSession` wraps a command in `printf` markers and reads `$?` for the exit code. The
markers are POSIX shell and a Windows session would not print them. The helper has no caller
outside its own tests — the dispatch path types its line through `SendInput` and learns the
exit status from `agent-exec` over the loopback API — so it is documented and skipped on
Windows rather than given a second renderer nothing would exercise.
