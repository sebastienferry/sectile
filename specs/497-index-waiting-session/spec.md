# Specification #497 - Index task_activities.waiting_session

- Ticket: https://github.com/sebastienferry/sectile/issues/497
- Branch: `feat/batch-393-497-498-584-647-648`
- Clarification: `docs/clarifications/497.md` (round 1, no open question)
- Framework: Spec Kit

## Summary

The lookup every Sectile tool call runs to end its session's waits reads an
index instead of scanning every activity.

## Scope

In scope: one numbered migration, the lookup's query, and tests on both
engines. Out of scope: the baseline, other activity queries, and when a wait
ends.

## User stories

### US1 (P1) - The per-call wait lookup uses an index

1. Given a database migrated to the new version, when the wait lookup runs
   for a session, then its plan uses `idx_task_activities_waiting_session`,
   on SQLite and on PostgreSQL.
2. Given a database at the previous version with waiting and non-waiting
   runs, when it is migrated, then the index exists and every run keeps its
   waiting state.
3. Given a session that declared a wait, when the session makes a call, then
   the wait ends exactly as before.

## Functional requirements

- FR1. A new numbered migration creates a partial index on
  `task_activities (waiting_session)` for non-empty values.
- FR2. The frozen baseline is not edited.
- FR3. The lookup states the index predicate so both planners can use it.

## Acceptance criteria

- AC1. The migration creates the index on SQLite and PostgreSQL.
- AC2. The migration and rewind tests pass on both engines.
- AC3. A test reads the lookup's plan and finds the index.

## Open points

None.
