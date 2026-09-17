# Design

## Context
Two deliverables that share one signal: a desktop alert raised by Claude Code hooks, and a waiting
state visible on a Sectile run. The clarification settled the reversible choices (user-level hook
installation, `osascript` as the trigger, silent degradation, English) and recommended one option
per open question. This design applies those recommendations and records how the code makes them
cheaper than the clarification assumed.

## Decisions

### D1 — The hook reports to the local agent loopback, not the reverse
Retained: option (a) of the clarification. The loopback (`internal/agent/agent.go:355-424`) is
already the channel a launched session uses to talk back to Sectile, and it is the only one that
tells "blocked" from "finished" — `ccd_session_mgmt.list_sessions` cannot, and PTY inactivity is a
heuristic that would flag a long build as waiting.

Rejected: polling `ccd_session_mgmt` (undocumented, unversioned, and cannot answer the question);
deriving the state from PTY inactivity (false positives on any slow tool call).

### D2 — The run is identified by the session environment, not by correlating `cwd`
The clarification anticipated a `cwd` → run correlation. It is unnecessary: `internal/agent/agent.go:923-937`
already injects `SECTILE_RUN_ID`, `SECTILE_AGENT_URL`, `SECTILE_AGENT_TOKEN` and
`SECTILE_TASK_WORKTREE` into the launched session, and a Claude Code hook is a child process that
inherits that environment. The hook therefore reads the run identity directly; no directory
matching, no ambiguity when two worktrees share a basename, and a session with no `SECTILE_RUN_ID`
is self-evidently not a Sectile run.

Consequence: `internal/agent/agent_console.go:94-98` sets `SECTILE_RUN_ID` to the empty string for a
free console run, although that run does have an identifier (`run.desktop.ID`). It must be set there
too, otherwise console runs are the one Sectile-launched kind that cannot report waiting.

### D3 — `waitingSince *time.Time` on the activity rather than a new `ActivityStatus`
Retained from the clarification. `ActivityStatus` (`internal/models/models.go:37-44`,
`web/src/types/index.ts:20`) is shared by the server, the persisted activities, `ActivityStats` and
every filter; a new enum value would have to be handled by each of them and by activities written
before the change. A nullable timestamp is additive, needs no backfill, and carries the "waiting for
4 min" the UI wants — which a flat status cannot express.

`waitingSince` is set by the waiting report, cleared by the resumed report, and cleared whenever the
run reaches a terminal status, so a crashed session cannot leave a run waiting forever.

### D4 — Transport: one loopback route, relayed over the existing server REST surface
`POST <loopback>/control/runs/{id}/waiting` with `{"waiting": true|false}`, authenticated with the
loopback token the session already carries (`Authorization: Bearer $SECTILE_AGENT_TOKEN`) and
refused when an `Origin` header is present, exactly like the neighbouring `/control/runs/` handler
(`internal/agent/agent_run.go:97-133`). The daemon relays it to the server as a new sub-action
`POST /api/activities/{id}/waiting`, alongside the existing `retry` and `cancel`
(`internal/handlers/handlers.go:2537-2560`), which delegates to a new `SetRemoteRunWaiting` in
`internal/db/remoterun.go` next to `FinishRemoteRun` and `SyncRemoteRunStatus`, so the waiting state
is written by the same layer that owns the run row and its post-back notification.

Rejected: a new MCP tool. `finish_run` goes through MCP because it is part of the skill contract a
model calls; this report is made by a shell script with no MCP client, and a two-line `curl` is the
whole client.

### D5 — No new push channel to the UI
The waiting report is written through the remote-run layer (`internal/db/remoterun.go`), whose every
mutation ends on `notifyPostBackListeners`; the handler turns that into the existing
`task_updated` SSE event (`internal/db/postback.go:15-30`,
`internal/handlers/handlers.go:82-93`, `/api/events`). The web client already listens to it
(`web/src/context/AppContext.tsx:1237-1265`) and, as a second net, polls `/api/activities` every 3 s
while a run is active (AppContext.tsx:1267-1280) — a waiting run keeps its `running` status, so it
stays inside that fast window. `waitingSince` rides along in the existing payload; no new channel.

### D6 — Precedence and derivation stay in one pure helper
`deriveRunIndicator` (`web/src/lib/remoteRunIndicator.ts`) already reduces a task's runs to a single
state. Waiting is added there as the highest precedence — waiting > running > queued > cancelled —
so cards, list rows, the sidebar and the task detail inherit it without each deciding for itself.
The helper stays pure and directly testable.

### D7 — The alert: `osascript`, sound per event, always exit 0
Retained from the clarification: `osascript -e 'display notification ... with title ... sound name ...'`,
`Funk` for waiting and `Glass` for a finished turn, no Homebrew dependency. The scripts are POSIX
shell, parse the payload's `cwd` without requiring `jq` to be installed, trap every failure, bound
the loopback call with a short timeout, and end on `exit 0`. A non-zero exit or stray stdout from a
hook is read back by the agent, which is precisely the interruption this feature must not cause.

### D8 — Installation merges `~/.claude/settings.json`
Today Sectile only ever *reads* that file, in `checkExternalMCPPolicies`
(`internal/agentconfig/mcp_migration.go:222-255`); becoming a writer of it is genuinely new, and is
the reason the merge rules below are specified rather than assumed.
`ResolveLocations` (`internal/agentconfig/locations.go:44-73`) already establishes that Claude
configuration is user-level: skills go to `~/.claude/skills`, not into each project, because the
per-project layout left four dead copies per project. The hooks follow the same rule. The settings
file is read, decoded, merged on the Sectile-owned entries only, and rewritten; unrelated keys and
third-party hooks survive, a second run is a no-op, and an unparseable file is left untouched with
the failure reported rather than the setup aborted.

Rejected: scaffolding the hooks per project next to the skills — the flaw `locations.go` documents
having fixed.

The scripts themselves are written by the same mechanism as the skills: `Scaffold`
(`internal/agentconfig/local.go:115-208`) resolves the home directory, writes atomically through
`os.OpenRoot(home)` and records what it owns in its manifest; the two hook paths are added to the
managed-file whitelist in `internal/agentconfig/validation.go` so an uninstall removes exactly what
Sectile wrote and nothing else.

## Risks
- The Claude Code hook payload is an external contract with no version we control, and both upstream
  issues closed unresolved. Mitigation: the hook uses only `cwd`, treats its absence as a fallback
  name, and never fails the session.
- A session killed while waiting leaves `waitingSince` set until the run is closed. Mitigation: the
  terminal-status clearing in D3, which every closure path already goes through.
- Multiple hooks may already be registered for `Notification`. Mitigation: the merge in D8 appends
  rather than replaces.

## Open questions
None blocking. The three questions the clarification left open were answered by its own
recommendations and are applied here (D1, and the scope restriction to Sectile-launched runs, D3).
D2 is a refinement of D1 made possible by the existing environment injection, not a reopening.
