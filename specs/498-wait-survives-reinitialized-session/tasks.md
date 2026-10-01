# Tasks #498 - A launched run's next call ends its wait after a re-initialization

- [x] T1 `db.ResumeRunWait` with its guards; DB tests for match, other run,
  other owner, hand-set wait, calling session (FR2).
- [x] T2 `SessionRegistry.ResumeRun` and the middleware reading the header
  (FR2, FR3).
- [x] T3 Bridge transport adding the header from `SECTILE_RUN_ID` (FR1, FR4);
  unit test with and without the variable.
- [x] T4 Handler test: declare a wait in S1, call `get_task` from S2 with the
  header, wait ended; without the header or with another run's id, kept
  (US1, US2).
- [x] T5 Update the #475 residual and add the changelog line (AC2, AC3).
