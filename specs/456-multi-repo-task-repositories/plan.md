# #456: Plan

Spec: [`spec.md`](spec.md). Stack: Go server (`internal/db`, `internal/handlers`,
`internal/taskmcp`), Go local agent (`internal/agent`, `internal/agentconfig`),
WebSocket agent operations (`agentprotocol.Operation`), shared pure helpers
(`internal/models`), React web (`web/src`), Electron desktop (`desktop/src`).

## Architecture

```
                 server                                   local agent (workstation)
 ┌────────────────────────────────────┐        ┌───────────────────────────────────────────┐
 │ projects.repositories  [remotes]   │        │ settings.json                             │
 │ tasks.repository       identity    │        │   projects[id]      → code remote root    │
 │ tasks.changed_repositories [ids]   │        │   repositories[identity] → folder  (new)  │
 │ task_activities.waiting_reason     │        │   specRepos[id]     → spec checkout       │
 └────────────────────────────────────┘        └───────────────────────────────────────────┘

 dispatch ─► handleDispatchStep ─► prepareDispatchLocked
                                     ├─ convertLegacyRepoPaths (once per project, FR5)
                                     ├─ resolvePrimaryRepository(task, project, mappings)
                                     │     ├─ resolved ─► root of that repository
                                     │     └─ ambiguous / unmapped pin ─► awaitRepository
                                     │            POST /api/activities/{id}/awaiting-repository
                                     │            poll GET /api/tasks/{key} until pinned or canceled
                                     ├─ ensureLocalWorktree(primary root)          (unchanged logic)
                                     └─ buildFolderMap ─► SECTILE_REPOSITORIES + prompt block
                                                         + --add-dir per context folder (claude)

 skill ─► MCP prepare_repository_worktree{taskKey, repository}
          ─► DB.PrepareRepositoryWorktree (checks) ─► callAgentContext "repository_worktree"
          ─► agent: mapping ─► ensureLocalWorktree(secondary root) ─► {repository, path, branch}
          ─► tasks.changed_repositories += identity

 transition / post-back / adjust ─► validateStagePRs(task, prURLs)
          for each changed repository: pick its PR ─► validateStagePR (#392 rules, head on its checkout)
```

## Target files

