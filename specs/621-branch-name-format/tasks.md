# Tasks #621 - Choose the format of the task branch name

Ordered checklist. Each group leaves the tree buildable and is one commit
(Conventional Commits).

## 1. The renderer (FR-3, FR-7, US1, US2)

- [ ] T1.1 `internal/models/branchformat.go`: `DefaultBranchNameFormat`,
  `TaskBranchName`, `ValidateBranchNameFormat`, `ValidBranchName`; extract
  `titleSlug` from `MacroBranchName` and reuse it.
- [ ] T1.2 `internal/models/branchformat_test.go`: the table tests of the plan;
  `MacroBranchName` tests still pass.

## 2. Storage and validation (FR-1 to FR-4, US3)

- [ ] T2.1 Migration 33 `projects.branch_name_format`; column in the project
  `SELECT`s, `INSERT`, `UPDATE`; fields on `Project`, `CreateProjectRequest`,
  `UpdateProjectRequest`.
- [ ] T2.2 `ErrInvalidBranchNameFormat`, validation in `CreateProject` and
  `UpdateProject`, 400 in `repositoryErrorStatus`.
- [ ] T2.3 Migration tests that rebuild an older schema drop the new column.
- [ ] T2.4 Tests: round trip, refused update keeps the stored value, spaces
  stored empty, handler answers 400 with the French message.

## 3. Delivery to the agent and to skills (FR-5, FR-8)

- [ ] T3.1 `agentconfig.Config.BranchNameFormat`; `db.AgentConfig` fills it.
- [ ] T3.2 `get_project_context` returns `branchNameFormat`.
- [ ] T3.3 Tests for both.

## 4. The agent renders the format (FR-6, US2, US4, US5)

- [ ] T4.1 `taskWorktreeBranch(task, format)`, `localTaskPath(..., format)`,
  `ensureLocalWorktree(..., format)`; all callers pass
  `config.BranchNameFormat`.
- [ ] T4.2 Tests: existing branch tests unchanged; `{key}` gives `AUC-1234`;
  an assigned branch wins over a format.

## 5. Server fallback cleanup (T1 of the clarification, US5)

- [ ] T5.1 Runner fallback uses `models.TaskBranchName("", key, title)`;
  test.
- [ ] T5.2 Delete the unreferenced `db.GenerateTaskBranchName`.

## 6. Web settings (FR-9, US2.4)

- [ ] T6.1 `web/src/lib/branchNameFormat.ts` + `web/tests/branchNameFormat.test.ts`.
- [ ] T6.2 `Project.branchNameFormat`; modal input, presets, live example;
  French and English locale strings; payloads.
- [ ] T6.3 `npm test` and `npm run build` in `web/` (restore
  `internal/webui/dist/.gitkeep` after the build).

## 7. Documentation and skills (FR-11, FR-12)

- [ ] T7.1 `CHANGELOG.md` `Added` line; `docs/CAPABILITIES.md`,
  `docs/API_AND_DATA_SPEC.md`, the server-agent contract if it lists config
  fields.
- [ ] T7.2 `clarify/guard.md` wording; regenerate skill goldens and plugin
  test data under Linux.

## 8. Verification

- [ ] T8.1 `go build ./...`, `go vet ./...`, `gofmt -l` on the touched files.
- [ ] T8.2 The targeted Go tests of each touched package (models, db, agent,
  runner, taskmcp, handlers, skills), then the web tests; the PR pipeline runs
  the whole suite.
