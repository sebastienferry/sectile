# ADR 0002: Native coding clients and a local launcher

## Status

Accepted. Supersedes the experimental Electron chat companion.

## Decision

Keep TaskFlow's web interface for task management and dispatch. Use the native
Codex/Claude interface for conversations, tool approvals and execution. The local
agent owns outbound server connectivity, project configuration, worktree
preparation, native terminal launch and an MCP gateway. Do not recreate a chat
interface or provider-specific approval protocol in TaskFlow.

There are two entry points: a user invokes a pickup skill from the native coding
client, or the web dispatches that skill to the local agent. Both use the same
skills and server MCP tools for descriptions, comments and workflow transitions.
An explicit project connection bootstraps MCP in the selected repository; web
launches also bootstrap the target worktree before starting the CLI.

## Consequences

Native clients own provider authentication and user approvals. TaskFlow avoids
maintaining a separate Electron runtime, conversation history and approval UI.
The agent must remain running for its local MCP gateway to work. A future tray
utility may manage agent configuration and lifecycle, but is not required and
must not duplicate the native chat interface.
