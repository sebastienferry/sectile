# Plan #517 - MCP sessions kept alive, orphans released

## Stack

Go, `internal/taskmcp` (session registry), `internal/handlers` (settings),
go-sdk v1.7.0 (`ServerSession.Ping`, streamable HTTP).

## What exists

- `SessionRegistry` keeps `live` sessions with `lastSeen`, adopted `runs` and the
  transport `session`, and runs one sweeper goroutine (`sweep`) for silences.
- `Touch` is called by a receiving middleware, which sees client methods only.
- `NewSessionRegistryBounded(runs, notes, bound, abandon)` is built in
  `internal/handlers/handlers.go` from `mcpSilenceNotice()` and `mcpAbandonAfter()`.

## Decisions

### 1. A second loop in the registry

`NewSessionRegistryBounded` keeps its signature. A new
`SetKeepalive(interval time.Duration, failures int)` configures and starts the
keepalive loop, stopped by the same `stop` channel. A registry without it (tests,
hosts without HTTP) does not ping, which keeps every existing test as is.

Each tick snapshots the sessions that have a transport, releases the lock, and
pings them concurrently (one short-lived goroutine per ping, bounded by the
timeout), then waits for all of them before the next round. Results are applied
under the lock.

Rejected: go-sdk `ServerOptions.KeepAlive`. On failure it closes the session,
which reaches `SessionRegistry.Close` and cancels adopted runs.

### 2. The failure count

`liveSession.pingFailures int`. A successful ping resets it to 0, and so does
`Touch`. A failure increments it. When it reaches the threshold and
`len(entry.runs) == 0`, the session is closed exactly as an abandoned one is:
`r.Close(id)`, then `session.Close()`. With runs, one log line is written when
the threshold is first crossed, and the count keeps growing without closing.

### 3. What counts as an answer

With `JSONResponse: true` and no `EventStore`, a ping to a session whose client
has no GET stream open fails at once with the transport's own refusal, which is
itself a `*jsonrpc.Error`. So only `CodeMethodNotFound` (a client without ping
support that did answer) resets the count; every other error is a failure.

### 4. Settings

`internal/handlers`: `mcpKeepaliveInterval()` and `mcpKeepaliveFailures()` next
to `mcpSilenceNotice()`, with the same parsing rules. The registry is given them
right after construction in `handlers.go`.

## Target files

| File | Change |
| --- | --- |
| `internal/taskmcp/sessions.go` | exported defaults, `pingFailures`, `SetKeepalive`, keepalive loop, reset in `Touch` |
| `internal/taskmcp/sessions_keepalive_test.go` | new: US1-US4 over `httptest` + go-sdk client |
| `internal/handlers/agent_api.go` | two settings readers |
| `internal/handlers/handlers.go` | `SetKeepalive` on the registry |
| `internal/handlers/mcp_sessions_test.go` | settings parsing (US5) |
| `README.md`, `.env.sample`, `docs/contracts/server-agent-v1.md`, ADR 0007, `CHANGELOG.md` | FR6, FR7 |

## Risks

- A client that never opens the standalone stream fails every ping at once. The
  `Touch` reset keeps it alive while it talks. If it goes quiet with no run for
  75s, it is closed and re-initializes on its next call: one extra
  `initialize`, not a leak.
- The keepalive test binds a local port (`httptest`). It needs the sandbox's
  local binding, see `.agents/MEMORY.md`.
