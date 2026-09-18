# Design

## Context
The clarification on #260 left three questions open and named a default for
each; no answer arrived, so this document proceeds on that default and says so
plainly. **These are assumptions, not settled requirements.**

- **A1 — the hook becomes a subcommand of the agent binary** (option C of Q1),
  registered on every platform, rather than a Windows-only companion script or a
  POSIX script prefixed with a discovered `bash.exe`.
- **A2 — verification is GOOS-parameterised Go tests** (option (a) of Q2) over
  the registration and the reporting, with no Windows CI runner added.
- **A3 — the mislabelled session name is in scope** (Q3), because it is the same
  defect and disappears for free under A1.

If any of these is answered differently, the affected decisions below are the
ones to revisit; nothing else in the change depends on them.

Two constraints frame the rest. `internal/agentconfig` is reachable from
`cmd/agent` only (`cmd/server/runtime_boundary_test.go` forbids the workstation
packages on the server side), so the hook may depend on anything the agent
already depends on. And `Scaffold` runs inside the agent process, which is why
`os.Executable()` is a legitimate source for the command it registers — it is
already how `bootstrapLocalMCP` writes the MCP registration.

## Decisions

### The hook is `sectile-agent sectile-hook`, not a script
The script's whole dependency list is the problem: a POSIX shell, `curl`, `sed`,
`tr`, `basename`, and a `$HOME` the shell is expected to set. None of them is
guaranteed on Windows, and the first one is not even reached, because `cmd.exe`
never runs a `.sh` file as a program. A subcommand of the binary Sectile has
already installed needs none of them, starts in a few milliseconds, and puts the
event table in Go where it is unit-testable on any host.

Rejected: **a `sectile-hook.cmd` or `.ps1` companion installed on Windows only.**
It keeps the existing platforms untouched, which is its only merit. It
duplicates the event-to-state table in a second language, to be kept in sync by
hand forever, and under PowerShell `curl` is an alias for `Invoke-WebRequest`,
so the port is not even mechanical.

Rejected: **keeping one POSIX script and prefixing the command with a bash
discovered at install time.** It depends on Git for Windows staying where it was
when Sectile last ran, needs `bash.exe` rather than the detaching
`git-bash.exe`, needs correct `cmd.exe` quoting anyway, and still leaves the
`basename` defect. It trades a certain failure for a fragile success.

### The subcommand is named `sectile-hook`, and that name is the ownership marker
`sectileHookFile` reads ownership from the script's **file name**, never from the
full path, so a workstation whose home directory moved has its registration
updated in place instead of gaining a second, dead entry. A binary has the same
problem and no equivalent name to read: `make build-agent` produces `bin/agent`,
the desktop package ships `desktop/bin/sectile-agent`, and a user may rename
either. Matching on the executable name would misidentify both directions.

So the marker moves to the **argument**: a registration whose command ends in the
token `sectile-hook` is Sectile's, wherever the binary lives and whatever it is
called. The token is distinctive enough that a third-party hook will not collide
with it by accident, and it deliberately mirrors the retired `sectile-hook.sh`,
so the two releases read as the same thing in a settings file.

Rejected: a generic `hook` subcommand. `/opt/mine/hook` is a plausible
third-party command, and the ownership test would then claim it.

### The command is quoted for the shell that will read it
Claude Code passes `command` to the host shell. A path containing a space — and
`C:\Program Files\...` is the normal case on Windows — must therefore be quoted,
with different rules per shell:

- `cmd.exe`: `"<path>" sectile-hook`. A Windows path cannot contain a double
  quote, so no escaping is needed inside.
- POSIX `sh`: the path in single quotes, with an embedded single quote closed,
  escaped and reopened, exactly as `runner.quoteShell` does it.

`claudeHookCommand(executable, goos)` takes the target platform as an argument
rather than reading `runtime.GOOS`, which is what lets one test assert both
forms on one host (A2). It is six lines and stays in `hooks.go`; importing
`internal/runner.QuoteArg` for it would pull the runner into `agentconfig`'s
dependency graph for no other reason.

