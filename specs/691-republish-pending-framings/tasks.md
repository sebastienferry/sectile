# Tasks #691 - Roadmap: republish every pending framing copy

Order matters: each step leaves the tree buildable. Tests go with the step
they cover.

## 1. Listing

- [ ] T1.1 `pendingMacroCopies` and `PendingFramingCopies` in
      `internal/db/macrotodosmirror.go`.
- Tests (`internal/db/macroframingmirror_test.go`): US1.1 to US1.4 (never
  copied and framed, copied and unchanged, edited since, empty and never
  copied counted as skipped, `M-<n>` and roadmap project epics excluded,
  non-Jira project empty).

## 2. Batch op

- [ ] T2.1 `TrackerOpEpicFramingBulk`, `TrackerOp.EpicKeys`, its activity
      texts, its dispatch and `runEpicFramingBulkOp`.
- [ ] T2.2 `PushPendingFramingCopies`.
- Tests: US3.1 to US3.7 (sequential writes signed by the starter, already up
  to date makes no call, partial failure stored on the macro and the activity
  `completed`, all failed ends `failed`, missing token named, an epic made
  ineligible fails alone, no comment for a skipped epic).

## 3. Route

- [ ] T3.1 GET and POST `/api/projects/{id}/macros|epics/framing-mirror`.
- Tests (`internal/handlers/macroframingmirror_test.go`): US2.1, US2.3 to
  US2.5, 405 on another method, the per-macro route still answering.

## 4. Web

- [ ] T4.1 Context actions and strings.
- [ ] T4.2 Roadmap button.

## 5. Documentation

- [ ] T5.1 `docs/API_AND_DATA_SPEC.md`.
- [ ] T5.2 `CHANGELOG.md`.

## 6. Checks

- [ ] T6.1 `go build ./...`, `go vet ./...`, Go tests of `internal/db` and
      `internal/handlers`, web `tsc` and `oxlint`, web unit tests.