| File | Change |
| --- | --- |
| `internal/models/repository.go` | `ProjectRepository{URL, Identity}`, `NormalizeProjectRepositories(codeRemote, urls)` (code remote first, dedupe by `RepositoryIdentity`), `FolderMapEntry`. |
| `internal/models/models.go` | `Project.Repositories []string`, `Project.RepositoriesMigration string`; `Task.Repository string`, `Task.ChangedRepositories []string`; request types. `RepoPath` / `RepoPaths` stay, marked legacy. |
| `internal/db/migrations.go` | Migrations 17 to 21 (below; `main` landed 15 and 16 first). |
| `internal/db/db.go` | Read/write of the new columns in `getProjectsUnsafe`, `getProjectByIDUnsafe`, `CreateProjectAs`, `UpdateProjectAs`, `updateTaskBy`; pin validation (FR3); `registerProjectRepoPathUnsafe` no longer called for new writes. |
| `internal/db/repositories.go` (new) | `ApplyRepositoryConversion` (compare-and-set on `repositories_migration`), `PrepareRepositoryWorktree`, `AddChangedRepository`, `MarkRunAwaitingRepository`, `ClearRunAwaitingRepository`. |
| `internal/db/adjustment.go` | `validateStagePRs` over the changed repositories; `adjustmentPrerequisite` iterates. |
| `internal/db/stage.go`, `internal/db/postback.go` | Accept `prURLs`; call `validateStagePRs`. |
| `internal/db/db.go` (`RemoveTaskWorktree`) | Sends the task's repositories in the `remove_workspace` operation. |
| `internal/handlers/handlers.go` | `POST /api/projects/{id}/repositories/convert`; `GET /api/projects/{id}/legacy-repo-paths`; `POST /api/activities/{id}/awaiting-repository` (the run's owner or an admin); task `repository` in PUT/PATCH. |
| `internal/taskmcp/server.go` | `prepare_repository_worktree` tool; `transition_stage` gains `prUrls`. |
| `internal/agentprotocol/operations.go` | `Repositories []string` on `Operation`; action `repository_worktree`. |
| `internal/agentconfig/local.go`, `settings.go` | `Overrides.Repositories map[string]string` (`json:"repositories,omitempty"`); `WriteSettings` deletes an emptied `repositories` (and `specRepos`, same defect). |
| `internal/agent/repositories.go` (new) | `resolveRepositoryRoot`, `resolvePrimaryRepository`, `convertLegacyRepoPaths`, `buildFolderMap`, `awaitRepository`. |
| `internal/agent/agent_config.go` | `prepareDispatchLocked` uses the primary root; `modeCommandLine` / `headlessCommandLine` take the context folders; `{addDirs}` placeholder in `expandConfiguredTemplate`; error text names `~/.config/sectile/settings.json`. |
| `internal/agent/agent.go` | Env `SECTILE_REPOSITORIES`; folder map prompt block next to the retry notes; a waiting dispatch does not go through `finishDesktopRun(..., "failed")`. |
| `internal/agent/agent_operations.go` | `repository_worktree`; `remove_workspace` over `op.Repositories`. |
| `internal/agent/evidence.go` | `checkoutCandidates` puts the workstation mapping of `op.Repository` first; legacy paths stay as hints until converted. |
| `internal/agent/agent_desktop_repositories.go` | `GET /desktop/repositories?projectId=` (repositories with their folder here) and `POST /desktop/repositories` `{projectId, repository, path?, taskId?}` (map after the origin check, and pin). One pair serves the project settings and the waiting run's choice. |
| `desktop/src/main.js`, `desktop/electron/main.cjs`, `preload.cjs` | Project dialog: one folder per project repository; repository picker on a run waiting with reason `repository`. |
| `web/src/components/ProjectModal.tsx` | Repositories list editor (shown when `monoRepo` is false); conversion report notice. |
| `web/src/components/TaskDetailModal.tsx` | `repoPath` text field replaced by a repository select among the project's repositories. |
| `web/src/types/index.ts`, `web/src/locales/translations.ts` | New fields and French strings. |
| `internal/skills/fragments/**` | Implement / adjust / handoff fragments: read the folder map, request a worktree before changing a context folder, one PR per changed repository. |
| `docs/adrs/0027-repositories-are-keyed-by-remote.md` | New ADR. |
| `CHANGELOG.md` | `Added` entry under `[Unreleased]`. |

## Data contracts

### Migrations (one statement each, both engines)

| Version | Statement |
| --- | --- |
| 17 | `ALTER TABLE projects ADD COLUMN repositories TEXT NOT NULL DEFAULT '[]';` |
| 18 | `ALTER TABLE projects ADD COLUMN repositories_migration TEXT NOT NULL DEFAULT '';` |
| 19 | `ALTER TABLE tasks ADD COLUMN repository TEXT NOT NULL DEFAULT '';` |
| 20 | `ALTER TABLE tasks ADD COLUMN changed_repositories TEXT NOT NULL DEFAULT '[]';` |
| 21 | `ALTER TABLE task_activities ADD COLUMN waiting_reason TEXT NOT NULL DEFAULT '';` |

Never in the baseline `CREATE TABLE`. Renumber if `main` lands a migration first.

### Project repositories

`projects.repositories` is a JSON array of remote URLs as typed. The API returns
`repositories: [{url, identity}]`, the code remote first even when it is not
stored. `repositories_migration` is empty until converted, then the JSON report
below.

### Task repository

`tasks.repository` is an identity, `""` when not pinned. A pin must match one of
the project's repositories (400 otherwise). `changed_repositories` holds the
secondary identities only; the primary is always implied.

Primary repository resolution (FR6):

1. `task.repository` when set;
2. else the only project repository, when there is one;
3. else, on a mono-repo project, the code remote (today's behaviour);
4. else, when exactly one project repository is mapped on the workstation, it,
   and the agent pins the task to it so later stages stay there;
5. else, when none is mapped, the launch fails with the mapping message (US3);
6. else ambiguous.

### Workstation mappings

`settings.json` key `repositories`: `{ "<identity>": "/abs/folder" }`. Resolution
of a repository's root: the mapping; else, for the code remote, the root
`localProjectRoot` returns today; else unmapped. A mapping is written only after
`git -C <folder> remote get-url origin` has the same identity.

### Conversion (FR5)

`GET /api/projects/{id}/legacy-repo-paths` →
`{ "migrated": false, "paths": [{ "path": "/abs", "taskIds": ["…"] }] }`, the
project's `repoPath`, `repoPaths` and every task `repo_path`, de-duplicated.

The agent resolves each path (exists, is a checkout, has `origin`) and posts:

```json
POST /api/projects/{id}/repositories/convert
{ "converted": [{ "path": "/abs/a", "url": "git@github.com:o/a.git", "taskIds": ["t1"] }],
  "dropped":   [{ "path": "/abs/b", "reason": "not found", "taskIds": ["t2"] }] }
```

The server applies it in one transaction only if `repositories_migration` is
still empty (409 otherwise): adds the URLs to `repositories`, pins the listed
tasks, clears `repo_path` / `repo_paths`, stores the body plus `convertedAt` and
the agent's user as the report. Reasons: `not found`, `not a git checkout`,
`no origin`.

Trigger: the first dispatch or desktop project listing of a mapped project whose
report is empty. Other workstations then see `migrated: true` and skip.

### Waiting for a repository (FR7)

`POST /api/activities/{id}/awaiting-repository` `{ "waiting": true, "message": "…" }`,
agent token, the run's owner only. It sets `waiting_since` and
`waiting_reason='repository'` whatever the run's mode. This is the one exception
to "a headless run is left unmarked" (`ReportRemoteRunWaitingAs`): here the answer
is a pin, not a reply in the session. `waiting: false` clears both.

The agent then polls `GET /api/tasks/{key}` every 5 s over REST (not MCP: an MCP
call from the session clears a wait), until the task is pinned to a mapped
repository (resume the same dispatch), or the dispatch context is canceled
(finish as `canceled`, "Dépôt non choisi"). The run slot is released while
waiting, and taken again on resume.

### Folder map (FR8)

Env `SECTILE_REPOSITORIES`, a JSON array:

```json
[{"remote":"git@github.com:o/a.git","identity":"github.com/o/a","role":"primary","path":"/src/a","worktree":"/src/a/.tasks/worktrees/#12"},
 {"remote":"git@github.com:o/b.git","identity":"github.com/o/b","role":"context","path":"/src/b"},
 {"remote":"git@github.com:o/c.git","identity":"github.com/o/c","role":"context","path":""},
 {"remote":"","identity":"","role":"spec","path":"/src/specs"}]
```

`role` is `primary`, `changed`, `context` or `spec`; `path` is empty when not
mapped. The prompt gets the same content as a short English block, stating that
context folders are read-only and that `prepare_repository_worktree` must be
called before changing one.

### Additional directories (FR9)

- Claude, interactive and headless: `--add-dir <path>` per mapped context folder
  and for the spec folder, shell-quoted, after the model flag.
- `{addDirs}` in a custom template expands to the same flags for `claude`, and
  to nothing for other providers.
- Codex, vibe and the others: no flag (not attested in this repository or on the
  reference workstation).

### `prepare_repository_worktree` (FR10)

MCP input `{ "taskKey": "…", "repository": "<url or identity>" }`. The server
refuses a mono-repo project, a repository outside the project's list, a task
without a branch, or the task's primary repository (it already has its
worktree). It relays `Operation{Action:"repository_worktree", TaskID,
Repository: identity, Branch}` to the caller's agent through `callAgentContext`.
The agent answers `{ "repository": "<echo>", "path": "…", "branch": "…" }` via
`ensureLocalWorktree(ctx, root, task, true)`, or an error naming the missing
mapping. On success the server adds the identity to `changed_repositories`. A
missing or different echo is refused as an agent that is too old (as #392).

### Pull request validation (FR11)

`validateStagePRs(task, stage, prURLs)`:

1. Required set: the primary repository plus `changed_repositories`.
2. Candidates: the given `prUrl` and `prUrls`, then the task's recorded `PrLinks`
   on the task branch.
3. For each required repository, the candidate whose `ParsePullRequestLink`
   identity matches; none → refusal naming the repository.
4. A given URL matching no required repository → refusal.
5. Each chosen URL goes through today's `validateStagePR` (#392 rules and head
   check, the verified checkout found through the mapping first).
6. All accepted URLs are appended with `AppendPullRequestLink`; `prUrl` becomes
   the primary repository's.

A project with no declared repositories and no changed repositories keeps
exactly today's single-URL path, so the #392 shapes are unchanged.

### Worktree removal (FR12)

`remove_workspace` gains `Repositories` (primary plus changed identities). The
agent removes the task worktree in each mapped root and answers
`{ "removed": [...], "failed": [{ "repository": "…", "error": "…" }] }`.

## Decisions

- **Mappings keyed by identity, not by project.** One checkout serves every
  project that uses its repository; `projects[id]` stays the code remote root for
  compatibility.
- **The conversion runs on a local agent**, because only a workstation can read a
  checkout's `origin` (ADR 0003); the server is often on another host or in a
  container. Compare-and-set makes the first agent win.
- **Waiting is parked in the dispatch, not a new run**, so the board keeps one
  run per launch and cancel works as today. Polling over REST avoids a new push
  message and the MCP auto-clear of waits.
- **The secondary worktree is created by the local agent, relayed by the
  server**, as ADR 0026 does for macro worktrees: the skill only talks to the
  server's MCP, and git stays local.
- **One branch name everywhere** (`task.branchName`), so the existing branch
  checks and PR evidence rules apply per repository unchanged.

## Rejected alternatives

- **Absolute paths on the server** (today's `repoPaths`): meaningless on another
  workstation, and the server cannot check them.
- **A worktree in every repository at launch**: slow and noisy for tasks that
  touch one repository.
- **The skill running `git worktree add` itself**: branch reuse, naming and the
  changed-repository record would depend on the model.
- **Re-dispatching once pinned** instead of parking: creates a second run and
  loses the launch's mode and options.
- **Enforcing read-only with permission rules**: provider-specific; decided as an
  instruction only.
