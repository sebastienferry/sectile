# Make the orchestrator an agent over the backlog

## Why

The backlog orchestrator of ADR 0060 is a Go loop, not an agent. It walks the
tickets in board order, applies length and label filters, and launches a chain
on each ticket until the cap is reached. The user's orchestration policy is
pasted into every ticket run, where each chain step judges its own ticket in
isolation. Nothing compares tickets, nothing explains the order, declining a
ticket costs a ticket session, and nothing takes the next ticket when a chain
ends.

Decision: `docs/adrs/0064-the-orchestrator-is-an-agent-over-the-backlog.md`.

## What Changes

- **BREAKING (behaviour):** `POST /api/projects/{id}/orchestrate` and the MCP
  tool `orchestrate_project` start a project-scoped orchestrator session and arm
  the project, answering 202 with the run ID, instead of returning a synchronous
  launched/skipped report.
- New skill `orchestrate_backlog`, run as a project-scoped autonomous session
  with the selected profile's Markdown file as its policy.
- New MCP tools for that session: `get_orchestration_state`,
  `launch_task_chain`, `defer_task`.
- Hard guards (cap under the project row lock, one agent per ticket, blocker
  labels, stop at `implemented`) move into `launch_task_chain`. The title and
  description length filters are removed.
- Ticket chains no longer carry the eligibility preflight. They keep the
  `Backlog orchestration:` prefix, followed by the orchestrator's rationale.
- Armed state per project: a finished orchestrated chain wakes a new session,
  coalesced over 2 minutes, never more than one queued or running.
  New `POST /api/projects/{id}/orchestrate/stop` disarms it.
- Orchestrator decisions are stored and shown in the desktop overview.
- Desktop: Run agent becomes Start / Stop; the overview shows the orchestrator
  session's console and its decisions.

## Capabilities

### New Capabilities

- `backlog-orchestrator-session`: one agent session per project decides which
  tickets to launch, with the backlog and the previous decisions in view, and
  is woken when a slot frees.

### Modified Capabilities

- `desktop-orchestration-membership` (#871): its "genuine dedicated
  orchestration session" now has a producer, the orchestrator session.

## Impact

- `internal/db/orchestrator.go`, `internal/db/chain.go`, `internal/db/macroruns.go`
  (project-scoped run pattern), a migration for the armed state and the
  decisions table.
- `internal/taskmcp/server.go`, `internal/skills/catalog.go` and the skill
  fragments, `internal/handlers/handlers.go`.
- `internal/agent/agent_desktop_orchestration.go`, `desktop/src/orchestration.mjs`
  and its tests.
- `docs/contracts/server-agent-v1.md` (project-scoped dispatch),
  `docs/ARCHITECTURE.md`, `docs/CAPABILITIES.md`, ADR 0060 status line.
- User-visible: one `Changed` entry under `[Unreleased]` in `CHANGELOG.md`.
