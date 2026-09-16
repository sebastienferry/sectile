# Agent connection keepalive and reconnection-tolerant operations

## Why
The local agent WebSocket dropped and re-established three times in seven minutes right after
the desktop app was started, then settled once the machine went idle. While it flapped, a
`reviewed` stage transition on #123 failed outright with `no local agent connected for project
515fa50d-...`, because every stage transition validates the checkout through the agent's
`git_evidence` operation. The same call succeeded a minute later, unchanged.

Two defects were found in the keepalive path. Neither is proven to be the cause of that
particular incident, and the change says so rather than claiming a fix it cannot demonstrate.

**The pong can be dropped without a trace.** The agent answers the server's pings with
gorilla's default ping handler, which writes the pong with `WriteControl` under a hard-coded
one-second budget. `WriteControl` has to take the connection's write mutex, and returns
`errWriteTimeout` when it cannot within that second. That error is declared temporary, so the
default handler swallows it and reports success: the pong is discarded and nothing is logged
anywhere. The mutex is held for the duration of each frame flush, so the window is one stalled
socket write — narrow on a healthy loopback link, real as soon as the socket backs up. A test
reproduces it: with a frame flush held, the probe is never answered.

**Nothing has any margin.** The agent heartbeat ran at 30 seconds and the server read timeout
at 30 seconds. The heartbeat is the only keepalive left once the pong path fails, and it
arrived exactly on the deadline, so whether the connection survived came down to scheduling
order.

**And a drop was undiagnosable.** The server hung up without a close frame, so the agent log
only ever showed `close 1006 (abnormal closure)` — neither a reason nor a distinction from a
rebound session. Seven minutes of flapping left no usable trace, which is why the cause of that
incident cannot be stated today.

## What Changes
- The agent answers server pings from a dedicated sender that never competes for the
  connection write mutex on the read loop's time budget, so a pong is no longer dropped
  because an operation result is being written.
- A pong that still cannot be sent is logged rather than silently discarded.
- The agent heartbeat period and the server read timeout no longer share a value, and the
  margin between them is stated in a comment at each constant.
- The server closes a timed-out agent connection with an explicit close code and reason, and
  the agent logs the reason it was disconnected instead of an anonymous abnormal closure.
- A workspace operation addressed to an agent that is momentarily absent waits, within the
  caller's own deadline, for it to reconnect. If it does not, the error says the agent is
  reconnecting and that the call can be retried.

## Impact
`cmd/agent/agent.go` (keepalive and reconnection logging), `internal/handlers/agent_dispatcher.go`
and the agent WebSocket loop in `internal/handlers/handlers.go`, plus their tests. No schema
change, no new endpoint, no protocol message added or removed.

Explicitly not claimed: that this closes the incident of 2026-09-16. It removes a silent
failure mode, gives the keepalives a margin, keeps a stage transition working across a
reconnection, and makes the next occurrence name itself in the agent log.
