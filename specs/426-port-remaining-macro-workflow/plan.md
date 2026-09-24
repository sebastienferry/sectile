# #426: Plan

Spec: [`spec.md`](spec.md). Stack: Go server (`internal/db`, `internal/handlers`,
`internal/trackerapi`, `internal/taskmcp`, `internal/skills`), Go local agent
(`internal/agent`, `internal/agentconfig`, `internal/agentprotocol`), React web client (`web/src`), Electron desktop
console (`desktop/`, attaches to agent sessions only).

Reference implementation: taskativ (`internal/db/macroworktree.go`,
`internal/db/sprints.go`, `internal/db/roadmapprojects.go`,
`internal/db/skilltemplates.go` `realignMacroFrameworkBody`,
`internal/runner/jira_teams.go`, `docs/MACRO_WORKFLOW.md`). Port behaviour, not
code layout: Sectile's tracker calls go through `trackerapi` adapters, not the
runner.

## Before starting

`feat/426` was three commits behind `origin/main`, which already had migrations
7 and 8. It was brought up to date with a merge of `origin/main` (a rebase
would have needed a force push, which the owner declined). The migrations
below are numbered from 9; if `main` gains another before the merge, renumber
ours, never theirs.

## Delivery order (one commit per item)

1. `refactor(macros): drop the scenarios slicing source` (US7)
2. `feat(projects): declare a specifications repository` (US1)
3. `feat(macros): one worktree per macro` (US2)
4. `feat(macros): run a macro skill from the panel` (US3, launch infrastructure)
5. `feat(skills): realign-macro` (US3, skill body)
6. `feat(macros): create a line's story in its target project` (US4)
7. `feat(macros): read stories from declared roadmap projects` (US5)
8. `feat(sprints): manage Jira sprints from the timeline` (US6)
9. `docs: changelog and ADR` (FR12)

## Architecture

### Macro worktree and launch (US2, US3)

All Git work happens on the machine that holds the checkouts, i.e. the local
agent, as for task worktrees and `git_evidence`. The server never creates a
worktree.

```
Web macro panel "Réaligner la spec"
   │  POST /api/projects/{id}/macros/{key}/run-skill {skillId:"realign_macro", mode, model}
   ▼
handlers: macro run-skill ──► db.StartMacroRun(projectID, macroKey, skill)   (activity: project_id + macro_key, task_id NULL)
   │                         busy check: db.ActiveRunOnMacro(projectID, key) → 409
   ▼
agentDispatcher.DispatchAndWait  "dispatch_step"  Dispatch{..., MacroKey, ProjectID, TaskID:""}
   ▼
agent.handleDispatchStep ──► macro branch (MacroKey set, TaskID empty)
   │  fetchConfig(projectID)            (no task lookup)
   │  root = localProjectRoot(project)  (code repository: skills + cwd)
   │  specRepo = workstation mapping settings.specRepos[project] or root
   │             (the server's SpecRepoPath is never sent: AgentConfig carries no server path)
   │  ensureMacroWorktree(specRepo, key, title, useWorktrees)
   │  Scaffold + bootstrapLocalMCP as today
   │  env: SECTILE_MACRO_KEY, SECTILE_MACRO_PROJECT_ID, SECTILE_SPEC_REPO,
   │       SECTILE_SPEC_BRANCH, SECTILE_SPEC_WORKTREE, SECTILE_RUN_ID; SECTILE_TASK_* empty
   │  command "/realign-macro <KEY>" + runId line
   ▼
PTY session (sessionId = runId) ─► desktop console attaches as for a task run
   ▼
session exit / skill end ─► MCP finish_run{projectId, macroKey, runId, status}

By hand: /realign-macro M-7 in any agent session
   ▼
MCP prepare_macro_worktree{projectId, macroKey}
   ▼ server
agent operation "macro_worktree" {ProjectID, MacroKey} ─► same ensureMacroWorktree
   ▼
{path, branch, worktree, warning}
```

**Two paths for one setting (owner decision, 2026-09-24).** The project's
`specRepoPath` is read by the server only, for the slicing import it runs on
its own filesystem. The agent never receives it: `AgentConfig` exposes no
server path by design, so the workstation declares its own specifications
checkout per project (`specRepos` in the local settings, next to `projects`),
edited in the desktop project dialog. Without a mapping the project's local
checkout carries the specifications.