### `Scaffold` resolves the executable; a temporary one is registered anyway
`Scaffold` already resolves `os.UserHomeDir()` itself, and resolving
`os.Executable()` beside it keeps its signature and every caller unchanged
(`agent_config.go`, `agent_desktop.go`, `agent_operations.go`).

`bootstrapLocalMCP` refuses to register a **temporary** binary — `go run` leaves
a build-cache path that stops resolving when the cache is pruned. The hook
registration does not repeat that guard: it runs before the MCP bootstrap that
already aborts the dispatch, and a hook pointing at a pruned path fails silently
and harmlessly, where a missing MCP registration fails the run. Under `go test`
the executable is the test binary, which is exactly what the registration tests
assert.

### The script is retired, not renamed
`sectile-hook.sh` joins `retiredClaudeHookFiles`. That list is what makes an
existing manifest readable after a rename (`anyManagedPath`), what removes the
installed file on the next refresh, and what drops its registration instead of
leaving a second, dead entry. `hookFiles` then returns nothing for every
provider, `executableHooks` has nothing left to chmod and is deleted, and
`managedHookPath` keeps `claudeHookDir` so the retirement is allowed to touch
`.claude/hooks/sectile-hook.sh`.

`refresh` retires a managed file only when its content still matches the
manifest digest. A user who edited the script keeps their copy and loses the
registration, which is the right outcome: the file is theirs, the registration
is Sectile's.

### Reporting is one function with no process boundary in the test
`agenthook.Report(payload []byte, getenv func(string) string, home string)`
holds the whole decision and performs the single HTTP call. `Run` wires it to
`os.Stdin`, `os.Getenv` and `os.UserHomeDir` and returns nothing: the hook's two
absolute rules are *never write on stdout*, which the agent reads back, and
*always exit 0*, which is the difference between a report and an interrupted
session. `cmd/agent/main.go` therefore dispatches `sectile-hook` without the
`log.Fatal` the other subcommands use.

Testing `Report` directly rather than a spawned process is what makes the suite
runnable on Windows. The exit code and stdout are no longer observable from the
test, so they are guaranteed structurally instead: the function has no return
value and the package writes to neither stream.

### `os.UserHomeDir`, not `$HOME`
The connection file is `~/.taskflow/agent-connection.json`. `os.UserHomeDir`
reads `USERPROFILE` on Windows and `$HOME` elsewhere, so the file resolves
without the shell having set anything. This is the second Windows defect behind
the first, and it costs one line.

### The session name goes through `filepath.Base` and `json.Marshal`
`filepath.Base` understands the backslash on Windows and the slash everywhere,
so `C:\git\sectile` yields `sectile` and `/Users/x/worktrees/#174` yields
`#174`. The name then reaches the wire through `json.Marshal`, which escapes
whatever it contains, so the script's `tr -d` of quotes and backslashes — the
thing that turned `C:\git\sectile` into `C:gitsectile` — has no successor. A
payload with no `cwd`, or one whose base is empty, still announces the session
as `Claude Code`.

### Timeouts and failures stay exactly as the script had them
A two-second client timeout, every transport error swallowed, no retry, no
output. A hook must never make a session wait on the network, and must never
report a failure the user cannot act on.

## Risks
- **The `cmd.exe` contract is inferred, not read from Claude Code's source.** It
  was verified by emulating the call on a Windows 11 host, not by reading the
  client. If Claude Code on Windows executes the command some other way, the
  quoting decision is the part to revisit; the subcommand itself is immune,
  because an executable is a program under every launcher.
- **No Windows CI.** A2 accepts that the platform-specific behaviour is asserted
  by parameterised tests rather than observed. The registration is fully covered
  that way; what remains unobserved is Claude Code's own invocation, which no
  test in this repository could cover anyway.
- **An installation that never refreshes keeps the old registration.** Retiring
  happens on the next `Scaffold`, which every dispatch runs, so the window is one
  task.
