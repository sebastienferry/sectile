# Hooks that run on Windows

## Why
`registerClaudeHooks` writes the hook command as the bare path of the installed
script, `<home>\.claude\hooks\sectile-hook.sh`. Claude Code hands that string to
the host shell, and on Windows that shell is `cmd.exe`, which resolves a `.sh`
file through its file association. On a workstation with Git for Windows that
association is `"C:\Program Files\Git\git-bash.exe" --no-cd "%L" %*`: a detached
MinTTY window. Probed on a Windows 11 host, `cmd /c "<script>"` returned 0
immediately, the script body never ran, and stdin was never connected. On a
workstation with no `.sh` association, `cmd.exe` answers "not recognized as an
internal or external command" instead, which Claude Code surfaces as a hook
error on every tool call.

So on Windows the hook is not partly broken: it has never run. A session there
never reports waiting or working to the local agent, an unpaired session never
raises its banner, and the five registered events each try to pop a window.

The script is also unreachable by the test suite on that platform:
`hookscripts_test.go` executes it through `exec.Command("/bin/sh", script)`, so
the whole hook suite is unrunnable on Windows and CI, which builds in Linux
containers, cannot observe the defect.

Two smaller defects sit behind the first one and only become visible once the
script runs. `basename "$cwd"` on the Windows payload path `C:\git\sectile`
returns the whole string, and the `tr -d '"\'` that protects the JSON then
collapses it to `C:gitsectile`, so the banner names the wrong session. And
`$HOME` is not a variable a Windows shell is required to set, so the connection
file `~/.taskflow/agent-connection.json` may not resolve at all.

## What Changes
- The hook stops being a shell script and becomes **a subcommand of the agent
  binary**, `sectile-agent sectile-hook`, registered on every platform. It reads
  the payload from stdin, decodes it as JSON, and makes the same single report
  the script made: `POST /control/runs/<id>/waiting` for a session Sectile
  launched, `POST /desktop/session-alert` for any other one.
- `internal/agentconfig/hooks/hook.sh` and its embed are deleted.
  `sectile-hook.sh` joins `retiredClaudeHookFiles`, so an existing installation
  has the file removed and its registration replaced rather than duplicated.
- The registered command is quoted for the shell that will read it: single
  quotes on POSIX, double quotes for `cmd.exe`. The executable comes from
  `os.Executable()`, resolved by `Scaffold`, which already runs inside the agent.
- Ownership of a registration is read from the trailing `sectile-hook` argument
  instead of the script file name, with the three retired script names still
  recognised so their entries are dropped. An agent binary that moved is
  updated in place, exactly as a moved home directory was.
- The session name comes from `filepath.Base` of the payload's `cwd`, which
  understands both separators, and reaches the wire through `json.Marshal`
  rather than through a hand-rolled quote strip.
- The hook behaviour is tested in-process on any host, and the registration is
  tested per target platform by passing `GOOS` to the command builder instead of
  reading `runtime.GOOS`.

## Capabilities

### New Capabilities
- `cross-platform-session-hooks`: the Claude Code hook Sectile registers runs on
  every platform Sectile supports, reports the session state to the local agent,
  and never interrupts the session it reports on.

### Modified Capabilities
None: no `openspec/specs/` entry covers the hooks yet. The change
`174-waiting-session-signal` introduced them and is not archived.

## Out of scope
- The hook state machine: which events open and close a wait, which
  notification types count as a prompt, and the rule that an autonomous run
  never waits. Settled by #174 and ADR 0012, carried over unchanged.
- The desktop banner itself, its glyphs and its polling.
- Providers other than `claude`: `hookFiles` returning nothing for them stays a
  valid installation.
- A Windows CI runner. The pipeline builds in Linux containers and
  cross-compiles the Windows binaries without executing them; adding a runner is
  an infrastructure change this repository does not own.

## Impact
- `internal/agenthook/` (new): `hook.go` and `hook_test.go`.
- `cmd/agent/main.go`: the `sectile-hook` dispatch.
- `internal/agentconfig/hooks.go`: no installed file, the command it builds and its
  quoting, ownership by argument, `executableHooks` removed.
- `internal/agentconfig/hooks/hook.sh`: deleted, with
  `internal/agentconfig/hookscripts_test.go`.
- `internal/agentconfig/local.go`: `Scaffold` resolves the executable and stops
  calling `executableHooks`.
- `internal/agentconfig/hooks_test.go`: registration assertions follow the new
  command.
- `.agents/MEMORY.md` §6 and `docs/adrs/0012-waiting-for-input-as-a-timestamp.md`:
  the hook is a binary subcommand now.
