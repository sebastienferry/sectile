# ADR 0012: The waiting state is a timestamp on the run, reported through the loopback

Status: Accepted

## Context

Several agent sessions run in parallel across worktrees and desktop tabs. When
one blocks on a permission prompt or a question, nothing says so: the human has
to open each session in turn to find the one that stopped. The bottleneck is not
the agent, it is the human noticing.

The upstream requests for a native signal (anthropics/claude-code#36885 and
\#36850) were both closed without a solution. The desktop app's
`ccd_session_mgmt.list_sessions` reports `isRunning: false` for a finished turn
and for a blocked one alike, so it cannot answer the question either, and PTY
inactivity would flag a long build as waiting.

What does answer it is the `Notification` hook, which fires exactly when the
agent asks for input, and the `Stop` hook for the symmetric case.

## Decision

**The transport is the agent loopback, not a new MCP tool.** A launched session
is already given `SECTILE_RUN_ID`, `SECTILE_LOOPBACK_URL` and
`SECTILE_AGENT_TOKEN`, and a hook is a child process that inherits them. It
therefore knows which run it belongs to without any `cwd` correlation, which
would have been ambiguous the moment two worktrees shared a basename. The
loopback route relays the report to `POST /api/activities/{id}/waiting`.

`finish_run` goes through MCP because it is part of the contract a model calls;
this report is made by a shell script with no MCP client, and a two-line `curl`
is the whole client.

The route authenticates with the workstation API key rather than the per-run
token of `/control/runs/{id}`: that token exists only for a wrapped dispatched
run, and a free console has a run identifier but no such token. The key is what
every launched session is given, console included.

**The state is `waitingSince *time.Time`, not a new `ActivityStatus`.**
`ActivityStatus` is shared by the server, the persisted activities,
`ActivityStats` and every filter; a new enum value would have to be handled by
each of them, and by every activity written before the change. A nullable
timestamp is additive, needs no backfill, and carries the "waiting for 4 min"
the UI wants, which a flat status cannot express. A waiting run keeps the status
`running`, so it stays inside the fast polling window the client already applies
to live runs.

`waitingSince` is set by the waiting report, cleared by the resumed report, and
cleared by every path that gives a run a terminal status, so a session killed
while blocked cannot leave a run waiting forever.

**The hooks are installed once per workstation.** `ResolveLocations` already
established that Claude configuration is user-level, because the per-project
layout it replaced left four dead copies per project. The hooks follow that
rule, which makes Sectile a writer of `~/.claude/settings.json` — a file it
previously only read, in `checkExternalMCPPolicies`. The file is therefore
merged, never replaced: only the Sectile-owned entries are added or updated, and
a file that cannot be parsed is left alone with the failure reported rather than
the setup aborted.

## Consequences

- A run has two live presentations, running and waiting, derived in one place
  (`deriveRunIndicator`) so every surface inherits the same precedence.
- Sectile owns two more managed files, and they are in the whitelist, so an
  uninstall removes exactly what it wrote.
- The desktop alert is macOS-only, by way of `osascript`. Nothing else in the
  feature is: the state is stored and rendered identically on any platform, and
  the alert degrades to a silent no-op where the command is missing.
- The Claude Code hook payload is an external contract with no version we
  control. The hooks read only `cwd`, treat its absence as a fallback name, and
  never fail the session.
- A free console now exports its own `SECTILE_RUN_ID`, where it previously
  exported an empty value. It is a local run the server does not record, so its
  report is dropped at the relay; the desktop alert still fires.

## Alternatives rejected

- **Polling `ccd_session_mgmt`**: undocumented, unversioned, and it cannot tell
  a finished turn from a blocked one, which is the whole question.
- **Deriving the state from PTY inactivity**: a false positive on every slow
  tool call.
- **A new `ActivityStatus` value**: see above; and it could not carry a duration.
- **Scaffolding the hooks per project**: the flaw `locations.go` documents having
  fixed.
- **The `PreToolUse` workaround** circulated in the upstream issue: it fires on
  every single tool call.
