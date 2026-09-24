# #426: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md). One commit per section
0 to 9, in this order. Run `go test ./...` and the web checks after each
section; UI sections also run the browser tests.

## 0. Prepare the branch

- [ ] T0.1 Rebase `feat/426` on `origin/main`; confirm the last migration version on `main` (8 at the time of writing) and number ours from the next one.

## 1. Drop the `scenarios` source (US7, FR11)

- [ ] T1.1 Delete `models.MacroTodoFromScenarios`; rewrite the origin comments so none counts three sources.
- [ ] T1.2 Drop `'scenarios'` from `MacroTodoSource` in `web/src/types/index.ts`; `tsc` passes.
- [ ] T1.3 Test: a stored todo with `sourceKind: "scenarios"` loads and is treated as typed by hand by the slicing merge.

## 2. Specifications repository (US1, FR1)

- [ ] T2.1 Migration `projects.spec_repo_path`; model, requests, SELECT/scan/INSERT/UPDATE in `db.go`; schema parity test.
- [ ] T2.2 `macroSpecRepoPath` prefers `SpecRepoPath`; `FindMacroSpecDir` refusal names the option.
- [ ] T2.3 ProjectModal input "Dépôt des spécifications" + hint; translations (fr, en).
- [ ] T2.4 Tests: fallback to `repoPath` when empty or blank; slicing import from a second temp repository reads only it; update clears the value; refusal text.

## 3. Macro worktree (US2, FR2, FR4)

- [ ] T3.1 `internal/workspace/macroworktree.go`: `MacroBranchName`, `EnsureMacroWorktree`, per-repository lock, info/exclude entry; branch matcher shared with `findMacroBranch`.
- [ ] T3.2 Tests with temp repositories (bare remote + clone):
  - creates `.tasks/worktrees/M-7` on `M-7-<slug>` from `origin/main` after a new commit lands on the remote;
  - reuses an existing `M-7-foo` branch;
  - reuses a dirty worktree without touching its changes;
  - returns the main checkout when it is on the macro branch;
  - re-creates an invalid directory at the worktree path;
  - M-7 and M-8 get separate trees, an untracked file in one is absent from the other;
  - fetch failure (no remote) proceeds with a warning;
  - `useWorktrees=false` returns the repo, `worktree:false`, no tree created;
  - non-Git path fails naming it; never the default branch;
  - two concurrent calls return the same tree.

## 4. Macro runs (US3 launch, FR3, FR6)

- [ ] T4.1 Migration `task_activities.macro_key` + index; `StartMacroRun`, `FinishMacroRunAs`, `ActiveRunOnMacro`; activities with `project_id`, `macro_key`, `task_id` NULL.
- [ ] T4.2 `POST /api/projects/{id}/macros/{key}/run-skill`: skill must be macro-scoped; 409 busy; 424 no agent; dispatch with `MacroKey`.
- [ ] T4.3 `agentconfig.Dispatch.MacroKey`; agent macro path in `handleDispatchStep` (no task fetch, macro worktree, env, command `/<cmd> <KEY>`); `finishDesktopRun` sends the macro form.
- [ ] T4.4 Agent operation `macro_worktree`; MCP tool `prepare_macro_worktree`; `start_run` / `finish_run` accept `projectId` + `macroKey` (exactly one form, else a clear error).
- [ ] T4.5 Web: "Réaligner la spec" button, disabled without agent or while busy; active-run indicator on the macro.
- [ ] T4.6 Tests: handler refuses a task skill and a busy macro; dispatch payload carries `macroKey` and no task; agent macro dispatch builds env and command without calling `/api/tasks`; MCP start/finish with the macro form update the macro activity and refuse mixed forms; PostgreSQL run (see memory on the test DSN) accepts `task_id` NULL with `macro_key`.
- [ ] T4.7 Desktop: a macro run shows in the console with the macro key as label (manual check, fix the label if blank).

## 5. `realign-macro` skill (US3 behaviour, FR5)

