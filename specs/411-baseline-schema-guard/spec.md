# Specification #411 - A guard on the frozen baseline schema

Task: `gh-11a4f59c-b6bd-4747-8ea5-0a10d5b78da4-411`
Clarification: `docs/clarifications/411.md`
Branch: `fix/batch-411-499-517`

## Summary

`projects.enabled_views` was added to the frozen baseline instead of to a
numbered migration, so every database stamped beforehand went without it.
The column itself was fixed by #420 (migration 5). What remains is to make the
same mistake impossible to merge unnoticed: a change to the baseline schema
fails the test suite.

## Scope

In scope: a test that captures the schema the baseline builds and fails on any
difference. Out of scope: the migration runner, ADR 0021, migration 5, and the
PostgreSQL upgrade test, which already covers `lateColumns`.

## User stories (prioritised)

### US1 - A column added to the baseline fails the build (P1)

As a maintainer, I want a baseline edit to fail a test that always runs, so
that the next column goes into a numbered migration instead of reaching only
new databases.

- **Given** the baseline statements are unchanged,
  **when** the `internal/db` tests run,
  **then** the baseline guard passes.
- **Given** a column is added to a baseline `CREATE TABLE`, or a table to the
  baseline,
  **when** the `internal/db` tests run,
  **then** the guard fails and names the table and column that differ, and says
  that the baseline is frozen and the change belongs in a numbered migration
  (ADR 0021).
- **Given** a column's type, NOT NULL constraint or default changes in the
  baseline,
  **when** the tests run,
  **then** the guard fails in the same way.
- **Given** a numbered migration adds a column,
  **when** the tests run,
  **then** the guard still passes: only the baseline is frozen.

## Functional requirements

| ID | Requirement |
| --- | --- |
| FR1 | The guard builds a database with the baseline only, with no numbered migration applied. |
| FR2 | It compares every table with its columns (name, declared type, NOT NULL, default) against a snapshot kept in the repository. |
| FR3 | A difference fails with the differing lines and a message pointing to ADR 0021 and to numbered migrations. |
| FR4 | The guard runs without any external service (SQLite). |
| FR5 | No production behaviour changes. |

## Success criteria

- `go test ./internal/db/ -run Baseline` passes on the branch.
- Adding a throwaway column to a baseline `CREATE TABLE` makes it fail with the message of FR3 (checked by hand, then reverted).

## Open questions

None.
