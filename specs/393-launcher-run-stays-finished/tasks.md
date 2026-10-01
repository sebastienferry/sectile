# Tasks #393 - A finished launcher-owned run stays finished

- [ ] T1 DB test of the ticket's sequence: running and queued reports leave
  the run completed (US1.1, US1.2).
- [ ] T2 DB test: second `finish_run` same status accepted, other refused
  (US1.3).
- [ ] T3 DB test: dead-instance reclaim and `start_run(runId)` leave it
  unchanged (US2).
- [ ] T4 Handler test through `ApplyAgentRunningTasksFor` (US1.4).
- [ ] T5 Check T1 against the pre-#407 statement locally (FR3), restore.
- [ ] T6 Run the tests on SQLite and PostgreSQL.
