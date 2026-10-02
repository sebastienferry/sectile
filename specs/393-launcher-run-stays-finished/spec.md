# Specification #393 - A finished launcher-owned run stays finished

- Ticket: https://github.com/sebastienferry/sectile/issues/393
- Branch: `feat/batch-393-497-498-584-647-648`
- Clarification: `docs/clarifications/393.md` (rounds 1 and 2, every
  recommendation accepted by the owner)
- Framework: Spec Kit

## Summary

A run the board launched on the agent, adopted by the skill session and
closed with `finish_run`, stays in its final state whatever the agent or the
server does afterwards. The reopening path of the ticket was closed by #407;
this change proves it with the ticket's own sequence and checks the other
paths that run at a restart or a reconnection.

## Scope

In scope: regression tests for the ticket's sequence, and a bounded
reproduction, in tests, of every path that runs at a server restart or an
agent reconnection.

Out of scope: the agent's own view of a run after `finish_run` (the agent
keeps it until its console exits), the closure of the two stale runs on the
live server (a manual step for an admin), and runs that were never finished.

## User stories

### US1 (P1) - A finished launcher run is never reopened by an agent report

As the owner of a run, once `finish_run` answered `completed`, the board never
shows the run as running again.

1. Given a run created by the launcher and adopted by `start_run(runId)`,
   when `finish_run(completed)` closes it and the agent then reports it
   `running`, then the run stays `completed`, with its `completedAt` and its
   summary.
2. Given the same finished run, when the agent reports it `queued`, then it
   stays `completed`.
3. Given the same finished run, when `finish_run` is called again with
   `completed`, then it is accepted and changes nothing; with `failed`, it is
   refused.
4. Given the same finished run, when the agent's running list is applied by
   the handler that writes "Execution running on agent", then the run stays
   `completed`.

### US2 (P2) - Restart and reconnection paths leave a finished run alone

1. Given a finished launcher run, when the server reclaims the runs of dead
   instances, then the run is unchanged.
2. Given a finished launcher run, when a session calls `start_run` with its
   runId, then the call is refused as "already ended" and the run is
   unchanged.

## Functional requirements

- FR1. No agent report moves a `completed`, `failed` or `canceled` run back
  to `queued` or `running`.
- FR2. The tests run on SQLite and PostgreSQL.
- FR3. The test of US1.1 fails on the statement `SyncRemoteRunStatusFor` had
  before #407 (checked once by hand during implementation).

## Acceptance criteria

- AC1. US1 and US2 are covered by tests that pass on both engines.
- AC2. The stale runs `e07e72c2-dbaf-423b-9e83-051a97372164` and
  `b25f6ca9-941a-4dd5-b2b6-e52ebf3065d6` are checked on the live server and
  closed by an admin if still running (owner step, no code).

## Open points

- A row deleted from the activities view while its console is still open is
  recreated by the next agent report, as `running` with a fresh `createdAt`
  (the insert branch of `SyncRemoteRunStatusFor`). This matches the shape of
  the 2026-09-25 observation but is not a reopening of a stored row; fixing it
  needs either a record of deleted runs or the agent-side view the owner kept
  out of scope. It is reported to the owner for a separate ticket.
