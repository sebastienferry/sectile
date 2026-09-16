# Split server and agent binaries

## Why
The current executable embeds the web server, local agent, SQLite access, MCP bridge and subprocess execution. The server also calls Git and tracker CLIs through database helpers, so separating build outputs alone would not make it deployable as a headless control plane.

## What Changes
- Build `sectile-server` from `cmd/server` and `sectile-agent` from `cmd/agent`.
- Keep SQLite, board/roadmap/sprint configuration, tracker synchronization, REST, the agent relay and upstream HTTP MCP on the server.
- Move workspace, Git, pull-request, terminal, editor and LLM execution to the local agent, with explicit unavailability when it is disconnected.
- Replace server-side tracker subprocess adapters with native HTTP requests while preserving existing tracker operations.
- Bundle only the agent in Electron and update build, release, configuration and migration documentation.
- Document and test the Server API, Agent Loopback and MCP contracts.

## Impact
Breaking executable packaging change. Existing unified binary invocations must migrate. Server deployments need explicit tracker credentials and repository identities; workstation CLI login state and repository paths are no longer server dependencies. Existing product branding and MCP tool names remain as defined on main.

Out of scope: multi-tenant authentication, provider approval UI, server-hosted LLMs, unified compatibility executables, automatic merge or deletion of task worktrees.
