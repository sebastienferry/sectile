# ADR 0054: Codex conversations use app-server

- Status: Proposed
- Date: 2026-10-06
- Extends: [ADR 0047](0047-claude-conversations-speak-the-streaming-input-protocol.md)

## Context

Desktop already renders Claude conversations through an agent-owned transcript
and local controls. Its state, launch gates and composer assumed Claude's
streaming input protocol. Codex needs the same interactive launches, tool
approvals and follow-up messages, with its own permission semantics.

## Decision

Use Codex's documented app-server protocol over stdio, with one process per
conversation. Keep Claude's existing process-per-turn transport. Both feed the
same bounded transcript, approval cards, waiting state and Desktop endpoints.
The provider-specific state is held under the agent's queue lock; RPC reply
correlation uses an independent lock, and no RPC wait holds the queue lock.

Initialize on opening the Codex view without model inference, then create a
thread and discover models, supported efforts, skills and MCP state. Use native
skill inputs and paths. Preserve the CLI's existing authentication and MCP
configuration, and launch through `agentexec.Hidden` and `StartDetached` on
all platforms.

Codex modes select read-only or workspace-write sandbox policies and default
or plan collaboration instructions. Network access stays restricted. Explicit
session approvals never become Claude project rules or permanent Codex policy.
Unsupported server requests receive an error rather than hanging the process.

Use `turn/steer` for messages sent during a turn. Replay as a new turn only
when an RPC rejection and the turn-completed event confirm the boundary. Do not
retry ambiguous transport failures. Interrupt the turn first; terminate a
process that does not confirm interruption. Stop execution confirms process
exit, including while initialization is waiting. A process crash retains the
thread id for the next message's `thread/resume`.

## Consequences

Claude and Codex share a UI without pretending their modes or approval scopes
are identical. An older agent still opens the terminal; an incompatible Codex
CLI reports its startup or protocol error in the conversation. The schema and
non-inference handshake and a real read-only turn were verified against CLI
0.157.1. Protocol changes
need new fixtures and compatibility checks.

Agent-restart recovery remains read-only for both providers. The Codex child
inherits the task environment at process startup; folder sandbox roots are
read again for each turn. Account entitlements are not inferred from the model
catalog. No authentication flow, hook, tracker schema or server orchestration
change is introduced.

Reference: [Codex App Server](https://learn.chatgpt.com/docs/app-server).
