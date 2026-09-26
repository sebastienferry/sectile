# Tasks #411 - A guard on the frozen baseline schema

Ordered checklist. One commit, leaving the tree buildable.

## 1. Baseline-only opening (FR1, FR5)

- [x] T1.1 Test helper `openBaselineOnly` in `baseline_guard_test.go`, assembling the store without `migrateSchema` (plan decision 1; no production change).

## 2. Snapshot and guard (FR2-FR4)

- [x] T2.1 `baselineSchema(d)` reads every table and `PRAGMA table_info` into sorted lines.
- [x] T2.2 Generate `testdata/baseline_schema.txt` once from `origin/main`'s baseline.
- [x] T2.3 `TestTheBaselineSchemaIsFrozen` diffs the two and fails with the FR3 message.

## 3. Verification

- [x] T3.1 `go test ./internal/db/ -run 'Baseline|Migration|Stamped' -count=1`.
- [x] T3.2 By hand: add a column to a baseline `CREATE TABLE`, see the guard fail, revert.
- [ ] T3.3 `go vet ./internal/db/` and the full `go test ./internal/db/`.
