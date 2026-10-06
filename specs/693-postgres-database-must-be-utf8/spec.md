# Specification #693 - A PostgreSQL database must be UTF8

- Ticket: https://github.com/sebastienferry/sectile/issues/693
- Branch: `feat/batch-675-690-693` (batch #675, #690, #693)
- Clarification: `docs/clarifications/693.md` (rounds 1 and 2; Q1 refuse,
  Q2 no escape hatch)
- Framework: Spec Kit
- Type: bug

## Summary

A Sectile server refuses to open a PostgreSQL database whose encoding is not
UTF8, with an error that names the encoding found and the fix. The PostgreSQL
tests fail on the same check, naming `SECTILE_TEST_POSTGRES_DSN`, before any
test body runs. The deployment and testing docs say the database must be UTF8.

## Scope

In scope: the store's opening, the three PostgreSQL test entry points
(`internal/db`, `internal/handlers`, `cmd/server`'s multi-replica harness), the
README, `docs/TESTING.md`, the comment in `remoterun.go`, the changelog.

Out of scope: a correct output cap on a non-UTF8 database, converting an
existing database, an override to start anyway (Q2), SQLite, the dev
deployment.

## User stories

### US1 (P1) - An operator is told the database must be UTF8

1. Given `DB_DRIVER=postgres` and a database whose encoding is `SQL_ASCII`,
   when the server starts, then it stops with an error naming `SQL_ASCII`,
   saying the database must be UTF8 and how to create one, and writes nothing
   to the database.
2. Given a database whose encoding is `LATIN1`, then the same happens.
3. Given a database whose encoding is `UTF8`, then the server starts as today.

### US2 (P1) - A developer sees why the PostgreSQL tests cannot run

1. Given `SECTILE_TEST_POSTGRES_DSN` names a `SQL_ASCII` database, when a
   PostgreSQL test of `internal/db` or `internal/handlers` opens it, then the
   test fails at once with a message naming `SECTILE_TEST_POSTGRES_DSN` and the
   encoding.
2. Given the same DSN, when the multi-replica harness starts, then it fails on
   the encoding before it builds or starts a replica.

## Functional requirements

- FR1. Opening a PostgreSQL store reads the database's encoding right after
  the connection check and before the schema migration.
- FR2. Any encoding other than `UTF8` (case-insensitive) fails the opening
  with an error naming the encoding found, the requirement, and the fix
  (`CREATE DATABASE <name> ENCODING 'UTF8' TEMPLATE template0`, or a cluster
  made with `initdb --encoding=UTF8`).
- FR3. No environment variable or configuration bypasses FR2.
- FR4. SQLite has no such check.
- FR5. The `internal/db` and `internal/handlers` test helpers fail with a
  message naming `SECTILE_TEST_POSTGRES_DSN` when the opening fails.
- FR6. The multi-replica harness opens the store once before building the
  server, and fails the same way.
- FR7. The README "PostgreSQL" section and `docs/TESTING.md` state that the
  database must be UTF8 and how to create one.
- FR8. `CHANGELOG.md` gets one `Changed` line under `## [Unreleased]`.

## Acceptance criteria

- [ ] The server refuses to start on a non-UTF8 PostgreSQL database, with an
  explicit error, and the deployment docs say the database must be UTF8.
- [ ] The PostgreSQL test helpers fail fast with an explicit message on a
  non-UTF8 `SECTILE_TEST_POSTGRES_DSN`.
- [ ] The check has a unit test and a PostgreSQL integration test.
- [ ] `TestPostgresConcurrentOutputAppendsTruncateOnce` passes on a UTF8 test
  database.
