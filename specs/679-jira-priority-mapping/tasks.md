# Tasks #679 - Store and correct the Jira ticket priority mapping

Ordered checklist, one pull request from `feat/679`. Spec: `spec.md`; design:
`plan.md`. Tests are written before the code they cover.

## 1. Model

- [x] 1.1 `PriorityMapping`, `PriorityMappingOption`, `PriorityOption` in
  `internal/models`; `Project.PriorityMapping`,
  `UpdateProjectRequest.PriorityMapping`.
- [x] 1.2 Table tests, then `internal/models/prioritymapping.go`: `Empty`,
  `Writable`, `WritableLevels`, `OptionFor`, `MergePriorityMapping` (FR-2,
  FR-3, FR-5).

## 2. Storage

- [x] 2.1 Check the next free migration number on `origin/main`; add
  `projects.priority_mapping` in `migrations.go`; carry it in the project
  statements.
- [x] 2.2 Drop the column in every rewind helper that drops
  `epic_axis_prefixes`; update the replay fixtures; run `internal/db` on
  SQLite and PostgreSQL.
- [x] 2.3 Validation of a mapping sent with a project update (FR-4): unknown
  ids, bad level, manual flag set on changed lines. Tests.

## 3. Jira adapter

- [x] 3.1 `tracker.PrioritySchemeReader`; `JiraAdapter.PriorityScheme` (create
  screen, else site list, `fresh` bypassing the caches);
  `ClassifyJiraPriority`. Tests on Atlassian default, Server, French,
  `P1`..`P4`.
- [x] 3.2 `priorityFieldFor` with the mapping; `ErrGuessedPriority`;
  `UpdateIssue` returns it (FR-8); `CreateIssue` and `setPriorityAfterCreate`
  drop the field and set `PriorityNotice` (FR-7); empty mapping unchanged
  (FR-10). Tests.

## 4. Discovery

- [x] 4.1 `db.RefreshPriorityMapping`; tests with a fake reader (US1).
- [x] 4.2 Sync step under `describesProject`; test full against background
  sync (US1.6).
- [x] 4.3 `POST /api/projects/{id}/priority-mapping/refresh`; handler test.

## 5. Refusal

- [x] 5.1 `db.CheckPriorityWritable` and its call at the top of
  `UpdateTaskBy`. Tests: row unchanged, no activity queued, other fields not
  written, unchanged priority accepted, non-Jira and empty mapping untouched
  (US3).
- [x] 5.2 Handler 422; MCP `update_task` error. Tests.
- [x] 5.3 `priorityNotice` in MCP `create_task`'s answer. Test (US4).

## 6. Web

- [x] 6.1 Types: `priorityMapping` on `Project`, `priorityNotice` on `Task`.
- [x] 6.2 The mapping table in the Tracker tab of `ProjectModal.tsx`: levels,
  guessed badge, confirm, preferred option, non-writable levels, refresh.
  French and English labels in `projectSettings.ts` (US2).
- [x] 6.3 `handleBulkPriority` continues past a refused ticket and reports the
  refused ones through a pure helper with a `node --test` (US3.3).
- [x] 6.4 Creation toasts show `priorityNotice` (US4.1).

## 7. Docs and changelog

- [x] 7.1 `CHANGELOG.md`, `Unreleased`: the `Added` and `Changed` lines of
  `plan.md`.
- [x] 7.2 Tick part B of `specs/635-epic-axes-tracker-fields/tasks.md` as
  carried by #679, or note there that #679 supersedes it.

## 8. Gates

- [x] 8.1 `go test ./...`, `go vet ./...`, `cd web && npx tsc --noEmit`,
  `npx oxlint`, `node --test web/tests`.
- [ ] 8.2 Open the pull request at the implemented stage.
