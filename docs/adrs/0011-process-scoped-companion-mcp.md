# ADR 0011: Supply companion MCP per provider process

Status: Accepted
Date: 2026-10-29

## Context

Electron and the standalone Tauri runtime can run side by side. They share the
installed provider CLIs and their authentication, but reach different Sectile
servers. Replacing the user-level `sectile` MCP entry connects one desktop to the
other's tracker and execution state. Disabling automatic refresh prevents that
write but leaves companion conversations using the unrelated registration.

## Decision

The native companion passes `SECTILE_COMPANION_MCP_URL` to its Go agent, naming
its own loopback server. Go supplies a `sectile_companion` STDIO bridge to native
Claude and Codex launches. The bridge uses the running agent executable and the
paired identity inherited privately through its environment. Command arguments
and renderer metadata contain no key. Do not copy CLI credentials or change CLI
homes, session storage or shared configuration files.

Claude receives strict process-local MCP configuration. Codex disables the shared
`sectile` entry only in its process and adds the companion under a distinct name.
Its configuration tables merge: replacing a shared entry's transport can retain
incompatible fields, so a separate name is necessary. Other Codex MCP entries
keep their own configuration. Electron launches retain their existing behavior.

Readiness identifies this configuration as session-scoped. Companion setup and
bootstrap do not rewrite user entries. Live provider status is distinct from the
idle Claude transport check. Running Codex processes need a restart to acquire a
new configuration; retained conversation identities resume on the next message.

Codex's MCP elicitation request is an explicit decision in the conversation.
Consent has allow-once and decline, without persistent approval. Bounded boolean
and enumerated form fields retain their wire types. Unsupported schemas and URL
flows are refused. Only proposed tool arguments are shown, not protocol metadata.

## Consequences

Native built-in skill, autonomous, console and conversation launches receive the
connection. Direct Claude/Codex command templates must carry `{mcpArgs}` to opt
into their supplied arguments; a template that omits the required slot is refused
rather than silently using the other desktop's registration.

Verification combines argument/environment and no-write regression tests, a real
Codex MCP read after desktop approval, a desktop-launched Claude skill chain, and
a read-only call through Electron's unchanged shared MCP registration. The chain
uses a disposable GitLab HTTP fixture, real local Git worktrees and the installed
forge CLI with repository-local fixture configuration. No production tracker is
used for transition tests.
