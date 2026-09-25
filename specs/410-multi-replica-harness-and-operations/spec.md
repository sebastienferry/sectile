# #410: Multi-replica verification harness, operations and documentation

Parent macro: #397 (child ticket 8, FEAT-5 / US-1). Clarification:
[`docs/clarifications/410.md`](../../docs/clarifications/410.md) (Rounds 1-2). Depends
on #403 to #408 (merged) and #409 (first in this batch).

## Problem

Each child ticket of #397 proved its own slice, mostly inside one Go process. Nothing
proves that two real server processes sharing one PostgreSQL database serve as one,
nor that the loss of one of them is survivable. An operator has no readiness probe
that says whether a replica can serve, no clean way to take a replica out during a
rolling deploy, and no single place listing what the deployment must provide.

## User stories

### US1 (P1): CI proves two replicas serve as one

- **Given** two server processes A and B on one PostgreSQL database, behind a balancer,
  and a local agent connected to A,
  **when** an operation that needs the agent is requested through B,
  **then** it reaches the agent through A and its answer comes back through B.
- **Given** a browser event stream open on B,
  **when** a task changes through A,
  **then** the stream on B receives the change.
- **Given** an MCP session initialized through A,
  **when** its next calls reach B,
  **then** they are served by A's session and succeed.
- **Given** A is killed (SIGKILL) while it holds the agent and an MCP session,
  **then** the agent reconnects to B, an operation through B reaches it again, the
  session's client gets a `404` and initializes a new session that works, and the runs
  A owned are reclaimed by B.

### US2 (P1): a readiness probe distinct from liveness

- **Given** a server whose store answers and, on a shared store, whose instance is
  registered and whose internal listener serves,
  **then** `GET /api/ready` answers 200.
- **Given** any of those is not true, or the server is stopping,
  **then** it answers 503 with the reason, while `/api/health` still answers 200 as long
  as the process serves.

### US3 (P1): a deliberate stop drains the replica

- **Given** a replica receives SIGTERM or SIGINT,
  **then** readiness turns 503 at once, the replica keeps serving for the configured
  grace, then removes its instance row, closes its agent connections and exits;
  **and** the agents reconnect to a peer, which takes over the replica's work without
  waiting for the dead-after bound.

### US4 (P1): operators know what the deployment needs

- **Given** the README and ADRs,
  **then** they list: PostgreSQL as the only multi-replica store, the same
  `SECTILE_SECRET_KEY` on every replica, the internal port and address, the readiness
  and liveness probes, the shutdown grace against the pod's termination grace, a
  disruption budget, the replica count, and the ingress timeouts for WebSocket and SSE.

## Functional requirements

- **FR1**: `GET /api/ready`, public, 200 when ready, 503 with a JSON reason otherwise.
- **FR2**: On SIGTERM/SIGINT: readiness 503, grace (`SECTILE_SHUTDOWN_GRACE`, default
  `5s`), then unregister the instance, close agent connections, shut the listeners
  down, exit 0.
- **FR3**: The liveness bounds of #403 can be shortened through environment variables
  for tests; the defaults are unchanged and documented as not to be tuned in production.
- **FR4**: An integration test, run by the `test:postgres` CI job, drives two real server
  processes and covers US1 and the drain of US3.
- **FR5**: A new ADR records the multi-replica topology; ADR 0016 gets an amendment; the
  README gains an operator section; the CHANGELOG gains a line under `[Unreleased]`.

## Acceptance criteria

- The harness runs in CI and covers: cross-replica agent operation, SSE propagation, MCP
  session across replicas, node kill with agent reconnection.
- Operator documentation lists everything the external deployment needs (replica count,
  readiness probe, disruption budget, ingress timeouts for WebSocket and SSE).

## Out of scope

The change to the external deployment repository (#397 Q4), SQLite multi-instance,
autoscaling, and keeping MCP sessions alive across a stop (#397 Q2).
