# Make execution parallelism a workstation-only setting

## Why
Execution parallelism decides how many AI executions a *workstation* runs at once
for a project. It is bounded by the machine's CPU, memory and disk, not by
anything the server knows. Yet the web project settings own the default, the
server stores it in `projects.parallelism`, ships it in the agent configuration
payload, and the desktop app exposes an **Inherit from server** control on top of
it. A user tuning the value in the web UI changes nothing on their own machine
until they also clear a local override, and a value that fits one workstation is
pushed to every other one connected to the same project.

## What Changes
- Remove the parallelism picker from the web project settings, along with the
  field in the project payload and the `Project` type.
- Remove `parallelism` from `models.Project`, from the create/update requests,
  from the `projects` table and from the `agentconfig.Config` payload downloaded
  by the agent. The server-side skill-job limiter runs one worker per project.
- Move the `MaxParallelism` ceiling from `internal/models` to
  `internal/agentconfig`, next to `ExecutionLimit`, which now reads the local
  override alone and falls back to one execution.
- Drop the desktop **Inherit from server** control for parallel executions: the
  picker writes a workstation value directly, and the hint no longer quotes a
  server default.

## Impact
`internal/models`, `internal/db` (schema, project CRUD, job limiter),
`internal/agentconfig`, `internal/taskmcp`, `cmd/agent` (run admission, console,
desktop mapping API), `web` project modal and types, `desktop` renderer and UI
tests. No new contract version: the removed field is simply absent, and an agent
predating the change falls back to a single execution per project. No data
migration: the `projects.parallelism` column is left in place on existing
databases and is no longer read or written.
