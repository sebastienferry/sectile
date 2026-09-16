# Tasks

## 1. Give the console a Windows backend
- [x] 1.1 Move `internal/terminal` from `creack/pty` to `github.com/aymanbagabas/go-pty`:
      open the pty, take the command from it, resize through the `Pty` interface.
- [x] 1.2 Start the session in the host's own interactive shell — `pwsh`, then Windows
      PowerShell, then `cmd.exe` — and keep `$SHELL -l` on POSIX.
- [x] 1.3 Build the environment per platform: `PATH` with the host separator, POSIX locale
      variables only on POSIX.
- [x] 1.4 End a typed line the way the host console expects, leaving a viewer's keystrokes alone.

## 2. Supervise a Windows run
- [x] 2.1 Implement `startControlledCommand` on a Job Object with a new process group.
- [x] 2.2 Implement `stopControlledCommand`: break the group when asked politely, terminate the
      job when forced.
- [x] 2.3 Give a detached child its own process group, so a stop reaches it and nothing else.
- [x] 2.4 Quote the supervised command line for the shell that will read it.

## 3. Remove the external terminal launch
- [x] 3.1 Delete `runner.OpenExternalTerminal` and the launcher script renderers, which had no
      production caller.
- [x] 3.2 Keep the host shell detection and quoting, now used to start the session and to quote
      what is typed into it.
- [x] 3.3 Drop the host-terminal execution branch from the dispatch and the free console, and
      the `hostTerminal` run field and its console notice with it.

## 4. Tests
- [x] 4.1 A line typed into a session really runs, on every platform — the test that catches a
      console reading a bare newline as a continuation.
- [x] 4.2 Host shell detection, and `PATH` joined with the host separator.
- [x] 4.3 Quoting for both hosts, and the encoded multi-line prompt.
- [x] 4.4 A detached child is its own process group on Windows.
- [x] 4.5 Scope the POSIX-marker suites to POSIX hosts, naming the real limitation instead of
      claiming the platform has no pseudo-terminal.

## 5. Gates
- [x] 5.1 `go build ./...` and the Go, web and desktop suites, compared against the pre-change
      failure set on this host.
- [x] 5.2 Cross-compile the agent for darwin and linux to prove the build tags hold.
