# #408: Technical plan

References: [`spec.md`](spec.md), [`docs/clarifications/408.md`](../../docs/clarifications/408.md),
[`docs/adrs/0007-mcp-session-ownership.md`](../../docs/adrs/0007-mcp-session-ownership.md),
[#406 plan](../406-agents-reachable-from-any-replica/plan.md).

## Stack and constraints

- Go, `github.com/modelcontextprotocol/go-sdk` v1.7.0 (`mcp.StreamableHTTPHandler`,
  `ServerOptions.GetSessionID`, `404 session not found` on an unknown id).
- Liveness is `server_instances` (#403: heartbeat 10 s, dead after 45 s, reclaim every
  15 s). Addresses and the internal listener come from #406 (`SECTILE_INTERNAL_PORT`,
  `SECTILE_INTERNAL_URL`, `db.InternalToken()`).
- No migration: the owner is in the session id, liveness and address are already stored.
- The internal listener only starts when `database.Shared()`; SQLite never forwards.

## Session id (FR1)

`taskmcp.NewServerWithCallers` sets `ServerOptions.GetSessionID` to
`func() string { return database.InstanceID() + "." + rand.Text() }` (`crypto/rand`,
the SDK's own default for the random part). Instance ids are UUIDs, so the id stays a
header-safe token.

`taskmcp.SessionOwner(id string) string` returns the part before the first `.`, or `""`
when there is none (an id from an earlier version, or a client-forged one).

## Store: `internal/db/instances.go`

- `type InstanceLocation struct { ID, Address string }`.
- `LiveInstance(id string) (InstanceLocation, bool)`: the row with `last_seen >= now -
  instanceDeadAfter`.
- `LiveInstances() []InstanceLocation`: every such row, ordered by id.
- Both read under `d.mu` like the other presence reads.

## Recoverable reclaim (FR7)

- `interruptedClientRun` becomes
  `"Interrupted by server restart: " + models.RunDisconnectNote`, so the existing
  `LIKE %RunDisconnectNote%` guard of `FinishRemoteRunAs` (`remoterun.go`) and of the
  macro-run finish (`macroruns.go`) lets the owner rewrite it. Both reclaim paths use
  the constant: the single-process branch of `recoverInterruptedRuns` and
  `reclaimDeadInstances`. Nothing else changes in the reclaim (R4).
- `FinishRemoteRunAs` already requires an identified caller for that rewrite, so the
  server's own closure paths still cannot rewrite it.
- Runs canceled by a person carry their own summary and stay final.

## MCP handler: `internal/handlers/agent_api.go` and new `internal/handlers/mcp_cluster.go`

One `mcp.StreamableHTTPHandler` per `Handler`, built once (it holds the sessions in
memory) and shared by the two routes:

- **Public `/mcp`**: `AgentAPIAuth(mcpRouter(streamable))`.
  `mcpRouter` reads `Mcp-Session-Id`; when the cluster is enabled and
  `SessionOwner(id)` is non-empty and differs from `h.db.InstanceID()`:
  - `LiveInstance(owner)` false → serve locally (the SDK answers `404`);
  - true → reverse-proxy to `<address>/internal/mcp`;
  - otherwise serve locally.
- **Internal `/internal/mcp`** on the internal listener:
  `internalAuth(AgentAPIAuth(streamable))`. It never routes, which is the loop guard
  (FR3): a forwarded request is served where it lands, and answers `404` if the
  session is not there.

### Credentials on a forwarded request (FR6, resolved choice)

#406 carries the internal bearer in `Authorization`, which the client's bearer already
occupies on `/mcp`. The forwarded request keeps the client's `Authorization` intact and
carries the internal credential in `X-Sectile-Internal-Authorization: Bearer <token>`.
`internalAuth` checks that header in constant time (the `agentCluster.authorized`
comparison, factored to take the header name), then `AgentAPIAuth` checks the client's
bearer exactly as the public route does, so `mcpCaller` resolves the same user on the
owner. The public router strips any incoming `X-Sectile-Internal-Authorization` before
proxying, and the public listener never serves `/internal/mcp`.

### Reverse proxy (FR2, FR5)

`httputil.ReverseProxy` built per request target (or one with a `Rewrite` func):

- `Rewrite`: target URL `<address>/internal/mcp`, method, query, body and headers kept,
  `SetXForwarded()`, the internal header added.
- `FlushInterval: -1` so each SSE event and each JSON answer is flushed as it comes.
- Transport: dedicated `http.Transport` with a dial timeout (5 s) and no response or
  idle timeout on the body, since the `GET` stream is long-lived; the request context
  is the client's, so a client that disconnects ends the upstream stream.
- `ErrorHandler`: `503` with the message
  `the server instance holding this MCP session (<id>) did not answer: <err>`; no retry
  (the request may have reached the owner, as #406 decided).
- Request bodies are not buffered: nothing is replayed.

### Enabling

`Handler.EnableAgentCluster()` (called by `startInternalListener`) also records the
internal token for MCP. When the token is missing (`tokenErr`), `mcpRouter` never
proxies and `internalAuth` refuses with `401`; the warning already logged by #406 names
the reason, extended to mention MCP. On SQLite `EnableAgentCluster` is never called, so
routing is off.

## Sessions view (FR8)

- `taskmcp.SessionView` gains `Instance string \`json:"instance"\``, filled by
  `Snapshot` with the registry's instance id (given to the registry at construction,
  or set by the handler before serving).
- Internal `GET /internal/mcp/sessions` (internal auth only, no client bearer) returns
  `{"sessions": Snapshot()}`.
- `HandleMCPSessions`:
  - cluster off → `{"sessions": local, "unreachable": []}`;
  - cluster on → local snapshot plus, in parallel, each other `LiveInstances()` entry
    fetched with a 2 s context; failures and timeouts go to `unreachable` (instance
    ids); the merged list is sorted as `Snapshot` sorts (most recent first, then id).
- Answer shape: `{"sessions": [SessionView...], "unreachable": ["<instance id>"...]}`.
  Adding fields keeps the web client compatible; `web/src/lib/mcpSessions.ts` types
  gain optional `instance` and `unreachable` with no rendering change.

## Server wiring: `cmd/server/internal.go`, `cmd/server/main.go`

- `startInternalListener` registers `/internal/mcp` and `/internal/mcp/sessions` next to
  `/internal/agent/`, from `h.InternalMCPHandler()`.
- `/mcp` and `/api/mcp/sessions` public routes unchanged in `main.go`.

## Documentation

- ADR 0007, third amendment (#408): a session is owned by the instance that created
  it, its id names that instance, any instance forwards to the owner over the internal
  listener, the session dies with its owner (`404`, re-initialize), and a run lost with
  a server is recoverable through `finish_run`. Nothing else of the decision changes.
- README: next to `SECTILE_INTERNAL_PORT` / `SECTILE_INTERNAL_URL`, say that the internal
  port also carries MCP session forwarding, and that the balancer must not buffer the
  SSE stream of `GET /mcp`.
- CHANGELOG `[Unreleased]`: `Fixed`: an MCP client keeps its session whichever server
  replica receives its requests; `Changed`: a run lost with a restarted or stopped
  server can still be reported on through `finish_run`.

## Tests

- `internal/db`: `LiveInstance` / `LiveInstances` on SQLite and PostgreSQL (dead row
  excluded); reclaim tests assert the new summary and that `FinishRemoteRunAs` by the
  owner rewrites a reclaimed run, on both paths; a person's cancellation stays final.
- `internal/taskmcp`: `SessionOwner` parsing; `GetSessionID` prefix; `Snapshot` fills
  `Instance`.
- `internal/handlers/mcp_cluster_test.go` (PostgreSQL, skipped without
  `SECTILE_TEST_POSTGRES_DSN`, as the other cluster tests): two `Handler`s on one test
  database, each with a public and an internal `httptest` server, instance addresses
  set to the internal servers; a go-sdk client whose transport alternates between the
  two public servers:
  - initialize on A, tool calls and `start_run`/`finish_run` through B, run adopted on A;
  - `GET` stream through B receives a server notification emitted on A;
  - `DELETE` through B closes the session on A and its running runs;
  - owner row aged past the bound → `404` through B, re-initialize on B works;
  - owner live but its internal server closed → `503`, one upstream attempt;
  - wrong internal header, or missing client bearer → owner refuses;
  - `/api/mcp/sessions` lists both, then one unreachable.
- A handler-level test without PostgreSQL covers the router decisions with a fake
  instance lookup (local, own id, unknown prefix, dead owner, live owner).
- `web`: existing unit tests for `mcpSessions.ts` stay green.

## Target files

`internal/taskmcp/{server.go,sessions.go}`, `internal/db/instances.go`,
`internal/handlers/{agent_api.go,agent_cluster.go,handlers.go,mcp_cluster.go}`,
`cmd/server/internal.go`, `web/src/lib/mcpSessions.ts`, tests beside each,
`docs/adrs/0007-mcp-session-ownership.md`, `README.md`, `CHANGELOG.md`.
