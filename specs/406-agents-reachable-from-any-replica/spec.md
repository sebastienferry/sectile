# #406: Agents reachable from any replica

Parent macro: #397. Clarification: [`docs/clarifications/406.md`](../../docs/clarifications/406.md).

## Problem

A local agent keeps one WebSocket to whichever server instance the load balancer gave
it. Any request reaching another instance, a stage transition that checks git evidence,
a launch, a workspace operation, fails with "no local agent connected".

## User stories

### US1 (P1): agent work succeeds whichever instance receives the request

- **Given** an agent is connected to instance A,
  **when** instance B needs it (operation, launch, dispatch, task pull),
  **then** B forwards the work to A, A runs it on the agent, and B returns A's result.

### US2 (P1): an agent that moved is found again

- **Given** instance A holding an agent stops, **when** the agent reconnects to instance
  B, **then** later work through any instance reaches it on B, and work arriving during
  the reconnection waits for it as it does today.
- **Given** a forwarded operation is in flight, **when** the instance holding the agent
  dies before answering, **then** the caller receives an explicit error; nothing is
  replayed.

### US3 (P1): one connection per agent slot across instances

- **Given** an agent slot is held by instance A, **when** the same user and project
  connect through instance B, **then** A closes its connection with "Session Rebound".

### US4 (P2): one agent status for the deployment

- **Given** agents are connected to several instances, **when** any instance is asked for
  the agent status, **then** it lists all of them.

### US5 (P1): the internal surface is private

- **Given** the internal endpoints, **then** they listen on a dedicated port, not the
  public one, and refuse any request without the bearer derived from the shared server
  key.

## Functional requirements

- **FR1** Each instance records in the database which agent slots it holds, and when an
  agent left.
- **FR2** An instance that does not hold an agent forwards operations, dispatches,
  confirmed launches and task pulls to the instance that does.
- **FR3** Waiting for a reconnecting agent considers agents connected to any instance.
- **FR4** A new connection for a slot closes the previous one, whichever instance holds
  it.
- **FR5** The agent status lists the agents of every live instance.
- **FR6** Internal endpoints listen on `SECTILE_INTERNAL_PORT` (8092 by default), only
  when the store can be shared, and require the bearer; without a server key forwarding
  is refused with an explicit error.
- **FR7** An instance advertises `SECTILE_INTERNAL_URL`, or its first non-loopback IPv4
  with the internal port.
- **FR8** Agents of dead instances are ignored and their records removed by the reaper.
- **FR9** The web agent indicator refreshes on agent connection and disconnection events.

## Out of scope

MCP session routing (#408), passphrase keys (#409), agent-side changes.

## Success criteria

Two dispatchers with a shared directory: an operation, a dispatch, a confirmed launch and
a pull through B reach the agent on A; a rebind through B closes A's connection; a dead
owner fails explicitly; unauthenticated internal calls are refused. Presence SQL tested on
both engines.
