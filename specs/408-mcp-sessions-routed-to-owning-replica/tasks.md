# #408: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Store (FR7, FR8 lookups)

- [x] T1.1 `LiveInstance(id)` and `LiveInstances()` in `internal/db/instances.go`.
- [x] T1.2 `interruptedClientRun` carries `models.RunDisconnectNote`; both reclaim paths
      use it.
- [x] T1.3 Tests on SQLite and PostgreSQL: lookups exclude dead instances; the owner
      rewrites a run reclaimed by a restart and by a dead instance through
      `FinishRemoteRunAs`; a person's cancellation stays final.

## 2. Session identity (FR1, FR8)

- [x] T2.1 `GetSessionID` returns `<instanceID>.<random>` in `NewServerWithCallers`.
- [x] T2.2 `SessionOwner(id)` helper.
- [x] T2.3 `SessionView.Instance`, filled by `Snapshot`.
- [x] T2.4 Unit tests for T2.1 to T2.3; existing `taskmcp` tests adjusted to the id
      format only.

## 3. Routing and forwarding (FR2 to FR6, FR9)

- [x] T3.1 Build the `StreamableHTTPHandler` once per `Handler`; share it between the
      public and internal routes.
- [x] T3.2 Factor the constant-time bearer check of `agentCluster` to take a header
      name; `internalAuth` on `X-Sectile-Internal-Authorization`.
- [x] T3.3 `mcpRouter`: local, dead owner (local `404`), live owner (proxy); strip the
      internal header from public requests.
- [x] T3.4 Streaming reverse proxy: flush on every write, client context, dial timeout,
      `503` error handler, no retry, no buffering.
- [x] T3.5 `InternalMCPHandler()` serving `/internal/mcp` (internal + client auth, never
      routes) and `/internal/mcp/sessions` (internal auth).
- [x] T3.6 `EnableAgentCluster` enables MCP routing; missing key disables it with the
      logged reason.
- [x] T3.7 Router unit test with a fake instance lookup.

## 4. Sessions view (FR8)

- [x] T4.1 `HandleMCPSessions` aggregates live instances in parallel, 2 s each,
      `unreachable` list, merged sort; local-only when the cluster is off.
- [x] T4.2 `web/src/lib/mcpSessions.ts`: optional `instance` and `unreachable` in the
      types, no rendering change.

## 5. Server wiring

- [x] T5.1 `startInternalListener` registers the MCP internal routes.

## 6. Two-instance tests (success criteria)

- [x] T6.1 `internal/handlers/mcp_cluster_test.go` on PostgreSQL: alternating client
      (initialize, tools, runs, `GET` stream, `DELETE`), dead owner `404` and
      re-initialize, unreachable owner `503` once, refused credentials, aggregated view
      with and without an unreachable instance.

- [x] T6.2 The same scenarios on SQLite with a directory the test controls
      (`internal/handlers/mcp_cluster_test.go`), so they run on every `go test`;
      the PostgreSQL test (`postgres_mcp_cluster_test.go`) joins the `test:postgres`
      CI job, with `-p 1` since both packages empty the same database.

## 7. Documentation (FR10, FR11)

- [x] T7.1 ADR 0007 third amendment.
- [x] T7.2 README: internal port also carries MCP forwarding; no SSE buffering on
      `GET /mcp`.
- [x] T7.3 CHANGELOG `[Unreleased]` lines (`Fixed`, `Changed`).

## Test plan

- `go build ./...`, `go vet ./...`, `go test ./...`.
- PostgreSQL subset with a throwaway `SECTILE_TEST_POSTGRES_DSN` (never the dev
  database): `go test ./internal/db/... ./internal/handlers/... -run 'Instance|Reclaim|MCP'`.
- `go test -race` on `internal/taskmcp` and the new handler tests.
- `npm test` in `web`.
- Manual check, optional: two servers on a copied database without tracker tokens, one
  MCP client pointed through a round-robin proxy, then one server killed.
