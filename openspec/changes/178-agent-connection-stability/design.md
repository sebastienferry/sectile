# Design

## Context
Three pieces of code decide whether an agent connection survives.

`internal/handlers/agent_dispatcher.go` pings the agent every `defaultAgentPingInterval`
(10s) with `WriteControl`, which is the one write method safe to call concurrently.
`internal/handlers/handlers.go` sets a read deadline of `defaultAgentReadTimeout` (30s) and
refreshes it on every frame, pongs included. `cmd/agent/agent.go` sends an application-level
`heartbeat` message every 30s and otherwise serialises all its writes through `d.connMu`.

The pong itself is written by gorilla, not by this repository. `SetPingHandler(nil)` installs
a handler that calls `WriteControl(PongMessage, ..., now+writeWait)` with `writeWait` fixed at
one second (`websocket@v1.5.3/conn.go:35`). `WriteControl` takes the connection's internal
write mutex, and returns `errWriteTimeout` if it cannot get it in time
(`conn.go:443`). `errWriteTimeout` is `&netError{timeout: true, temporary: true}`
(`conn.go:177`), and the default handler explicitly returns `nil` for a temporary error
(`conn.go:1158`). The pong is dropped, the read loop carries on, and no log line is produced
anywhere.

That mutex is contended exactly when the agent is busy: `handleOperation` holds `d.connMu` and
writes the result with a five-second write deadline (`cmd/agent/agent_operations.go:53`). Any
result that takes longer than a second to flush — a large `git_diff`, a `skill_files` listing —
costs a pong.

## Decisions

### The pong gets its own sender, not a longer deadline
Raising `writeWait` would require replacing the ping handler anyway, and a larger value simply
moves the cliff while blocking the read loop for longer. Instead the ping handler hands the
pong to a dedicated goroutine through a one-slot channel and returns immediately. The sender
calls `WriteControl` with a budget close to the read timeout, so it waits for the write mutex
instead of giving up on it, and the read loop never blocks on a write.

One slot is enough: pongs are not cumulative. If a pong is still queued when the next ping
arrives, the queued one answers both, which is exactly the semantics the protocol allows.

**Rejected:** routing the pong through `d.connMu` like every other write. It would serialise
the keepalive behind whatever large payload is in flight, which is the bug.

**Rejected:** dropping the server ping and relying on the application heartbeat alone. The
pong path is the one that detects a peer that has stopped reading; the heartbeat only proves
the agent is still writing.

### A dropped pong is logged
When the sender cannot write the pong even with its larger budget, it logs it. This is the
failure that produced seven minutes of unexplained flapping with no trace in any log, so it
gets a line naming what could not be sent and why.

### The heartbeat and the read timeout get an explicit margin
The agent heartbeat moves to 10s and the server read timeout to 45s. Both constants carry a
comment stating the margin: with a keepalive from each side every 10s, at least four
consecutive keepalives must be lost before the server drops the connection. The two values are
deliberately different so neither can mask the other by arriving exactly on the deadline, which
is what made the old behaviour a coin flip.

The heartbeat stays as a second keepalive rather than being removed: it is the agent's own
proof of life, and it is the only keepalive that survives a broken pong path.

### The server states why it hung up
On a read deadline the server sends a close frame with code 4002 and a reason naming the
silence window, before closing. The agent logs the close code and reason it receives. Today it
reports `websocket: close 1006 (abnormal closure)`, which says nothing about why. The reason
lands in the agent log the desktop already surfaces (#98), which is where the user reads it.

4002 sits next to the existing 4001 ("Session Rebound") in the private close-code range and
distinguishes "you went silent" from "somebody took your slot".

### An operation waits for a reconnecting agent
`CallOperation` fails immediately when no agent is registered. That is correct when nothing is
running locally and wrong during a reconnection that lasts a second. It now waits for an agent
to appear, bounded by the smaller of a short grace and the caller's remaining deadline, and
polls the registry rather than growing a subscription mechanism for a wait measured in
seconds.

The grace is deliberately short. `git_evidence` is given 15s by
`internal/db/agentoperations.go`, so the wait must stay well inside it and still leave the
operation itself room to run. A reconnection that takes longer than the grace is no longer a
hiccup, and the caller is better served by an error it can act on.

That error names the reconnection and the retry. "no local agent connected" reads as a
configuration problem; the actual situation is transient and retrying is the fix.

**Rejected:** queueing the operation until the agent returns. The caller is an HTTP request
with its own deadline; a queue would only move the timeout somewhere less visible.

## Verification of the idle requirement
Thirty minutes of idle time cannot be asserted in the Go suite. The automated test proves the
mechanism instead: with a write held on the connection for longer than gorilla's one-second
budget, a ping still receives its pong. The wall-clock requirement is verified manually by
sampling `GET /api/agent/status` and checking that `connectedAt` does not change, a procedure
recorded with the change.

## Risks
The pong sender adds one goroutine per connection, stopped with the connection's context. If
it ever leaked, it would hold a reference to a closed connection and its writes would fail
immediately, which is contained but worth keeping tied to the same lifetime as the read loop.
