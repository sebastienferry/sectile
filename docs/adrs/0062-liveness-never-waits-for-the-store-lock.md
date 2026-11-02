# ADR 0062: Liveness never waits for the store lock

- Status: Accepted
- Date: 2026-10-29

## Context

The store serialises its writers with one process-wide `sync.RWMutex`, `d.mu`.
On SQLite that lock is what keeps a single-writer engine consistent. On
PostgreSQL, several server instances share the database and none of them
shares this lock, so it orders the writers of one process and nothing more.

Two kinds of work depend on that lock without being store work:

- the instance heartbeat, which keeps the instance's `server_instances` row
  fresh so the other instances do not reclaim its runs;
- the agent WebSocket read loop, which has to keep reading or the 45 s read
  deadline drops the agent.

On a deployment with large trackers, a full synchronisation wrote every ticket
under the write lock and held it for tens of seconds. The heartbeat waited
behind it, so the other instance took a live instance for dead, reclaimed its
runs and forgot its agent sessions. The read loop wrote presence and inventory
to the database inline, stopped reading, and every agent was dropped.
`acceptInventory` also held the dispatcher mutex during its database write,
which stalled routing to every agent of the instance (#886).

## Decision

1. **Work whose length depends on the data does not hold `d.mu` from end to
   end.** An import, a reclaim or a sweep whose size follows the number of
   tickets, runs or rows releases the lock at bounded intervals.
   `ImportOrUpdateTasks` releases it every `importChunkSize` tickets.
   Statements that must stay atomic go in a transaction, not under a longer
   lock hold.
2. **No network call runs under `d.mu`.** The post-synchronisation steps
   already read from the tracker first and lock only to write the answer; new
   code follows the same order.
3. **Liveness paths never wait for `d.mu` on a shared database.** The instance
   heartbeat runs on its own goroutine and writes its own row without the
   lock. SQLite keeps the lock, because there it serialises the engine's only
   writer and no peer can take the instance for dead.
4. **A connection's read loop does not write to the database.** It hands
   presence and inventory to a per-connection writer that keeps only the
   latest of each, and goes back to reading. The same goes for any new message
   type whose handling reaches the database.
5. **No database write runs under the dispatcher mutex.** The session is
   checked under the lock, written outside it, and checked again before the
   result is published.
6. **A session another instance forgot is hung up on, not refused forever.**
   The presence writes report `db.ErrWorkstationSessionGone`, and the
   connection is closed with code `4003` so the agent registers again.

## Consequences

A long import no longer stalls the rest of its process, and instances no longer
take each other for dead during a synchronisation. Readers interleave with an
import between chunks, so a list read during a large import can show part of a
synchronisation. Each ticket was already written on its own, so no reader could
ever count on seeing a whole synchronisation at once.

Several imports in one process still take turns on the lock rather than running
side by side, which also keeps their load on PostgreSQL bounded.

Code review has one more rule to hold: a loop over external input, a network
call or a read loop under `d.mu` or the dispatcher mutex is a defect, even when
it is fast on a small dataset.
