# ADR 0030: Several server replicas share one PostgreSQL database

Status: Accepted. Amended by
[ADR 0032](0032-unlocked-sealed-credentials-live-with-their-owners-presence.md)
for the keys derived from sealing passphrases (#501).

Amends: [ADR 0016](0016-postgresql-as-an-alternative-store.md), its statement
that PostgreSQL backs one server instance, not several.

## Context

Sectile's server ran as one process. ADR 0016 called several instances on one
database unsafe: a booting process reclaimed every other process's work, the
queue, the cancellations, the live updates, the agent connections and the MCP
sessions all lived in one process's memory. The owner expects load to grow, so
macro #397 made the server able to run as several active replicas behind a load
balancer. Its clarification is in `docs/clarifications/397.md` (on `feat/397`),
and each child ticket has its own under `docs/clarifications/40x.md`.

## Decision

**Active/active on PostgreSQL.** Every replica serves every request. SQLite stays
a single-instance store: a file is not shared between pods.

**PostgreSQL is the only shared infrastructure.** No Redis, no session store, no
sticky sessions. What the replicas must share goes through the database, and
what cannot (a WebSocket, an MCP session) is reached through the
replica that holds it, on an internal port:

| Concern | Mechanism | Ticket |
| --- | --- | --- |
| Which replicas are alive, and whose work is whose | `server_instances` heartbeat; activities carry their `instance_id`; only the work of a replica silent past the bound is reclaimed | #403 |
| Tracker synchronisation | per-project claim, persisted pacing and backoff | #404 |
| Live updates and cancellations | `LISTEN/NOTIFY` on `sectile_events` | #405 |
| Local agents | `agent_presence`; work forwarded to the replica holding the agent | #406 |
| Read-check-write sequences, per-project limit | conditional updates and row locks instead of the process mutex | #407 |
| MCP sessions | the session id names its replica; requests forwarded to it | #408 |
| Keys derived from sealing passphrases | stored in the database, sealed under the server key, while their owner is present (ADR 0032, which replaced the in-memory relay of #409) | #501 |
| Probes, drain, end-to-end proof | `/api/ready`, SIGTERM drain, a two-process harness in CI | #410 |

**The replicas authenticate each other with a token derived from
`SECTILE_SECRET_KEY`**, which they already share. No second secret is
distributed.

**Liveness and readiness are separate.** `/api/health` says the process serves
HTTP. `/api/ready` says a balancer may send it traffic: the database answers,
the replica is registered, its internal listener serves, and it is not stopping.

**A stop drains.** On SIGTERM a replica turns not ready, keeps serving for a
grace, removes its instance row, closes its agent connections and exits, so its
peers take over at once. A replica that dies without draining is taken over
once its silence passes the dead-after bound (45 s).

## Consequences

- What a lost replica loses is settled (#397 Q2): an agent operation in flight
  through it fails with an explicit error and is not replayed; its MCP sessions
  end, their clients start new ones, and the runs they owned are canceled but
  stay recoverable through `finish_run`; its agents reconnect elsewhere.
- The deployment has to provide what the README's "Several replicas" section
  lists: the shared key, the internal port unrouted by the ingress, both
  probes, a termination grace longer than `SECTILE_SHUTDOWN_GRACE`, a
  disruption budget, and ingress timeouts suited to WebSocket and SSE. That
  deployment lives outside this repository.
- The scale assumption is a handful of replicas and hundreds of agents.
  `LISTEN/NOTIFY` and pod-to-pod forwarding are sized for that; an order of
  magnitude more would call for revisiting both.
- CI proves the whole in `TestPostgresMultiReplicaHarness`, which runs two real
  server processes on the `test:postgres` job's database, kills one and drains
  the other.

## Alternatives rejected

- **Active/passive with a leader lock.** Recommended in Round 1 of #397 and
  rejected by the owner: it survives a node loss but does not scale.
- **Balancer affinity** on a cookie or on `Mcp-Session-Id`. It depends on the
  balancer, and cannot help agent routing: the replica a user's request reaches
  and the replica the agent's WebSocket reached are unrelated.
- **Redis or another message broker.** A second stateful dependency for what
  PostgreSQL already does at this scale.
- **Persisting derived keys under the server key.** Rejected by #409 because it
  would reduce a sealing passphrase to the server key it exists to go beyond,
  then adopted by #501 (ADR 0032): the in-memory relay lost every key when all
  replicas restarted at once, which every redeploy does. The exposure is bounded
  by the owner's presence, see ADR 0032.