- [ ] T5.1 `SkillDirNames` aliases; catalog entry `realign_macro` (scope macro, interactive); run contract for macro skills.
- [ ] T5.2 Fragments `internal/skills/fragments/realign_macro/*` (Spec Kit and OpenSpec steps, guard, report) per spec US3.
- [ ] T5.3 Golden files `realign_macro.{speckit,openspec}.{skill,command}.md` via `UPDATE_GOLDEN=1`.
- [ ] T5.4 Tests (port of taskativ `realignmacro_test.go`): body names the four cases, the orphan suffix, "never renumber", "never delete", "never the default branch", no `scenarios` case; framework variant follows the project; alias resolution; the skill appears as a macro skill.

## 6. Target project and Jira parent (US4, FR7)

- [ ] T6.1 `sameTrackerInstance` with French reasons; `CreateStoryFromMacroTodo` consumes `TargetProjectID`; missing target refused.
- [ ] T6.2 `CreateStoryUnderMacro` creates in the target and calls `JiraAdapter.SetParent(story, epic)`; failure returns a notice, key kept.
- [ ] T6.3 Handler returns the notice with the created story.
- [ ] T6.4 Tests with a fake tracker: no target → macro project, Jira parent written; Jira target same URL → created there, parent written; other URL / other kind / other GitHub repo → refused, no create call, line unchanged; parent failure → key recorded, notice; GitHub milestone path unchanged.
- [ ] T6.5 Leave the target picker unimplemented until O1 is settled.

## 7. Roadmap projects (US5, FR8)

- [ ] T7.1 Migration `projects.roadmap_projects`; model, requests, db plumbing; `NormalizeRoadmapProjects` on write.
- [ ] T7.2 `entryKeyPrefixes` includes them; write paths (story creation, parent, sprint move, labels) refuse a roadmap-project key.
- [ ] T7.3 ProjectModal comma list (Jira only), `web/src/lib/roadmapProjects.ts` helpers with tests.
- [ ] T7.4 Tests: normalisation (`abc, DEF abc SFE` on SFE → `ABC, DEF`); `ABC-12 …` attaches on import, `XYZ-1 …` does not; write refused on an `ABC-` key.

## 8. Jira sprint management (US6, FR9, FR10)

- [ ] T8.1 `CapSprintManage`, `SprintManager`; `JiraAdapter.CreateSprint` / `UpdateSprint` / `DeleteSprint`.
- [ ] T8.2 `internal/db/sprints.go`: batch create (validation, naming, conflict check, partial failure), update (with `moveOpenTo` before close), delete; mirror helpers.
- [ ] T8.3 Routes `POST/PATCH/PUT/DELETE /api/projects/{id}/sprints[/{sprintId}]`.
- [ ] T8.4 `SprintTimelineView` on Jira: calls the routes, renders the answer, no local `sprint-N` ids or default sprints, moves by id; GitHub: controls hidden; local: unchanged.
- [ ] T8.5 Tests against an `httptest` Jira: the three-sprint example of US6 (names, dates, board id); count 0/13 and weeks 0/5 refused before any request; taken name refused before any request; failure on the third → two mirrored, error text; rename/dates mirror Jira's answer; close with `moveOpenTo: next` moves open issues then closes; Jira refusal leaves the mirror unchanged; delete 404 is success; no board → refusal; GitHub project → 409.
- [ ] T8.6 Web tests for `lib/sprints.ts` naming/date helpers used by the creation bar.

## 9. Docs

- [ ] T9.1 `CHANGELOG.md` `[Unreleased]` lines per plan.
- [ ] T9.2 ADR "macro runs and worktrees" in `docs/adrs/`.
- [ ] T9.3 Open the follow-up ticket for GitLab iterations (needs a GitLab tracker adapter) under M-7, after the owner's approval.

## Test plan summary

| Story | Tests |
| --- | --- |
| US1 | T2.4 |
| US2 | T3.2 |
| US3 | T4.6, T4.7, T5.4, manual realignment on a test project |
| US4 | T6.4 |
| US5 | T7.4 |
| US6 | T8.5, T8.6 |
| US7 | T1.3, `tsc` |

## Blocked by open requirements

- O1 blocks T6.5 (target picker UI) only.
- O2 blocks the GitHub branch of T8.4 only (whether existing local sprints stay visible read-only).
