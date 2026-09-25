# Plan: #429 desktop board views

Spec: [`spec.md`](spec.md). Checklist: [`tasks.md`](tasks.md).

## Stack

Go server (`internal/db`, `internal/handlers`, `internal/models`), Go local agent
(`internal/agent`, `internal/agentconfig`), Electron desktop (`desktop/`, plain JS
renderer), React web app (`web/`).

## Architecture

The desktop renderer never calls the server. Every call goes through the local
agent's loopback surface (`/desktop/*`), which reads the server with its device
token. Views are served there by new agent routes, and a capability,
`board-views`, tells the desktop they exist.

```
renderer (main.js) ─ipc→ electron main ─http→ agent /desktop/views  ─→ server /api/me/board-views
                                             agent /desktop/tasks?viewId ─→ server /api/tasks?viewId
                                             agent /desktop/tasks POST {viewId} ─→ server /api/tasks/{id}/run-skill {viewId}
```

### Data

| Where | What | Migration |
| --- | --- | --- |
| `board_views.repository` | the view's remote URL, as entered and trimmed, `''` for none | 23 |
| `tasks.view_repository` | identity (`host/path`) of the recorded view repository | 24 |
| `tasks.view_repository_by` | user ID of whoever launched with it; reaches their agent for GitLab | 24 |
| `settings.json` `viewDirectories` | view ID → local folder, workstation only | none |