The shape of the macro dispatch follows the taskless desktop console
(`internal/agent/agent_console.go` `launchConsole`): no task, mapped checkout,
`SECTILE_TASK_*` empty.

### Story in the target project (US4)

```
POST /api/projects/{id}/macros/{key}/story {todoId, title}
   ▼
db.CreateStoryFromMacroTodo
   ├─ target = todo.TargetProjectID or macro project
   ├─ sameTrackerInstance(macroProject, targetProject) ─ no ─► refusal (FR7), nothing written
   ├─ CreateStoryUnderMacro(targetProjectID, macroProject, macroKey, text)
   │     ├─ CreateTask in target project
   │     ├─ writeTaskParentLocally
   │     ├─ GitHub: SetGithubIssueMilestone (unchanged)
   │     └─ Jira:   JiraAdapter.SetParent(storyKey, epicKey)   (sync; failure → notice, key kept)
   └─ todo.StoryKey = key; SaveMacroMeta
```

### Sprints (US6)

Synchronous tracker writes, outside the tracker operation queue, as taskativ
does (a sprint must exist on Jira before a ticket can be moved to it). The
mirror stays the `projects.sprints` JSON column that board sync already
refreshes; no new column.

```
Timeline (Jira project)
   │ POST   /api/projects/{id}/sprints              {name, count, start, weeks}
   │ PATCH  /api/projects/{id}/sprints/{sprintId}   {name?, start?, end?, state?, moveOpenTo?}
   │ DELETE /api/projects/{id}/sprints/{sprintId}
   ▼
db.CreateProjectSprints / UpdateProjectSprint / DeleteProjectSprint
   ▼
tracker.SprintManager (new optional interface, capability CapSprintManage)
   └─ JiraAdapter.CreateSprint / UpdateSprint / DeleteSprint  (/rest/agile/1.0/sprint)
   ▼
mirror Jira's answer into projects.sprints (replace / append / forget by id)
```

## Target files

