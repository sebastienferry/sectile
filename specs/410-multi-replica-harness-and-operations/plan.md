# #410: Technical plan

References: [`spec.md`](spec.md), [`docs/clarifications/410.md`](../../docs/clarifications/410.md).

## Readiness: `GET /api/ready`

- `handlers.ReadyPath = "/api/ready"`, public in `RequireSession` like `HealthPath`.
- `Handler.HandleReady`: 503 `{"status":"not_ready","reason":...}` when draining, when
  the store does not answer a ping (`DB.Ping(ctx)`, 2 s), or, on a shared store, when
  this instance's row is missing (`DB.InstanceRegistered()`) or the internal listener is
  not serving (`Handler.SetInternalServing(bool)`, set by `startInternalListener`).
  Otherwise 200 `{"status":"ready","instance":<id>}`.

## Drain on SIGTERM / SIGINT

- `cmd/server/main.go` serves through an `http.Server`, keeps the stop function
  `StartInstance` returns, and waits on `signal.NotifyContext(SIGTERM, SIGINT)`.
- On the signal: `h.BeginDrain()` (readiness 503), sleep the grace
  (`SECTILE_SHUTDOWN_GRACE`, Go duration, default `5s`, `0` allowed), then the instance
  stop (removes its row), `h.CloseAgentConnections()` (close code 1001 "Going Away",
  presence rows marked disconnected), `server.Shutdown` bounded by 5 s, exit 0.
- SQLite: the same sequence; the instance stop only removes a row nobody else reads.

## Liveness bounds for tests

`db.SetInstanceTiming(heartbeat, deadAfter, reclaim time.Duration)`; the server reads
`SECTILE_INSTANCE_HEARTBEAT`, `SECTILE_INSTANCE_DEAD_AFTER`, `SECTILE_INSTANCE_RECLAIM`
(Go durations) before `StartInstance`; an unparsable or non-positive value is fatal;
unset keeps 10 s / 45 s / 15 s. Documented as test settings.

## Harness: `cmd/server/multireplica_test.go`

`TestPostgresMultiReplicaHarness`, skipped without `SECTILE_TEST_POSTGRES_DSN`.

- Builds the server once (`go build -o <tmp>/sectile-server .`).
- Starts replicas as child processes with a clean environment: `DB_DRIVER=postgres`,
  `DATABASE_URL`, distinct `PORT` and `SECTILE_INTERNAL_PORT`,
  `SECTILE_INTERNAL_URL=http://127.0.0.1:<port>`, one `SECTILE_SECRET_KEY`, one
  `SECTILE_SERVER_TOKEN`, short liveness bounds and grace, an isolated `HOME` and
  `XDG_CONFIG_HOME`, working directory a temp dir. Output goes to a per-replica log,
  printed when the test fails. Waits for `/api/ready` 200.
- A balancer in the test (`httptest` server, `httputil.ReverseProxy`, WebSocket
  upgrade included) sends each request to the first replica of its list that accepts a
  connection, so placement is deterministic: A while it lives, then B.
- A scripted agent dials `/ws/agent-connect?projectId=default&deviceId=harness` through
  the balancer with the server token, answers `pull_tasks` with an empty
  `running_tasks` and every `workspace_request` for `git_branches` with a
  `GitBranchesInfo`, and reconnects on its own when the connection drops.
- REST and SSE use a web session from `POST /auth/local`; MCP uses the server token
  with the go-sdk client.

Scenario:

1. A and B ready; sign in; create a project through A.
2. Agent connects (lands on A). `GET /api/git/branches?projectId=` through B answers
   the agent's branch: cross-replica operation.
3. `GET /api/events` on B; create a task and post back a title change through A; the
   stream on B carries `task_updated` for that task: SSE propagation.
4. MCP session initialized on A, `list_projects` called through B: session across
   replicas.
5. SIGKILL A. The agent reconnects through the balancer to B; the git operation through
   B answers again. The MCP session's next call through B gets `404` once A is dead;
   a new session on B works.
6. Restart A (A2). SIGTERM B: `/api/ready` on B answers 503 within the grace while
   `/api/health` still answers 200; B exits 0; the agent reconnects to A2; A2 no longer
   lists B among the live instances before the dead-after bound.

## CI

`test:postgres` runs `go test -p 1 ./internal/db/ ./internal/handlers/ ./cmd/server/
-run 'TestPostgres|TestMigrate'`.

## Documentation

- `docs/adrs/0030-several-server-replicas-share-one-postgresql.md`: context, decision
  (active/active on PostgreSQL, the eight child tickets), consequences, operator
  contract.
- ADR 0016: amendment pointing at ADR 0030.
- README: "Several replicas" section: requirements, variables (including the test
  bounds), probes, drain and termination grace, disruption budget, replica count,
  ingress timeouts for WebSocket (`/ws/agent-connect`) and SSE (`/api/events`,
  `GET /mcp`), sticky sessions not required.
- CHANGELOG: one line under `[Unreleased]`.

## Target files

`cmd/server/{main.go,internal.go,multireplica_test.go,shutdown.go}`,
`internal/handlers/{ready.go,auth.go,agent_dispatcher.go}`, `internal/db/instances.go`,
`.gitlab-ci.yml`, `docs/adrs/0030-...md`, `docs/adrs/0016-...md`, README, CHANGELOG.
