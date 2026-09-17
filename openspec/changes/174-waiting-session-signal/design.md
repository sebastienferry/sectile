# Design

## Context
Two deliverables that share one signal: an operating-system notification telling the user which
session is waiting, and a waiting state visible on a Sectile run. The clarification settled the
reversible choices (user-level hook installation, silent degradation, English). A first
implementation raised the alert from the hook itself with `osascript`; that is revised here, and D7
records why.

## Decisions

### D1 — The hook reports to the local agent loopback, not the reverse
The loopback (`internal/agent/agent.go:355-424`) is already the channel a launched session uses to
talk back to Sectile, and it is the only one that tells "blocked" from "finished" —
`ccd_session_mgmt.list_sessions` cannot, and PTY inactivity is a heuristic that would flag a long
build as waiting.

Rejected: polling `ccd_session_mgmt` (undocumented, unversioned, and cannot answer the question);
deriving the state from PTY inactivity (false positives on any slow tool call).

### D2 — The run is identified by the session environment, not by correlating `cwd`
`internal/agent/agent.go` already injects `SECTILE_RUN_ID`, `SECTILE_AGENT_TOKEN` and
`SECTILE_TASK_WORKTREE` into the launched session, and a Claude Code hook is a child process that
inherits that environment. The hook therefore reads the run identity directly; no directory
matching, and no ambiguity when two worktrees share a basename.

Two consequences, both applied:
- `internal/agent/agent_console.go` sets `SECTILE_RUN_ID` to the empty string for a free console
  run, although that run does have an identifier (`run.desktop.ID`). It is set there too, otherwise
  console runs are the one Sectile-launched kind that cannot report waiting.
- The environment carries `SECTILE_AGENT_URL`, but that names the **server**, not the loopback.
  `SECTILE_LOOPBACK_URL` is added; overloading the existing name would break the call sites that
  rely on it meaning the server.

### D3 — `waitingSince *time.Time` on the activity rather than a new `ActivityStatus`
`ActivityStatus` (`internal/models/models.go:37-44`, `web/src/types/index.ts:20`) is shared by the
server, the persisted activities, `ActivityStats` and every filter; a new enum value would have to be
handled by each of them and by activities written before the change. A nullable timestamp is
additive, needs no backfill, and carries the "waiting for 4 min" the UI wants — which a flat status
cannot express.

`waitingSince` is set by the waiting report, cleared by the resumed report, and cleared whenever the
run reaches a terminal status, so a crashed session cannot leave a run waiting forever.

### D4 — Transport: one loopback route, relayed over the existing server REST surface
`POST <loopback>/control/runs/{id}/waiting` with `{"waiting": true|false}`, refused when an `Origin`
header is present and validated against the gateway host, exactly like the neighbouring
`/control/runs/` handler. The daemon relays it to the server as a new sub-action
`POST /api/activities/{id}/waiting`, alongside the existing `retry` and `cancel`, which delegates to
`SetRemoteRunWaiting` in `internal/db/remoterun.go`, so the waiting state is written by the same
layer that owns the run row and its post-back notification.

It authenticates with the workstation API key rather than a per-run token: that token is created by
`wrapRun` for a wrapped dispatched run only, and a free console has a run identifier and no token.
The key is what every launched session is given, console included.

Rejected: a new MCP tool. `finish_run` goes through MCP because it is part of the skill contract a
model calls; this report is made by a shell script with no MCP client, and a two-line `curl` is the
whole client.

### D5 — No new push channel to the UI
The waiting report is written through the remote-run layer, whose every mutation ends on
`notifyPostBackListeners`; the handler turns that into the existing `task_updated` SSE event. The web
client already listens to it and, as a second net, polls `/api/activities` every 3 s while a run is
active — a waiting run keeps its `running` status, so it stays inside that fast window. The desktop
polls `/desktop/runs` every 2 s and learns the same way. `waitingSince` rides along in both existing
payloads; no new channel.

### D6 — Precedence and derivation stay in one pure helper
`deriveRunIndicator` (`web/src/lib/remoteRunIndicator.ts`) already reduces a task's runs to a single
state. Waiting is added there as the highest precedence — waiting > running > queued > cancelled —
so cards, list rows, the sidebar and the task detail inherit it without each deciding for itself.
The helper stays pure and directly testable.

### D7 — The desktop raises the notification, through the OS notification facility
**Revised.** The first implementation called `osascript -e 'display notification …'` from the hook.
It works, and it is the wrong banner: macOS attributes it to **Script Editor**, not to Sectile, and
`display notification` accepts no icon at all. The user cannot tell which application is talking,
and the banner cannot carry the glyph that marks the same state in the task list — which is half of
what this change is for. It is also macOS-only, which the proposal's own degradation rule forbids.

