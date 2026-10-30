# ADR 0001: A local desktop supervisor over existing Go services

- Date: 2026-10-28
- Status: Accepted for the bootstrap; credential migration and provider handoff require further design

## Context

The product is an open-source, self-hosted desktop application. Users want local
GitHub, GitLab and Jira sign-in, remote trackers as the board's source of truth,
and orchestration without recreating every provider's chat interface. Existing
Sectile Go services already implement tracker synchronization, agent execution,
worktrees and MCP. The original desktop uses an authenticated loopback API.

## Decision

Build a separate Tauri 2 / React / TypeScript application under MIT. Retain Go
services as externally running dependencies for the first milestone. Do not fork
or embed them during bootstrap. Rust reads the Go agent discovery file and exposes
only project and task read operations to the local webview. Reject non-loopback
origins and redirects. No provider tokens enter frontend state.

The initial board includes only tasks explicitly sourced from GitHub, GitLab or
Jira, grouped by `trackerStatus`. An unavailable remote status is shown explicitly;
internal SDD workflow stages must not be presented as remote tracker statuses.
There are no local task mutations. Server data is a synchronized projection, not
a claim of a live remote read. Authentication remains in the existing services
until the local credential architecture is implemented.

## Supervisor boundary

The intended supervisor owns scheduling, worktrees, run identity, progress and
human checkpoints. Provider adapters own launch, events, interruption, resumption
and capability discovery. Preserve provider session IDs and represent unsupported
operations explicitly. UI handoff is optional and must use a documented mechanism.

Documented integration candidates checked on 2026-10-28:

- [Codex App Server](https://learn.chatgpt.com/docs/app-server): thread/turn lifecycle, streamed events and approvals.
- [Claude Code CLI / Agent SDK](https://code.claude.com/docs/en/headless): programmatic runs and structured output.
- [Antigravity CLI](https://antigravity.google/docs/cli/headless/): prompts, streamed events and conversation resumption.

These interfaces do not establish that arbitrary sessions can be controlled in
an already-open provider desktop application. Verify that separately for each
provider. MCP provides tools for an agent to call into Sectile; it does not itself
initiate a conversation in an MCP client. Reuse the existing Go execution adapters
before introducing duplicate Rust implementations.

## Consequences and next milestones

1. Bootstrap delivers a native shell and actual agent-backed reads, without execution controls.
2. Add a local credential vault and browser OAuth callbacks for each tracker. Move credential ownership off the server deliberately; no silent fallback to shared identity.
3. Add execution monitoring and explicit launch/stop actions through existing Go routes, with approval handling and lifecycle tests.
4. Package and supervise Go sidecars with health checks, compatibility checks, graceful shutdown and Windows console suppression.
5. Add provider-app handoff only where documented and tested. Otherwise offer the provider CLI or supported client.

The initial dependency on a separate Go server is transitional. Fully desktop-owned
hosting and direct local tracker sign-in are goals, not features delivered by this bootstrap.
