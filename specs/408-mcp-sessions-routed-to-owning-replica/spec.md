# #408: MCP sessions routed to their owning replica

Parent macro: #397 (child ticket 5, D10, FEAT-4 / US-4). Clarification:
[`docs/clarifications/408.md`](../../docs/clarifications/408.md). Depends on #403 and
#406, both merged.

## Problem

The MCP endpoint is stateful (ADR 0007): a session lives in the memory of the server
instance that created it. When several instances share one PostgreSQL database behind
a load balancer, a request carrying an `Mcp-Session-Id` can reach an instance that
does not hold the session, and fails. A run lost with a dead instance, or with a
restarted process, is closed for good although ADR 0007 calls it recoverable. The
sessions view only shows the sessions of the instance that answered it.

## User stories

### US1 (P1): one session, whichever replica receives the request

- **Given** an MCP client initialized its session on instance A,
  **when** a later `POST`, the `GET` event stream or the closing `DELETE` of that
  session reaches instance B,
  **then** B has the request served by A, the client receives A's answer unchanged
  (status, headers, body, streamed events as they come), and the session keeps working.
- **Given** a run started through that session, whichever instance carried the
  `start_run` call,
  **then** the run belongs to the session on A: it is adopted there, released when it
  finishes, and closed by A if the session ends while it is still running.
- **Given** an `initialize` request (no session yet),
  **then** the instance that receives it creates and holds the session.

### US2 (P1): a dead owner ends its sessions cleanly

- **Given** instance A holding a session is no longer live (unseen for the liveness
  bound of #403),
  **when** the client's next request reaches instance B,
  **then** B answers `404` (session not found), the client re-initializes on B, and
  its new session works.
- **Given** the runs that session had started and were still running,
  **then** they are closed as `canceled` within the liveness bound, whichever surviving
  instance notices first, and each is closed once.
- **Given** instance A is still listed as live but cannot be reached,
  **when** a request for its session reaches B,
  **then** B answers `503` with a reason naming the unreachable instance, and does not
  retry the request.

### US3 (P1): a run lost with its server stays recoverable

- **Given** a client run canceled because its instance died,
  **or** because the single server process restarted,
  **when** the run's owner reports its real outcome through `finish_run`,
  **then** the run takes that outcome, exactly as after an MCP disconnection.
- **Given** such a run, **when** nobody reports on it, **then** it stays `canceled`.
- **Given** a run canceled by a person (from the board or the activities view),
  **then** it stays final, as today.

### US4 (P2): one sessions view for the deployment

- **Given** sessions are held by several live instances,
  **when** any instance is asked for `GET /api/mcp/sessions`,
  **then** the answer lists the sessions of every live instance, each naming the
  instance that holds it.
- **Given** a live instance does not answer within 2 seconds,
  **then** the answer still lists the sessions of the others, and names the instance
  that did not answer.
- The web sessions view looks and behaves as today.

### US5 (P1): forwarding never widens access

- **Given** a forwarded MCP request,
  **then** the owning instance accepts it only when it carries both the internal
  credential of the deployment and a client credential the public endpoint would have
  accepted; the client's identity is the one that owns what the call creates.
- **Given** the internal MCP endpoints,
  **then** they are served only on the internal listener of #406, never on the public
  port.

### US6 (P1): nothing changes for a single instance

- **Given** SQLite, or PostgreSQL with one instance,
  **then** MCP sessions, the sessions view and run recovery behave as today, apart
  from the format of the session id and the `instance` field in the sessions view.

## Functional requirements

- **FR1** Every session id an instance creates names that instance, on every storage
  engine.
- **FR2** A request on `/mcp` whose session id names another live instance is served
  by that instance, through the internal listener, with the method, headers, body and
  streamed response carried unchanged.
- **FR3** A forwarded request is never forwarded again. A request without a session id,
  or whose session id names this instance or no instance, is served locally.
- **FR4** A session id naming an instance that is not live gets the local answer, which
  is `404` for a session this instance does not hold.
- **FR5** A session id naming a live instance that cannot be reached gets `503` with a
  reason, and is not retried.
- **FR6** The owning instance checks both the internal credential and the client's own
  credential on a forwarded MCP request.
- **FR7** Client runs canceled by the reclaim of a dead instance, or by the restart of a
  single-process engine, can be rewritten by their owner through `finish_run`, as runs
  canceled on a disconnection can. Other cancellations stay final.
- **FR8** `GET /api/mcp/sessions` answers with the sessions of every live instance that
  answered within 2 seconds, each carrying the id of its instance, plus the ids of the
  live instances that did not answer. It never fails because one instance did not
  answer.
- **FR9** Without a server key, or on an engine that serves one process, no MCP request
  is forwarded and the sessions view is local; the reason is logged at start, as #406
  does for agents.
- **FR10** ADR 0007 records that a session is owned by one instance, found through its
  id, and dies with it.
- **FR11** `CHANGELOG.md` gains, under `[Unreleased]`, the user-visible part: an MCP
  client keeps its session across replicas, and a run lost with a server can still be
  reported on through `finish_run`.

## Out of scope

- Moving a session to another instance: a session dies with its instance (#397 Q2).
- Stateless MCP, balancer affinity, balancer configuration and the external deployment
  change (#397 Q4).
- Showing the instance, or the unreachable instances, in the web sessions view.
- The multi-replica integration harness (macro ticket 8).
- Any schema migration.

## Open points

- **A run closed from the board on one instance while its session lives on another.**
  The run is closed correctly, but the owning instance keeps listing it under the
  session in the sessions view until the session ends (closing it again is a no-op).
  The clarification did not address it; it is not part of this ticket's criteria and
  blocks nothing. To be decided by the owner: accept, or open a follow-up.
- **Rolling upgrade.** A session created by an instance running an earlier version has
  an id that names no instance; a newer instance that receives it answers `404` and the
  client re-initializes. This follows FR3/FR4 and needs no decision; it is noted for
  the release notes of #397.

## Success criteria

- Two instances on one PostgreSQL test database: a client alternating between them
  initializes, calls tools, starts and finishes a run, holds the `GET` stream and ends
  with `DELETE`, all on one session held by the first instance.
- The owner stopped and declared dead: the next request gets `404`, re-initialization
  works on the survivor, the old running runs end `canceled`, and `finish_run` by their
  owner rewrites them. The same recovery after a single-process restart on SQLite.
- The owner live but unreachable: `503` with a reason, one attempt.
- `/api/mcp/sessions` on either instance lists both instances' sessions, and lists one
  unreachable instance without failing.
- A forwarded request with a wrong internal credential, or without a valid client
  credential, is refused by the owner.
- The existing MCP and sessions tests pass unchanged apart from the id format.