The notification is therefore raised by the desktop application, which is Electron. Electron's
notification API is a thin binding over each platform's own facility — `UNUserNotificationCenter` on
macOS, toast notifications on Windows, the freedesktop notification spec on Linux — so the banner is
a real system notification, attributed to the packaged application, carrying its identity and an
icon we choose. One code path covers the three platforms, and no `terminal-notifier`, no Homebrew,
no external binary is involved.

The desktop already polls `/desktop/runs` every two seconds and already holds the run list it
renders. The waiting state is added to that payload, and the transition — not the state — is what
notifies: a run that was not waiting and now is raises the waiting banner; a run that was running
and has reached a terminal status raises the finished banner. Reading transitions off the polled
list means no new channel and no duplicate banner when a poll repeats.

Rejected: `terminal-notifier` and `notify-send` from the hook. They do carry an icon, but they are
external binaries the workstation may not have, one per platform, and they would put the icon
vocabulary in a second place where it would drift from the UI's.

Rejected: raising the banner from the web UI through the browser Notification API. It requires a
permission prompt per browser, the tab has to be open, and a background tab is throttled — none of
which is true of the desktop application.

### D8 — One state vocabulary, shared by the badge and the banner
The correlation the user asks for is not a coincidence to maintain but an invariant to enforce: the
glyph on the banner and the glyph on the task row are the same glyph because they are read from the
same table. A single module names each run state, its glyph and its colour; the web badge renders it
as an icon component, and the desktop renders it into the notification's icon. Adding a state means
adding one row, and nothing can show a state the other surface does not know.

### D9 — A session Sectile did not launch is still announced
The ticket's own objective names "several Claude Code desktop tabs", not only Sectile executions, so
moving the banner to the desktop must not quietly drop them. A hook with no `SECTILE_RUN_ID` reads
`~/.taskflow/agent-connection.json` — the handshake file the agent already publishes atomically with
the loopback URL and the desktop credential — and posts a session-level report naming the session by
the basename of its `cwd`. The daemon holds those reports in a small bounded list, the desktop drains
them on the same poll, and the banner is raised identically.

Such a session has no run, so nothing is written to the server and no row changes: it is a banner and
nothing more. A workstation that has never been paired has no connection file, and the hook is then a
no-op, which is correct — there is no Sectile to notify.

### D10 — Installation merges `~/.claude/settings.json`
Today Sectile only ever *reads* that file, in `checkExternalMCPPolicies`
(`internal/agentconfig/mcp_migration.go:222-255`); becoming a writer of it is genuinely new, and is
the reason the merge rules below are specified rather than assumed. `ResolveLocations`
(`internal/agentconfig/locations.go:44-73`) already establishes that Claude configuration is
user-level: skills go to `~/.claude/skills`, not into each project, because the per-project layout
left four dead copies per project. The hooks follow the same rule. The settings file is read,
decoded, merged on the Sectile-owned entries only, and rewritten; unrelated keys and third-party
hooks survive, a second run is a no-op, and an unparseable file is left untouched with the failure
reported rather than the setup aborted.

Rejected: scaffolding the hooks per project next to the skills — the flaw `locations.go` documents
having fixed.

The scripts themselves are written by the same mechanism as the skills: `Scaffold`
(`internal/agentconfig/local.go:115-208`) resolves the home directory, writes atomically through
`os.OpenRoot(home)` and records what it owns in its manifest; the two hook paths are added to the
managed-file whitelist in `internal/agentconfig/validation.go` so an uninstall removes exactly what
Sectile wrote and nothing else. They are chmod 0700 after the write: `Scaffold` writes 0600, which is
right for a skill and useless for a script Claude Code has to execute.

## Risks
- The Claude Code hook payload is an external contract with no version we control, and both upstream
  issues closed unresolved. Mitigation: the hook uses only `cwd`, treats its absence as a fallback
  name, and never fails the session.
- The banner now depends on the desktop application running. Mitigation: this is a deliberate
  trade-off for a real, attributable, cross-platform notification; the waiting state is recorded and
  rendered whether or not the desktop is open, so the list still answers the question.
- The operating system may deny notifications to the application. Mitigation: the desktop checks
  once and degrades to the list alone, without prompting on every event.
- A session killed while waiting leaves `waitingSince` set until the run is closed. Mitigation: the
  terminal-status clearing in D3, which every closure path already goes through.
- Multiple hooks may already be registered for `Notification`. Mitigation: the merge in D10 appends
  rather than replaces.

## Open questions
None blocking.