`tasks.repository` (the #456 pin) is not reused: it must be one of the project's
repositories and a mono-repo project ignores it, while a view repository may be
any repository.

`models.BoardView` gains `Repository string json:"repository"`, and
`BoardViewRequest` gains `Repository *string`. `models.Task` gains
`ViewRepository string json:"viewRepository,omitempty"`; the launching user is
never serialized and is read only by discovery. `models.RunSkillRequest` gains
`ViewID string json:"viewId,omitempty"`.

### Server

- `internal/db/boardviews.go`: read and write the column. On update, a nil
  `Repository` keeps it. It is validated by `models.RepositoryIdentity` having a
  host and a path (`validViewRepository`), and refused with
  `ErrBoardViewRepositoryInvalid` (400 in `writeBoardViewError`).
- `internal/db/viewrepository.go` (new): `RecordTaskViewRepository(taskID,
  identity, userID)` and `taskViewRepositoryUser(taskID)`.
- `internal/handlers/handlers.go` run-skill: with a `viewId`, resolve the view
  for the session user (`ErrBoardViewNotFound` → 404), refuse a view that does not
  select the task's project (400), and record the view's repository once the run
  is admitted (after the busy check, so a refused launch leaves no trace).
- Evidence (`internal/db/adjustment.go`, `internal/db/stageprs.go`):
  - `resolveStagePRTarget(project, task, url)` takes the task. A link whose
    identity is `task.ViewRepository` is foreign even on a mono-repo project; an
    empty URL on a task with a recorded view repository that is not the
    project's own resolves to `repositoryTarget(view)`.
  - `evidencePrimary(project, task)` is the recorded view repository when there
    is one, else `TaskPrimaryRepository`. A task with a recorded view repository
    and no changed repository takes the single-pull-request path, whatever its
    #456 pin. With changed repositories, the view repository replaces the primary
    repository in the required set.
  - `adjustmentPrerequisite` passes the task, and uses `evidencePrimary`.
- Discovery (`internal/db/prdiscovery.go`): `discoverViewRepositoryPullRequest`
  runs under the same gate for a task with a recorded view repository and a
  branch. It calls `lookupStagePR` with `repositoryTarget(view)` as the recorded
  user (GitHub on the server, GitLab through that user's agent) and returns the
  pull request as a discovered link. It runs whether or not the tracker can
  discover, and a failure is a warning step, never a halt.
  `rediscoverProjectPullRequests` no longer returns early when the tracker
  cannot discover.

### Agent

- `internal/agentconfig/local.go`: `Overrides.ViewDirectories map[string]string`,
  merged in `ReadSettings`, written by `WriteSettings` (and nulled when empty, as
  the other maps).
- `internal/agent/agent_desktop_views.go` (new):
  - `GET /desktop/views`: the server's views with `directory` from settings.
  - `POST /desktop/views {viewId, path}`: validates the checkout (`rev-parse
    --show-toplevel`), compares its `origin` with the view repository, saves or
    clears, and answers `{directory, warning}`.
- `desktopTasks`:
  - GET accepts `viewId` instead of `projectId`.
  - POST accepts `ViewID`. It resolves the view root (FR3 order), validates
    against it instead of the project mapping, forwards `viewId` to run-skill,
    and records the root for the task (`d.viewRoots`, task ID → folder, in
    memory). A launch without a view forgets it.
- `prepareDispatchLocked` and `admitProjectRun` resolve the root through
  `taskProjectRoot`: the recorded view root when there is one, then
  `localProjectRoot`. With a view root, the #456 primary-repository resolution is
  skipped: the view root is the primary folder.
- The `board-views` capability is added to `/desktop/status`.

### Desktop

- `electron/main.cjs` and `preload.cjs`: `views()`, `viewTasks(viewId, q,
  launchable)`, `setViewDirectory(viewId, path)`, and a `viewId` argument on
  `launchServerTask`. Each is gated on the capability.
- `src/main.js`:
  - `openTicketsFromPalette` lists views after projects when `api.views()`
    answers, and falls back to projects only on any error.
  - The Tickets pane takes a scope (`{projectID}` or `{view}`) and keeps one
    project info per project (`view.infos`). Rows read `task.projectId`, launch
    with their own project and the view ID, and show the project name in a
    column.
  - The header of a view's pane shows its directory and a "Local directory…"
    action that uses the existing folder picker, then shows the warning if any.
  - A run launched from a view is remembered in memory (`viewLaunches`, task ID
    → view ID), so that relaunch and next step send the same view.

### Web

- `web/src/types/index.ts`: `repository` on `BoardView` and `BoardViewPayload`.
- `web/src/context/AppContext.tsx` `boardViewRequest`: sends `repository`.
- `web/src/components/BoardViewModal.tsx`: an optional "Dépôt Git" field (the
  web interface speaks French), with client validation through a
  `boardViewFormError` rule in `web/src/lib/boardViews.ts`.

## Rejected alternatives

- **Reusing the #456 pin (`tasks.repository`)**: it is refused outside the
  project's repositories and ignored on mono-repo projects, which are the main
  case here.
- **Sending the view directory from the renderer on every launch**: the server
  chains stages without the desktop, so the root has to live on the agent.
- **Persisting the per-task view root across agent restarts**: it would grow
  without bound. After a restart, a relaunch from the desktop still sends its
  view, and only a server-chained step falls back to the project root.
- **Discovering by branch in the project repository as well**: that is a separate
  change to today's issue-reference discovery, and #429 does not ask for it.

## Target files

`internal/db/migrations.go`, `internal/db/migrations_test.go`,
`internal/db/activerun_test.go`, `internal/db/boardviews.go`,
`internal/db/viewrepository.go`, `internal/db/db.go`,
`internal/db/adjustment.go`, `internal/db/stageprs.go`,
`internal/db/prdiscovery.go`, `internal/models/models.go`,
`internal/handlers/boardviews.go`, `internal/handlers/handlers.go`,
`internal/agentconfig/local.go`, `internal/agentconfig/settings.go`,
`internal/agent/agent_desktop.go`, `internal/agent/agent_desktop_views.go`,
`internal/agent/agent_config.go`, `internal/agent/agent_run.go`,
`desktop/electron/main.cjs`, `desktop/electron/preload.cjs`,
`desktop/src/main.js`, `web/src/types/index.ts`,
`web/src/context/AppContext.tsx`, `web/src/components/BoardViewModal.tsx`,
`web/src/lib/boardViews.ts`, `CHANGELOG.md`.
