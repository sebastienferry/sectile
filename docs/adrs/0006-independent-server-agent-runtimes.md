# ADR 0006: Independent server and agent runtimes

Status: Accepted

## Context

The unified executable and database helpers could execute local Git, tracker
CLIs, terminals and LLMs on the server. Separate entry points alone would leave
those deployment dependencies intact. Tracker synchronization must continue
while every workstation is offline.

## Decision

Build `sectile-server` and `sectile-agent` from independent command packages.
The server owns persisted state, UI/API/MCP services and native HTTP tracker
adapters. GitHub and Linear use explicit server credentials; neither depends on
CLI login state. Jira was preserved as metadata only by this change; the Jira
Cloud REST adapter that reinstated it (ticket #180) follows the same rule, with
no workstation CLI.

The agent owns repositories, worktrees, skill/configuration writes, provider
processes, PR commands and consoles. Electron bundles only this executable.
Workspace requests name capabilities and identities, resolve directories locally,
and require a correlated response from the connection that received them.
Disconnects never cause a server execution fallback.

Background skill jobs dispatch native skills. MCP stage reports and remote run
completion stay separate from launch acknowledgements. Server startup preserves
active remote executions because their processes are owned outside the server. PR checks combine forge
HTTP evidence with local checkout evidence from the agent. The old server-local
result-file worker and terminal endpoints are retired.

## Consequences

Deployments migrate executable names and configure tracker API credentials.
Upgrade both components together; no unified shim is distributed. Browser
startup becomes manual. SQLite and existing project metadata need no migration.
Local mutations can become uncertain after a lost connection; report that state
and require inspection before retrying, rather than risking duplicate actions.

See [the interface contract](../contracts/server-agent-v1.md) and
[deployment instructions](../../README.md#quick-start).
