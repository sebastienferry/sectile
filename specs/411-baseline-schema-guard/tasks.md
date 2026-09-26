# Tasks #411 - A guard on the frozen baseline schema

Ordered checklist. One commit, leaving the tree buildable.

## 1. Baseline-only opening (FR1, FR5)

- [ ] T1.1 `DB.migrations` field and `migrationList()`; `migrateSchema` uses it.
- [ ] T1.2 Test helper `openBaselineOnly` in `baseline_guard_test.go`.

## 2. Snapshot and guard (FR2-FR4)

- [ ] T2.1 `baselineSchema(d)` reads every table and `PRAGMA table_info` into sorted lines.
- [ ] T2.2 Generate `testdata/baseline_schema.txt` once from `origin/main`'s baseline.
- [ ] T2.3 `TestTheBaselineSchemaIsFrozen` diffs the two and fails with the FR3 message.

## 3. Verification

- [ ] T3.1 `go test ./internal/db/ -run 'Baseline|Migration|Stamped' -count=1`.
- [ ] T3.2 By hand: add a column to a baseline `CREATE TABLE`, see the guard fail, revert.
- [ ] T3.3 `go vet ./internal/db/` and the full `go test ./internal/db/`.
