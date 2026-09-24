# #404: Auto-sync runs once per project across replicas

Parent macro: #397. Clarification: [`docs/clarifications/404.md`](../../docs/clarifications/404.md).

## Problem

Each server process runs its own background synchronisation loop and keeps its pacing
in memory. With N processes on one database, every project is synchronised N times per
interval, gets N full reads, and a tracker asking to slow down (429) is only heard by
the process that received the answer.

## User stories

### US1 (P1): one synchronisation per interval, whatever the number of replicas

- **Given** two server processes share a database and a project has auto-sync on,
  **when** both run a pass while the project is due,
  **then** exactly one synchronisation is queued for it.
- **Given** a project was claimed less than its interval ago by any process,
  **when** another process runs a pass,
  **then** nothing is queued for it.

### US2 (P1): one full read per period

- **Given** one process recorded a successful full read of a project less than 30
  minutes ago,
  **when** another process queues the next synchronisation,
  **then** that synchronisation is incremental, bounded on the previous pass plus the
  overlap.
- **Given** a full read failed, **then** it is not dated, whichever process ran it.

### US3 (P1): a tracker asking to slow down is heard by every replica

- **Given** one process received a rate-limit answer,
  **when** any process runs its loop in the next ten minutes,
  **then** it queues nothing.

### US4 (P2): one status for the whole deployment

- **Given** several processes, **when** `/api/sync/auto` is asked of any of them,
  **then** last run, last error, last imported count, totals and backoff are the same.

## Functional requirements

- **FR1** Per-project pacing (last pass, last successful full read) is stored in the
  database.
- **FR2** A process queues a project's synchronisation only after atomically claiming
  it: the claim succeeds for one process when the project is due, and fails for every
  other.
- **FR3** The incremental window is computed from the pacing read before the claim,
  with today's rules (full on first pass, every 30 minutes, after a failed full read,
  or beyond 24 hours).
- **FR4** The rate-limit backoff (ten minutes, global) is stored in the database and
  honoured by every process.
- **FR5** Last run, last error, last imported count and the pass and import totals are
  stored in the database; "running" stays per process.
- **FR6** Existing behaviour for a single process is unchanged.

## Out of scope

Deduplicating a synchronisation that outlives its interval; per-tracker backoff.

## Success criteria

Existing auto-sync tests pass, rewritten only where they set in-memory state; new tests
prove the claim with two stores on one database file, the shared backoff and the
shared full-read dating.
