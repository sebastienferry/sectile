# Tasks #882 - Triage: add priority

Order matters: each step leaves the tree buildable and `make test` green.
Tests go with the step they cover. One commit per step.

## 1. Sync keeps the local priority

- [ ] T1.1 `internal/db/db.go`: `trackerCarriesPriority(source)`; in the update
      branch of `ImportOrUpdateTasks`, write `priority` only when it returns
      true (FR3, US2.1 to US2.3). Insert branch untouched (FR4, US2.4).
- [ ] T1.2 Confirm `postback.go` ~237 and `UpdateTask` ~3319 do not copy a
      tracker priority; no change expected (plan, Sync).
- Tests: `internal/db/syncpriority_test.go`: GitHub and GitLab keep `high`
  across a re-import carrying `medium`; Jira takes `medium`; a first import
  stores `medium`; other fields still update (US2.5).

## 2. Triage column

- [ ] T2.1 `web/src/locales/planning.ts`: `triage.columns.priority`, French
      and English (FR6).
- [ ] T2.2 `web/src/components/TriageView.tsx`: priority header after
      "Title", per-row dot-select writing through `updateTask`, empty-state
      `colSpan` 9 (FR1, FR2, US1.1 to US1.5).
- Tests: browser case for the triage column (header, dot per row, update sent
  with `{ priority }`, refused update leaves the dot); `i18n.test.mjs` green.

## 3. Finish

- [ ] T3.1 `CHANGELOG.md`: `Added` and `Fixed` lines under `## [Unreleased]`
      (FR7).
- [ ] T3.2 Check no priority is written to GitHub or GitLab (FR5): no change
      in `internal/trackerapi`.
- [ ] T3.3 `make test` green.
