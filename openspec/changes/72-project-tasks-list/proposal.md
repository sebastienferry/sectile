# Order desktop project tasks by execution state and start time

## Why
Desktop task rows inherit unordered agent map results, so refreshes can shuffle tasks and bury active work. Ticket #72 confirms active, queued, then finished groups, newest first within each group, using actual execution start time.

## What Changes
- Select each task's representative execution by state priority and effective execution time.
- Order task rows by the representative, with deterministic identity ties.
- Expose actual start time in the desktop agent response, retaining submission-time fallback for queued, preparing, and older runs.

## Scope
Desktop project sidebar and its local agent run contract only. Preserve alphabetic projects, one row per task, archive filtering, history ordering, and selected execution. No scheduler, tracker, web UI, or sidebar layout changes.

## Impact
`cmd/server/agent.go`, `cmd/server/agent_desktop.go`, desktop rendering and tests, and desktop/contract documentation. No database migration or unresolved product decision. Settled clarification: `docs/clarifications/72.md`.
