# #403: Server instance identity and scoped restart sweep

Parent macro: #397 (Multi-replica server). Clarification: [`docs/clarifications/403.md`](../../docs/clarifications/403.md).

## Problem

Every start of `sectile-server` marks all `running`, `queued` and `pending` work in the
database as interrupted. With one process that is right: whatever was running died
with it. With two processes on one PostgreSQL database it is wrong: starting the
second one, or a rolling deploy that starts the new pod before stopping the old one,
fails the jobs and cancels the client-owned runs of the process that is still alive.

## User stories

### US1 (P1): a second replica does not destroy the live replica's work

As an operator running Sectile on PostgreSQL, I start another server process against
the same database, and the work the first process is executing keeps running.

- **Given** replica A is alive and executes a server job and holds a client-owned run,
  **when** replica B starts on the same PostgreSQL database,
  **then** the job and the run are still `running`.

### US2 (P1): the work of a dead replica is reclaimed

As an operator, when a replica disappears without warning, its unfinished work does
not stay active forever.

- **Given** replica A owned a running server job and a client-owned run, and has not
  been seen for longer than the liveness bound,
  **when** any surviving replica runs its periodic check, or any replica starts,
  **then** the server job is `failed` with "Interrupted by server restart" and the
  client-owned run is `canceled` with the existing restart summary.
- **Given** replica A owned an agent-dispatched run,
  **when** A is declared dead,
  **then** that run stays `running` (its agent reports the real outcome, ADR 0006).

### US3 (P1): a single SQLite server behaves as today

- **Given** a SQLite server stopped while work was running,
  **when** it starts again,
  **then** that work is reclaimed at once, exactly as before this change.

### US4 (P2): tools that open the store are not replicas

- **Given** `sectile-migrate` or a test opens the store,
  **then** no instance row is registered and no heartbeat runs.

## Functional requirements

- **FR1** Each process that opens the store has an instance id, random per start.
- **FR2** A serving process registers its instance in the database (id, hostname,
  PID, start time, last seen) and refreshes "last seen" periodically. If its row has
  disappeared, it registers again.
- **FR3** Every activity the process creates, and every job it starts executing,
  records that process's instance id as owner.
- **FR4** On PostgreSQL, opening the store reclaims only work whose owner is empty
  (written by a previous version) or is not a live instance, and removes dead
  instance rows. On SQLite it reclaims all unfinished work and removes every instance
  row, as SQLite serves one process by construction.
- **FR5** A serving process periodically reclaims the work of dead instances with the
  same rule as FR4 on PostgreSQL.
- **FR6** Reclaiming keeps the current outcomes: server work becomes `failed`
  ("Interrupted by server restart"); a client-owned remote run becomes `canceled` with
  the current summary; an agent-dispatched run is never touched.
- **FR7** An instance is dead when not seen for 45 seconds. Heartbeat every 10 seconds,
  reclaim pass every 15 seconds.

## Out of scope

Detecting two processes on one SQLite file (owner decision, clarification Round 2);
graceful shutdown and readiness (#410); agent, MCP and event routing (#405, #406, #408);
auto-sync coordination (#404).

## Success criteria

- The three existing restart tests pass unchanged.
- A PostgreSQL test proves a live instance's work survives another process opening the
  store, and a dead instance's work is reclaimed.
- The reclaim rule is tested on the default engine without PostgreSQL.
