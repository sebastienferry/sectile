# ADR 0061: Codex interruption terminates the local provider tree

- Status: Accepted
- Date: 2026-10-28

## Context

Codex app-server can acknowledge `turn/interrupt` while a unified-exec shell tool
continues in a separate session. A manual desktop test confirmed a delayed file
write after interruption and after the conversation had already resumed.
An interrupted turn notification is not evidence of tool termination.

## Decision

A desktop Codex interruption retires the dedicated app-server and its local
process descendants before reporting the conversation idle. The conversation
retains its thread ID; the next message starts a new app-server and calls
`thread/resume`. No message is retried automatically.

On Unix, `agentexec.StopTree` freezes the provider, enumerates descendants through
parent IDs, freezes descendants until the tree stabilizes, then kills descendants
before their parent. This includes tools that created separate process groups or
sessions. Enumeration is bounded and failures are reported. On Windows, the
existing per-provider Job Object is terminated. Missing job ownership is an error.
The operation only addresses the dedicated supervised provider tree.

## Consequences

Interruption has process-startup cost on the next turn. It no longer relies on a
provider acknowledgement to declare tools stopped. Killing a process cannot undo
side effects already performed and cannot cancel work delegated to remote services.
This is local process containment, not a rollback mechanism. Tools that daemonized
and reparented before enumeration are outside this parent-tree guarantee on Unix.

A regression test spawns a delayed writer in a separate session. Real-provider
verification additionally waits past a 90-second delayed-write deadline and checks
that the same Codex conversation can resume.
