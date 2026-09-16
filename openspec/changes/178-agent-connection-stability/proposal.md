# Agent connection keepalive and reconnection-tolerant operations

## Why
The local agent WebSocket dropped and re-established three times in seven minutes right after
the desktop app was started, then settled once the machine went idle. While it flapped, a
`reviewed` stage transition on #123 failed outright with `no local agent connected for project
515fa50d-...`, because every stage transition validates the checkout through the agent's
`git_evidence` operation. The same call succeeded a minute later, unchanged.

The cause is in the pong path, and it is silent. The agent answers the server's pings with
gorilla's default ping handler, which writes the pong with `WriteControl` under a hard-coded
one-second budget. `WriteControl` has to take the connection's write mutex, and returns
`errWriteTimeout` when it cannot within that second. That error is declared temporary, so the
default handler swallows it and reports success: the pong is dropped and nobody is told.

The agent holds that write mutex for every message it sends — each operation result is written
under `d.connMu` with a five-second deadline. Right after startup the agent writes constantly
and with large payloads (`sync_config`, `workspace_info`, `git_diff`, `skill_files`), so pongs
are lost. The only keepalive left is the agent's own heartbeat, which runs at exactly the
server's read timeout, thirty seconds: it arrives on the deadline, so whether the connection
survives is a coin flip. Once the startup burst is over the agent stops writing, pongs get
through, and the connection is stable. That is precisely the reported profile.

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
