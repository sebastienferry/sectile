# Tasks #663 - Roadmap: ordered epic todos, mirrored on the tracker

Ordered checklist. Each group is one commit and leaves the tree buildable.
References point at `spec.md` (US, FR) and `plan.md`.

## 0. Base

- [ ] T0.1 Fetch and compare `feat/663` with `origin/main`; merge `origin/main`
  rather than rebase if it moved. Check the highest migration number on
  `origin/main` before T1.1.

## 1. Schema and mirror status (FR17, NFR4)

- [ ] T1.1 Migration 39 `macros.todos_mirror` (four columns) in
  `internal/db/migrations.go`, never in the baseline.
- [ ] T1.2 Update the rewind helpers and the forget-and-replay fixtures of the
  db tests for the new columns.
- [ ] T1.3 `models.MacroTodosMirror` and `MacroMeta.TodosMirror`.
- [ ] T1.4 Tests: the db suite on SQLite, and on PostgreSQL with a throwaway
  DSN.

## 2. Eligibility, rendering, block split (FR12, FR14, FR15)

- [ ] T2.1 `internal/db/macrotodosmirror.go`: `macroTodosMirror`,
  `renderTodosMirror`, `splitTodosBlock`, `joinTodosBlock`, body hash.
- [ ] T2.2 Fill `TodosMirror` on every macro read returned to a client
  (`GetProjectMacros`, PUT answer, `findMacroMeta`).
- [ ] T2.3 Tests: eligibility table, rendering per kind (order, checkboxes,
  keys, empty list, truncation), block round trips including missing and
  malformed blocks.

## 3. Tracker writes (FR10, FR11)

- [ ] T3.1 `tracker.MarkedCommentWriter` and `UpsertMarkedCommentRequest` in
  `internal/tracker/ticketing.go`.
- [ ] T3.2 `JiraAdapter.UpsertMarkedComment`: PUT by id, 404 then property
  search, then POST with the property. Check `MarkdownToADF` on the rendered
  list and switch to the `☐` / `☑` rendering if needed.
- [ ] T3.3 `GetGithubMilestone` and `SetGithubMilestoneDescription` in
  `internal/trackerapi/github.go`.
- [ ] T3.4 Tests with httptest servers: Jira create, update, 404 fallback,
  foreign comment untouched; GitHub read and description write, empty
  description sent.

## 4. Queued write and scheduling (FR6 - FR9, FR13)

- [ ] T4.1 `TrackerOpEpicTodos` in `trackerops.go`: activity text, dispatch
  to `runEpicTodosOp`.
- [ ] T4.2 `PushMacroTodosMirror`: re-check, render current list, hash no-op,
  `retryTransient`, store ref / hash / time / error; keep the
  credential-missing chain (#645).
- [ ] T4.3 `scheduleTodosMirror` with `todosMirrorTimers` and
  `todosMirrorDelay`, carrying the actor of the last save.
- [ ] T4.4 Call it from `UpdateMacro` (todos), `recordLineStoryKey`, the end
  of `CreateStoriesFromMacroTodos`, `TodosFromSDD`, `TodosFromMacroStories`
  (give it a ctx, update its handler call).
- [ ] T4.5 Tests: debounce coalesces; last list rendered; ineligible macros
  queue nothing; each call site schedules; job outcomes with a fake tracker
  (create, update, retry then success, permanent failure stored, success
  clears error).

## 5. GitHub description sharing (FR16)

- [ ] T5.1 `UpdateMacro` GitHub branch: description plus block through
  `SetGithubMilestoneDescription`; store the block hash on success.
- [ ] T5.2 `GetProjectMacros` milestone import strips the block.
- [ ] T5.3 Tests: description edit keeps the block; import strips it; block
  only when the description is empty (US4).

## 6. Republish route (FR18)

- [ ] T6.1 `POST /api/projects/{id}/macros/{key}/todos-mirror` (and `epics`).
- [ ] T6.2 Handler tests: 202 with activity; 400 with the reason on an
  ineligible macro.

## 7. MCP (FR19 - FR21, US6)

- [ ] T7.1 `GetMacro` and `ReplaceMacroTodos` in `internal/db`.
- [ ] T7.2 `get_macro` and `update_macro_todos` in
  `internal/taskmcp/server.go`, with `requireCaller` and
  `tracker.WithActingUser` on the write.
- [ ] T7.3 Tests: db merge rules and whole-list refusals; MCP tool listing,
  answer shapes, anonymous and unknown-macro refusals.
- [ ] T7.4 Update the MCP tool list in `docs/ARCHITECTURE.md` (and
  `docs/contracts/server-agent-v1.md` if it lists MCP tools).

## 8. Skill (FR22, US7)

- [ ] T8.1 `refine_macro` fragments: read-first, steps, guard, report.
- [ ] T8.2 Regenerate the four `refine_macro.*` goldens, read the diff;
  `catalog_test.go` and `plugin_test.go` green. Do not regenerate the
  marketplace plugin.

## 9. Web panel (US1, US2, US5)

- [ ] T9.1 Types for `todosMirror`; API call for the republish route.
- [ ] T9.2 `moveTodo` helper in `web/src/lib/roadmap.ts` with unit tests.
- [ ] T9.3 Inline rewording in `RoadmapView.tsx` (Enter, blur, Escape, blank,
  unchanged), disabled during a batch.
- [ ] T9.4 Drag handle, move up / down buttons, Alt+Up / Alt+Down, focus kept;
  disabled during a batch; no control for a single todo.
- [ ] T9.5 Mirror status line, link, **Republier**, credential offer; refresh
  the macro when the `epic_todos` activity ends.
- [ ] T9.6 French and English strings in `web/src/locales/planning.ts`.
- [ ] T9.7 Browser component test of the panel; `tsc`, `oxlint`; `npx vite
  build`, then restore `internal/webui` `.gitkeep`.

## 10. Documentation

- [ ] T10.1 ADR 0045 "Macro todos are mirrored one way on the tracker":
  Sectile authoritative, Jira comment with a property marker, GitHub
  description block, no GitLab mirror (D5), refused on declared roadmap
  projects.
- [ ] T10.2 `CHANGELOG.md`, `## [Unreleased]` / `Added`, one line (AC6).

## 11. Verification

- [ ] T11.1 `go test ./...` (sandbox off for httptest), the db suite on
  PostgreSQL, the web tests.
- [ ] T11.2 Manual check on a copy of the dev database, server started
  without a tracker token for writes that must not reach the real tracker;
  then one Jira epic and one GitHub milestone of a test project: AC2, AC3,
  AC4.
