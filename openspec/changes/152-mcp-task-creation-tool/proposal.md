# The Sectile MCP catalog gains a task creation tool

## Why
The MCP catalog exposes eight tools and none of them creates a task, while the
skills Sectile itself generates require exactly that. `internal/db/skilltemplates.go:245`
tells a session to "turn any remaining follow-up into a separate ticket to create",
and line 518 tells it to use "the local Sectile agent's exposed task-management
interface first for ticket reads, updates, comments, creation". The second
instruction cannot be honoured, so the first one is only reachable by abandoning
the MCP interface: creating #162 and #163 during the #142 handoff required falling
back to `curl http://localhost:8090`, which forces a session to know the port, the
`models.CreateTaskRequest` shape and the meaning of `requireRemoteCreation` — three
things a tool encapsulates.

## What Changes
Add `create_task` to the MCP catalog, delegating to the existing `db.CreateTask`
so tracker-backed creation, key allocation, workflow labelling and external URL
resolution stay on one path with the HTTP adapter.

The tool requires an explicit `projectId`: a bare key such as `#47` can name
another project's ticket, and inferring the project from ambient context is how an
agent files a ticket on the wrong board. It forces `requireRemoteCreation`, so a
tracker that cannot create remotely fails loudly instead of leaving a local-only
ticket that no reviewer will ever see. It does not accept a status: a created task
lands where `task-creation-label` says it lands.

The stdio proxy in `cmd/agent/mcp.go` whitelists the eight names and refuses any
other catalog size; it learns the ninth tool. The naming specification, the shared
naming contract test and the three documents that enumerate the catalog move from
eight tools to nine.

## Capabilities
### Added Capabilities
- `mcp-task-creation`: creating a Sectile task through the MCP interface.

### Modified Capabilities
- `sectile-mcp-naming`: the canonical catalog becomes nine tools.

## Impact
`internal/taskmcp/server.go`, `cmd/agent/mcp.go`, `internal/mcptest/contract.go`,
`README.md`, `docs/ARCHITECTURE.md`, `docs/contracts/server-agent-v1.md`. No schema
change, no migration, no change to `POST /api/tasks` or to `db.CreateTask`. An
agent older than its server keeps refusing the catalog, which is the existing
coordinated-upgrade behaviour and not a new failure mode.
