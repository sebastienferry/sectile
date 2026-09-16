# Raise the worker ceiling to five and exempt consoles

## Why
A project can run at most three concurrent background executions, which starves
workstations that can comfortably drive more worktrees. Free consoles are counted
as workers, so opening an interactive console silently consumes a background slot
and can queue behind unrelated task executions.

## What Changes
- Raise the configurable per-project parallelism ceiling from 3 to 5, from the
  project setting through the local agent override, the desktop and web pickers,
  and the server-side job limiter.
- Exclude free consoles from the per-project active worker count, and admit a
  console without waiting for a background slot.
- Keep shared-checkout serialization unchanged: a console still waits when another
  execution holds the same checkout.

## Impact
`internal/models`, `internal/db` (limiter and project read), `internal/agentconfig`,
`cmd/agent` (run admission, console, desktop mapping API), `web` project modal,
`desktop` renderer. No database migration and no execution-contract version change.
