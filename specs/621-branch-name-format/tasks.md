# Tasks #621 - Choose the format of the task branch name

Ordered checklist. Each group leaves the tree buildable and is one commit
(Conventional Commits).

## 1. The renderer (FR-3, FR-7, US1, US2)

- [x] T1.1 `internal/models/branchformat.go`: `DefaultBranchNameFormat`,
  `TaskBranchName`, `ValidateBranchNameFormat`, `ValidBranchName`; extract
  `titleSlug` from `MacroBranchName` and reuse it.
- [x] T1.2 `internal/models/branchformat_test.go`: the table tests of the plan;
  `MacroBranchName` tests still pass.

## 2. Storage and validation (FR-1 to FR-4, US3)

- [x] T2.1 Migration 33 `projects.branch_name_format`; column in the project
  `SELECT`s, `INSERT`, `UPDATE`; fields on `Project`, `CreateProjectRequest`,
  `UpdateProjectRequest`.
- [x] T2.2 `ErrInvalidBranchNameFormat`, validation in `CreateProject` and
  `UpdateProject`, 400 in `repositoryErrorStatus`.
- [x] T2.3 Migration tests that rebuild an older schema drop the new column.
- [x] T2.4 Tests: round trip, refused update keeps the stored value, spaces
  stored empty, handler answers 400 with the French message.

## 3. Delivery to the agent and to skills (FR-5, FR-8)

- [x] T3.1 `agentconfig.Config.BranchNameFormat`; `db.AgentConfig` fills it.
- [x] T3.2 `get_project_context` returns `branchNameFormat`.
- [x] T3.3 Tests for both.

## 4. The agent renders the format (FR-6, US2, US4, US5)

- [x] T4.1 `taskWorktreeBranch(task, format)`, `localTaskPath(..., format)`,
  `ensureLocalWorktree(..., format)`; all callers pass
  `config.BranchNameFormat`.
- [x] T4.2 Tests: existing branch tests unchanged; `{key}` gives `AUC-1234`;
  an assigned branch wins over a format.

## 5. Server fallback cleanup (T1 of the clarification, US5)

- [x] T5.1 Runner fallback uses `models.TaskBranchName("", key, title)`;
  test.
- [x] T5.2 Delete the unreferenced `db.GenerateTaskBranchName`.

## 6. Web settings (FR-9, US2.4)

- [x] T6.1 `web/src/lib/branchNameFormat.ts` + `web/tests/branchNameFormat.test.mjs`.
- [x] T6.2 `Project.branchNameFormat`; modal input, presets, live example;
  French and English locale strings; payloads.
- [x] T6.3 `npm test` and `tsc -b` in `web/`; the Vite build is left to the
  pipeline, since it rewrites `internal/webui/dist`.

## 7. Documentation and skills (FR-11, FR-12)

- [x] T7.1 `CHANGELOG.md` `Added` line; `docs/CAPABILITIES.md`,
  `docs/API_AND_DATA_SPEC.md`, the server-agent contract if it lists config
  fields.
- [x] T7.2 `clarify/guard.md` wording; regenerate skill goldens and plugin
  test data under Linux.

## 8. Verification

- [x] T8.1 `go build ./...`, `go vet ./...`, `gofmt -l` on the touched files.
- [x] T8.2 The targeted Go tests of each touched package (models, db, agent,
  runner, taskmcp, handlers, skills), then the web tests; the PR pipeline runs
  the whole suite.
