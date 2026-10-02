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
Preview snippets never contain the pairing credential. The UI offers three explicit choices: remote HTTP (default), local HTTP proxy,
and STDIO to the remote server. Selecting local HTTP does not start a STDIO
bridge. The lower-level API retains support for STDIO through the local proxy. Existing saved combinations are
not rewritten until the user applies a selection.

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

## Amendment (2026-10-03, #716)

A provider without a saved selection kept its bootstrap default, and the agent
start rewrites only saved selections. Claude Code's `sectile` entry written by
`sectile-agent init`, **Initialize** or `sync_config` was therefore never
rewritten: after a new pairing it kept the revoked key and Claude Code could no
longer connect.

- `init`, **Initialize** and `sync_config` now record the Claude Code
  registration they write as the saved choice `remote/http`, with its
  fingerprint. An existing saved choice is honoured as it is, local included.
  The agent start then rewrites the entry after a new pairing, through the
  fingerprint rule of ADR 0039. The other providers keep their bootstrap
  default unrecorded.
- An entry in `~/.claude.json` that the agent did not save and whose key is not
  the daemon's (the user-scope entry, or a project-scope entry, which Claude
  Code prefers over the user one) is only reported: the MCP settings flag it
  and offer **Repair**. It is rewritten, and the outdated project entries are
  removed, only when the user clicks **Repair**. Nothing adopts it unasked.
- No key appears in any preview or response: the desktop learns, per entry,
  whether the key and the address match, never the key. The start-up rewrite
  logs the file it rewrote, not the key or its fingerprint.
