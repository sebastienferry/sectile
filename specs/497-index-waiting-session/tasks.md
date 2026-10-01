# Tasks #497 - Index task_activities.waiting_session

- [x] T1 Migration 39 with the partial index (FR1, FR2).
- [x] T2 `ResumeWaits` query as a constant with `waiting_session <> ''` (FR3).
- [x] T3 Plan test on both engines (AC3); migration test that an upgraded
  database has the index and keeps its waits (US1.2).
- [x] T4 Run `go test ./internal/db/...` on SQLite and PostgreSQL (AC2).
