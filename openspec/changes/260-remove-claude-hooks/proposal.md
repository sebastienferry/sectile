# Remove the Claude Code hook

## Why
Since #174, setting up the Claude provider installed a script under
`~/.claude/hooks`, registered it in `~/.claude/settings.json` on five Claude Code
events, and had it report to the local agent whether the session was waiting for
the user or working. #260 was opened because that hook never ran on Windows:
the registered command was the bare path of a `.sh` file, which `cmd.exe`
resolves through its file association — a detached Git Bash window where Git for
Windows is installed, "not recognized as an internal or external command" where
it is not.

Porting the hook into the agent binary would have fixed the platform defect and
left the two objections that motivate this change untouched:

- **It is intrusive.** Sectile writes a user file it otherwise only reads, and
  runs a process on every tool call of every Claude Code session on the
  workstation — launched by Sectile or not, because the registration is
  user-level. A workstation used for anything else pays for the hook on every
  turn.
- **It is not needed.** The desktop already raises a banner when a run reaches
  a terminal status, from the run list it polls, and the agent observes that
  exit itself. What the hook alone provided — the waiting mark on a run, and a
  banner for sessions Sectile did not launch — was not judged worth that cost.

So the hook is removed rather than fixed. The waiting state it fed is kept as an
idle model, because it is additive and a future reporter that is not a
per-tool-call hook can feed it without touching the interface.

## What Changes
- **Nothing is installed under `~/.claude/hooks` and nothing is registered in
  `~/.claude/settings.json` any more.** The embedded script, its installer and
  the registration merge are deleted.
- **Every earlier installation is retired.** The three script names Sectile
  ever installed (`sectile-notification.sh`, `sectile-stop.sh`,
  `sectile-hook.sh`) stay recognised so the managed-file manifest retires the
  files, and the settings cleanup drops only the registrations Sectile wrote —
  by script name, or by the trailing `sectile-hook` argument a pre-release build
  of #260 registered. Third-party hooks and every other key survive, a file
  Sectile never touched is not rewritten, an unparseable file is left alone and
  reported. The cleanup runs for every provider, not only `claude`: a
  workstation that moved to another agent still uses Claude Code by hand.
- **The agent loses the receivers that existed only for the hook**:
  `POST /control/runs/{id}/waiting` on the loopback and its relay to the
  server, `/desktop/session-alert` and the alert backlog, and the
  `waitingSince` mark on the agent's own run list. The desktop stops polling
  session alerts.
- **The waiting model is kept unchanged**: `waitingSince` on the persisted run,
  `POST /api/activities/{id}/waiting`, the web indicator and filter, the desktop
  row and banner for a waiting transition, `shared/runStates.ts`. Nothing calls
  the route today.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `agent-provider-setup`: setting up a provider installs no Claude Code hook,
  and retires the hooks earlier releases installed.

## Out of scope
- Removing the waiting model (column, route, indicator, desktop rendering).
  It is idle, additive, and a change of its own if it is ever dropped.
- Any other feed for the waiting state.
- The desktop banner on a terminal run status, which does not depend on the
  hook and is unchanged.

## Impact
- `internal/agentconfig/hooks.go`: the retirement alone remains —
  `retireClaudeHooks`, the retired names and the ownership test.
  `internal/agentconfig/hooks/hook.sh`, `hookscripts_test.go` and
  `hookrecorder_test.go` are deleted; `hooks_test.go` covers the retirement.
- `internal/agentconfig/local.go`: `Scaffold` installs no hook file and calls
  the cleanup for every provider, then removes the emptied `.claude/hooks`.
- `internal/agent/agent_run.go`, `agent_desktop.go`: the loopback waiting
  receiver, its relay, the session-alert endpoint and backlog, and
  `desktopRun.WaitingSince` are removed; `waiting_test.go` with them.
- `desktop/electron/main.cjs`, `preload.cjs`, `desktop/src/main.js`: the
  `session-alerts` IPC and its poll are removed; the UI test no longer stubs it.
- `docs/adrs/0012-waiting-for-input-as-a-timestamp.md`: revision recording the
  withdrawal. `docs/CAPABILITIES.md` §4bis and `.agents/MEMORY.md` §6 rewritten.
