# ADR 0023: Explicit workstation MCP connection choices

Status: Accepted

## Context

Users need to understand HTTP and STDIO per AI engine and configure their local
provider from the desktop. Some prefer a direct remote connection that survives
agent shutdown; others explicitly want a local proxy without duplicating their
pairing credential into provider configuration files.

## Decision

Store an explicit transport and target per provider in workstation settings.
Unselected providers keep their existing bootstrap defaults. Selected providers
retain their choice during task preparation and refresh their endpoint on agent
restart. The authenticated desktop API owns configuration writes, using the
existing parser, migration checks and atomic owner-only file replacement.
Preview snippets never contain the pairing credential.

A local selection permits no-auth requests only on the loopback `/mcp` route.
The listener binds to `127.0.0.1`; Origin and Host guards remain mandatory. It
forwards the paired credential upstream. The mode is disabled by default and
ends when no provider selects local access. Repository overrides cannot enable
it. Other local API routes and every remote machine surface remain authenticated,
so ADR 0019's remote authentication requirement remains unchanged.

## Consequences

Local mode grants native processes on the workstation access to MCP as the
paired user; the settings UI states this before the update action. It requires
the running agent. Remote mode stores the revocable key in the provider file
and operates independently of the daemon. STDIO starts the installed bridge;
HTTP connects directly to the selected endpoint. Existing provider permissions
and unrelated registrations survive switching modes.

Provider schema references:

- [Codex MCP](https://developers.openai.com/codex/mcp)
- [Antigravity MCP](https://antigravity.google/docs/mcp)
- [Vibe MCP](https://docs.mistral.ai/vibe/code/cli/mcp-servers)
