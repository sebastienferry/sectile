# Retire stale `.taskflow/config.json` documentation

## Why

`.taskflow/config.json` was a retired per-checkout project-context file. The
current documentation still presents it as an active source of truth or as an
agent-written artifact, which gives maintainers a false configuration path and
contradicts the shipped MCP project-context flow.

## What Changes

- Replace the stale current-document references in `AGENTS.md`,
  `docs/ARCHITECTURE.md`, and `docs/contracts/server-agent-v1.md`.
- State that live project and tracker context is supplied through authenticated
  Sectile MCP calls, not a checkout file.
- Preserve documentation for active `.taskflow/agent.json` and diagnostic
  `.taskflow/remote-config.json` behavior.

## Scope

This change is documentation-only. It neither restores nor migrates the
retired file, nor changes the agent/server protocol or any active `.taskflow/`
configuration.

## Impact

Maintainers receive one accurate explanation of project context across the
repository instructions, architecture guide, and server-agent contract.
