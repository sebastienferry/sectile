# ADR 0041: SQLite behavior tests copy a migrated template

- Status: Accepted
- Date: 2026-09-28
- Issue: [#595](https://github.com/sebastienferry/sectile/issues/595)

## Context

A successful Go CI job spent 875.9 seconds executing `internal/db`. A local
profile of a small instrumented behavior test attributed 73.9% of sampled CPU
time to schema migration. Hundreds of unrelated behavior tests rebuild the same
schema. Race instrumentation also applies to the pure-Go SQLite driver.

Increasing timeouts does not shorten feedback. A shared development PostgreSQL
database would introduce destructive interference and would not verify SQLite.
Sharing a mutable SQLite database or transaction between tests would change their
isolation and the behavior of asynchronous workers.

## Decision

Ordinary behavior tests opt into `internal/testsqlite.New`. Once per process,
it builds a database through `NewDB`, closes it, verifies that no nonempty WAL
remains, and retains the resulting file bytes. Each test receives its own file
in its own directory and opens it through `NewDB` again. Only bytes are reused;
connections, keys, queues and runtime identities are constructed normally.

The helper has no dependency on `internal/db`, so database package tests and
external consumers use the same mechanism without an import cycle. Its
constructor parameter must always identify the same database implementation
within a process. The template is never persisted between test executions.

Migration, first-start, seeding, persistence, recovery and multi-instance tests
retain direct constructors. PostgreSQL integration coverage remains unchanged.
Existing destinations are rejected rather than reset. Dedicated tests compare
schema objects and integrity and check isolation and concurrent copies.

## Consequences

Most behavior tests avoid replaying migrations while still exercising normal
startup against a current schema. One template initialization remains in every
test process. The helper is opt-in, and future lifecycle tests must not use it.

Fixtures inherit the initial database's seed data and migration timestamps;
tests that depend on fresh initialization timing must use a direct constructor.
The helper does not make environment-mutating tests parallel-safe or change the
production database lifecycle, durability, race checks or coverage settings.

See [database testing](../TESTING.md) for usage and timing commands.
