# Open a discussion on a task instead of running a skill

## Why
Every task action in Sectile runs a workflow skill: the task menu, the command palette and the desktop launch dialog all submit a skill id, and the agent turns it into a `/skill <task>` line. When the user only wants to talk to the agent about a task — understand the code, weigh an approach, ask why a stage failed — there is no way to do it. The project-level free console exists but is desktop-only and carries no task, so it opens in the project root with no branch, no worktree and no task identity.

## What Changes
- Add a reserved skill id `discuss` that opens the configured agent CLI interactively in the task's checkout instead of invoking a skill command.
- Expose it as a task action: a "Discuss" entry in the web task menu and in the task detail skills area, and a "Discussion" option in the desktop launch and relaunch selectors.
- Launch prompt-free: no skill command, no workflow instructions, no stage transition and no skill result. The session inherits the task's worktree, branch and `SECTILE_*` environment, so the agent can read the task through Sectile MCP when the user asks for it.
- Track the session as an ordinary execution so it appears in the runs list and can be stopped and replayed like any other console.

## Impact
- `cmd/agent/agent_config.go` (`dispatchCommand`) and `cmd/agent/agent_desktop.go` (launch admission).
- `web/src/components/TaskCard.tsx`, `web/src/components/TaskDetailModal.tsx`, `desktop/src/main.js`.
- No server change: `POST /api/tasks/{id}/run-skill` already forwards any non-empty skill id to the local agent.
- No database migration, no tracker change, no change to existing skill behaviour.
