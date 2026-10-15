# Desktop engine setup assistant

## Why
Desktop currently exposes independent deployment, MCP and permission controls without guiding users through a complete setup or indicating what remains incomplete. A guided assistant makes configuration outcomes understandable and recoverable.

## What Changes
- Add an on-demand assistant in Settings > General for catalogue engines using Claude, Codex or Antigravity.
- Offer optional standalone skills (as an alternative to plugins), required MCP configuration and explicit permission review, and optional project SDD installation.
- Notify users once per launch with a dismissible in-window action when readable configuration indicates incomplete setup; never automatically open the assistant.
- Preserve partial successes, unrelated configuration, plugin installations and existing permission choices. Confirm configuration writes and explain engine restart requirements.

## Scope
Provider configuration is shared by engines using the same provider. SDD applies only to the explicitly selected project. Skills source selection is explicit. Custom providers, CLI installation, provider login, automatic plugin installation, live AI verification and permission escalation are excluded.

## Impact
Desktop renderer, Electron IPC, local agent setup-state persistence/readback and existing deployment/MCP/permissions integrations. No server database migration or new service is required. Implementation adds one user-facing Added entry under CHANGELOG.md Unreleased. This specification does not change runtime code.

## Accepted Baseline
Issue #842, clarification rounds 2–4 in docs/clarifications/842.md. Round 4 confirms all product choices; no open product questions remain.