| File | Change |
| --- | --- |
| `internal/models/models.go` | Remove `MacroTodoFromScenarios` and fix the "trois" comment (US7). `Project.SpecRepoPath` (`specRepoPath`), `Project.RoadmapProjects []string` (`roadmapProjects`) and their `CreateProjectRequest` / `UpdateProjectRequest` fields (`*string`, `*[]string`). `SkillDirNames`: `realign_macro`, `realign-macro` → `"realign-macro"`. `SprintPatch{Name, Start, End, State, MoveOpenTo *string}`. Update the `TargetProjectID` comment (now consumed). |
| `internal/db/migrations.go` | v9 `projects.spec_repo_path` (`TEXT NOT NULL DEFAULT ''`); v10 `projects.roadmap_projects` (`TEXT NOT NULL DEFAULT '[]'`); v11 `task_activities.macro_key` (`TEXT NOT NULL DEFAULT ''`) + index `(project_id, macro_key)`. Never the baseline (`lateColumns` and the baseline `CREATE TABLE` stay frozen). |
| `internal/db/db.go` | Project SELECT lists and scans (both), INSERT, UPDATE and request handling for the two project columns; `NormalizeRoadmapProjects` on create/update. |
| `internal/db/schemaparity_test.go` | Passes with the new columns on both engines. |
| `internal/db/sddslicing.go` | `macroSpecRepoPath` returns `SpecRepoPath` when set, else `RepoPath`; the "Deux sources" header stays true. `FindMacroSpecDir` refusal mentions the "Dépôt des spécifications" option. |
| `internal/db/sddentries.go` | `entryKeyPrefixes(proj)` returns the Jira key plus `proj.RoadmapProjects`; update its gap comment. |
| `internal/db/roadmapprojects.go` (new) | `NormalizeRoadmapProjects(raw []string, ownKey string) []string` (upper-case, split on commas and blanks, dedupe, drop own key), `parseRoadmapProjects` (tolerant JSON), `isRoadmapProjectKey(proj, key)` used to refuse writes (FR8). |
| `internal/models/macrobranch.go` (new) | `MacroBranchMatches(ref, key)` (shared by the server slicing read and the agent) and `MacroBranchName(key, title)` (`KEY-<slug≤30>`). `findMacroBranch` in `sddslicing.go` calls the matcher. |
| `internal/agent/agent_macro_worktree.go` (new) | `ensureMacroWorktree(ctx, specRepo, key, title, useWorktrees) (macroWorkspace, error)`, next to `ensureLocalWorktree` whose helpers it reuses (`gitLocal`, `worktreeForBranch`, `sameDirectory`): per-repository lock; fetch `--prune` (non-fatal, warning); existing macro branch (local, then `origin/`), else `MacroBranchName`; reuse a checkout of the branch anywhere; `.tasks/` in `info/exclude`; a stale path is pruned and removed only when empty, otherwise refused; `worktree add -b <branch> <path> origin/<default>` (or the local default); never the default branch. Placed in the agent rather than `internal/workspace`: the agent is its only caller, and the server may not import `workspace` (runtime boundary test). |
| `internal/agentprotocol/operations.go` | `MacroKey string \`json:"macroKey,omitempty"\`` on `Operation`; operation `macro_worktree`. |
| `internal/agentconfig/config.go` | `Dispatch.MacroKey`, `Dispatch.MacroTitle`. |
| `internal/agentconfig/local.go` | `Overrides.SpecRepos map[projectID]path`, workstation-owned like `Projects`. |
| `internal/agent/agent_macro_dispatch.go` (new) | `handleMacroDispatch` (interactive only, admission and slot as a task run, no task read, no branch recorded), `prepareMacroWorkspace`, `macroWorkspaceFor` (operation), `localSpecRepo`, `macroOfRun`. |
| `internal/agent/agent_desktop.go` | `desktopRun.MacroKey`; `/desktop/projects` reads and writes `specPath`; `finishDesktopRun` reports a macro run under `projectId` + `macroKey`. |
| `desktop/src/main.js` | "Specifications repository" field in the project dialog (General); macro runs grouped by macro, no next-step lookup, no task link. |
| `internal/agentmcp/mcp.go`, `internal/mcptest/contract.go` | The stdio bridge and the catalog contract know eleven tools. |
| `docs/contracts/server-agent-v1.md`, `docs/ARCHITECTURE.md`, `README.md` | Macro dispatch, `macro_worktree`, the macro form of `start_run`/`finish_run`, `prepare_macro_worktree`. |
| `internal/agent/agent_operations.go` | `macro_worktree`: resolve the local code root and the spec repo for the project, call `ensureMacroWorktree`, answer `{path, branch, worktree, warning}`. |
| `internal/agent/agent.go`, `agent_config.go` | `handleDispatchStep`: a dispatch with `MacroKey` and no task takes a macro path (no task fetch, no task worktree, no `patchTask`), prepares the macro worktree, sets the `SECTILE_MACRO_*` / `SECTILE_SPEC_*` env, builds `/<command> <KEY>`. Admission and concurrency as for a task run, keyed on the run. |
| `internal/agent/agent_desktop.go` | `finishDesktopRun` sends `projectId` + `macroKey` for a macro run. |
| `internal/db/macroruns.go` (new), `remoterun.go` | `StartMacroRun`, `StartMacroRunBy` (MCP, runId reuse), `FinishMacroRunAs`, `ActiveRunOnMacro`, `MacroRuns`, `PrepareMacroWorktree` (relays the `macro_worktree` operation); activities with `project_id` set, `task_id` NULL, `macro_key` written by an UPDATE after the insert (as `run_mode` is), so the six explicit column lists of `task_activities` are untouched; no stage, no chain, no post-back. `FinishRemoteRun("", runID)` closes an adopted macro run on a session end. |
| `internal/db/macros.go` | `CreateStoryFromMacroTodo` consumes `TargetProjectID`; `sameTrackerInstance(a, b *models.Project) (bool, string)` returning the French reason; `CreateStoryUnderMacro` takes the target project and writes the Jira parent through `SetParent`, returning a notice on failure; refuse when the line's `StoryKey` belongs to a roadmap project. `GetMacroActivities` (or a filter on project activities) for the panel. |
| `internal/trackerapi/jira.go` | `CreateSprint(ctx, boardID, name, start, end)`, `UpdateSprint(ctx, id, patch)`, `DeleteSprint(ctx, id)` on `/rest/agile/1.0/sprint`; dates RFC3339, a bare end day becomes 23:59:59; 404 on delete is success; Jira errors surfaced through `jiraError`. Declare `CapSprintManage`. |
| `internal/tracker/tracker.go`, `internal/tracker/sprints.go` (new) | `CapSprintManage` and its French `CapabilityLabel`; optional `SprintManager` interface (`CreateSprint`, `UpdateSprint`, `DeleteSprint`) with `SprintCreateRequest`. |
| `internal/trackerapi/jira_sprints.go` (new) | The Jira implementation on `/rest/agile/1.0/sprint`. |
| `internal/handlers/sprints.go` (new) | The sprint routes. |
| `web/src/lib/sprintApi.ts` (new) | Client for the sprint routes; the timeline re-reads the project after each write. |
| `internal/db/sprints.go` (new) | `CreateProjectSprints(projectID, pattern string, count int, start time.Time, weeks int) ([]TrackerSprint, error)` (count 1..12, weeks 1..4, `{n}` naming, conflict check against the board's live list before any write, partial-failure error listing what was created); `UpdateProjectSprint(projectID, sprintID string, patch SprintPatch)` (moves unfinished tickets with `SetSprint` / backlog synchronously before a close when `MoveOpenTo` is set); `DeleteProjectSprint`. Mirror helpers `replaceProjectSprint`, `appendProjectSprints`, `forgetProjectSprint`. |
| `internal/handlers/handlers.go` | Routes: `POST /api/projects/{id}/macros/{key}/run-skill`; `GET /api/projects/{id}/macros/{key}/runs` (or the activity field on the macro payload); `POST /api/projects/{id}/sprints` (200 `{created}`, 207 `{created, error}`), `PATCH\|PUT /api/projects/{id}/sprints/{sprintId}`, `DELETE /api/projects/{id}/sprints/{sprintId}` (400 with the message on refusal, 409 when the tracker cannot manage sprints). |
| `internal/taskmcp/server.go` | New tool `prepare_macro_worktree{projectId, macroKey}` → agent operation `macro_worktree` for the caller's agent. `start_run` / `finish_run` accept `projectId` + `macroKey` instead of `taskKey` (exactly one of the two forms). |
| `internal/skills/catalog.go` | `realign_macro` entry next to `refine_macro`: `Scope: "macro"`, stages `"macro"`, interactive by default, command `realign-macro`. Macro-scoped skills get a run contract adapted to a macro key (start/finish with `projectId` + `macroKey`, reuse `SECTILE_RUN_ID`). |
| `internal/skills/fragments/realign_macro/` (new) | `goal.md`, `read-first.md`, `steps.speckit.md`, `steps.openspec.md`, `guard.md`, `report.md`: port of `realignMacroFrameworkBody` with Sectile's names (`SECTILE_SPEC_WORKTREE`/`SECTILE_SPEC_BRANCH` or the `prepare_macro_worktree` answer; macro read via `GET /api/projects/{id}/macros` todos with `sourceKind`/`sourceEntry`; no `scenarios` case). Commit and push rules as in taskativ's `macroReadOnlyContract` "Pushing" section. |
| `internal/skills/testdata/golden/realign_macro.{speckit,openspec}.{skill,command}.md` (new) | Generated with `UPDATE_GOLDEN=1`. |
| `web/src/types/index.ts` | Drop `'scenarios'` from `MacroTodoSource`; `Project.specRepoPath?`, `Project.roadmapProjects?`. |
| `web/src/components/ProjectModal.tsx`, `web/src/locales/translations.ts` | "Dépôt des spécifications" input + hint (repository settings); "Projets de roadmap" comma list (Jira only); helpers `web/src/lib/roadmapProjects.ts` (`formatProjectKeyList`, `parseProjectKeyList`). |
| `web/src/components/RoadmapView.tsx`, `web/src/context/AppContext.tsx` | "Réaligner la spec" button next to "Raffiner AI": disabled without agent (`useAgentStatus`) or while a macro run is active; `runMacroSkill(projectId, key, skillId)`; active-run indicator on the macro; refusal/notice of story creation surfaced as today's errors. Per-line target project picker (macro project + same-instance projects, from a mirror of `sameTrackerInstance` in `web/src/lib/lookups.ts` next to `isProjectCompatible`), saved through the existing macro todos save; read-only once the line has a story; invalid saved target flagged. |
| `web/src/components/SprintTimelineView.tsx`, `web/src/lib/sprints.ts`, `AppContext.tsx` | On Jira: creation bar (pattern, count, start, weeks 1 to 4), rename/dates, close (with next sprint / backlog choice), delete (confirm) call the new routes and render the server's answer; moves pass the sprint **id**. On GitHub: no creation/edit/close/delete controls; sprints already stored locally stay visible, read-only, and accept no ticket move. Local projects: unchanged. French strings through `translations.ts`. |
| `desktop/` | No change expected: the console attaches by `sessionId` = runId. Verify a macro run appears in the run list with the macro key as its label; adjust the label if it shows a blank task key. |
| `CHANGELOG.md` | Under `[Unreleased]`: `Added` specifications repository, macro worktree, realign-macro, Jira sprint management, roadmap projects; `Changed` stories created under a Jira epic now get the parent on Jira and honour the line's target project. |
| `docs/adrs/0026-macro-runs-and-macro-worktrees.md` (new) | Why a macro run is a project activity with `macro_key` (not a pseudo task id, see #310), and why the macro worktree is created by the agent. |

## Data contracts

### Project

```json
{ "specRepoPath": "/Users/me/Sources/wiki", "roadmapProjects": ["ABC", "DEF"] }
```

`specRepoPath` trimmed, `""` = unset. `roadmapProjects` normalised on write;
ignored (and not offered) unless `issueTracker == "jira"`.

### Same tracker instance

| Macro project | Target project | Same instance when |
| --- | --- | --- |
| Jira | Jira | resolved Jira base URLs equal after lower-casing the host and trimming a trailing `/` (`trackerCredentials` resolution: project `trackerUrl`, else settings, else env) |
| GitHub | GitHub | resolved API URLs equal **and** `githubRepo` equal (case-insensitive) |
| local | local | always |
| any other pair | | never |

Refusal messages (French), each naming the target project:
`le projet cible « X » utilise le tracker jira, la macro github : les deux doivent partager le même tracker`,
`… est sur une autre instance Jira (…)`,
`… est un autre dépôt GitHub : un milestone ne peut pas y rattacher la story`,
`le projet cible « X » n'existe plus`.

### `macro_worktree` operation answer / `prepare_macro_worktree` result

```json
{ "path": "/…/wiki/.tasks/worktrees/M-7", "branch": "M-7-ux-improvements-and-fixes",
  "worktree": true, "warning": "fetch failed: … ; base may be stale" }
```

With `useWorktrees` off: `path` = spec repo, `worktree: false`, `branch` = the
macro branch (found or proposed); the skill checks that the checkout is on it.

### Macro run-skill

Request `{skillId, mode?, model?, force?}`. Responses: 202 `{runId, activity}`;
409 busy; 424 no agent; 400 unknown or non-macro skill.
Dispatch payload adds `"macroKey": "M-7"` with `taskId`/`taskKey` empty.

### Sprints

- `POST /api/projects/{id}/sprints` `{name, count, start: "YYYY-MM-DD", weeks}`;
  start parsed at 09:00 local; each sprint ends `start + weeks*7d − 1s`; the
  next starts at `start + weeks*7d`.
- `PATCH /api/projects/{id}/sprints/{sprintId}` `SprintPatch`:
  `state ∈ {active, closed, future}` passed to Jira; `moveOpenTo ∈ {"next",
  "backlog"}` only with `state: "closed"`.
- Mirror: `projects.sprints` entries keep `{id, name, state, startDate, endDate}`
  as Jira returns them.

## Rejected alternatives

- **Server-side macro worktree.** The server reads the slicing on its own
  filesystem, but the Run session and the skill write on the agent's machine;
  creating the worktree on the server would give the agent a path it may not
  have. Rejected for the agent operation.
- **A pseudo task id for macro runs** (`macro-<key>` in `task_id`): the magic
  string #310 removed, and a broken foreign key on PostgreSQL.
- **Sprint writes through the tracker operation queue**: a sprint must exist on
  Jira before the UI can move tickets to it or show its id.
- **GitLab iterations now**: needs a GitLab tracker adapter Sectile lacks
  (follow-up ticket #430).

## Risks

- Jira Agile write permissions differ from read ones; the error must reach the
  user verbatim.
- `SprintTimelineView` today invents `sprint-N` ids and default sprints; on Jira
  these paths must be disabled, not merely bypassed, or a save will overwrite
  the mirror until the next sync.
- A spec path that differs between the server and the agent machine: the slicing
  (server) and the worktree (agent) would read different checkouts. Documented
  in the option hint; not solved here.
