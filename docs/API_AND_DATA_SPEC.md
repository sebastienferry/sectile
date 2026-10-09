# API & Data Specifications

This document defines the SQLite schema, domain data models, REST endpoints, and WebSocket streaming protocols for **Sectile**.

---

## 1. SQLite Database Schema

The database file is located at `tasks.db` in the server root. Foreign keys and WAL journal mode are enabled on connection.

```sql
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- Projects Table
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    icon TEXT DEFAULT 'folder',
    color TEXT DEFAULT 'indigo',
    repo_path TEXT NOT NULL DEFAULT '.',
    repo_paths TEXT NOT NULL DEFAULT '[]',  -- legacy working directories, converted to repositories once per project (#456)
    repositories TEXT NOT NULL DEFAULT '[]',  -- remotes the project's tickets work in, besides the code remote (migration 17)
    repositories_migration TEXT NOT NULL DEFAULT '',  -- JSON report of the legacy path conversion, empty until done (migration 18)
    git_remote_url TEXT DEFAULT '',
    github_repo TEXT DEFAULT '',
    -- Per-project connection overrides. Empty falls back to the settings row,
    -- then to the server environment. Tokens are never returned by the API.
    github_api_url TEXT NOT NULL DEFAULT '',
    github_token TEXT NOT NULL DEFAULT '',
    gitlab_url TEXT NOT NULL DEFAULT '',
    gitlab_project TEXT NOT NULL DEFAULT '',
    gitlab_token TEXT NOT NULL DEFAULT '',
    jira_project TEXT DEFAULT '',      -- Legacy Jira project identifier
    issue_tracker TEXT NOT NULL DEFAULT 'local',  -- 'github' | 'gitlab' | 'jira' | 'local'
    tracker_url TEXT DEFAULT '',       -- tracker project URL, or the Jira base URL
    is_default INTEGER DEFAULT 0,
    stage_mapping TEXT DEFAULT '{}',  -- unused: kept so older binaries still open the base
    skill_overrides TEXT DEFAULT '{}',
    ai_provider TEXT DEFAULT '',
    ai_command_template TEXT DEFAULT '',            -- interactive launches
    ai_command_template_autonomous TEXT DEFAULT '', -- headless launches; empty falls back to the line above
    spec_framework TEXT DEFAULT '',    -- 'speckit' | 'openspec'
    default_skill_mode TEXT NOT NULL DEFAULT '',            -- '' (interactive) | 'interactive' | 'autonomous'
    full_chain_stop_stage TEXT NOT NULL DEFAULT 'reviewed', -- 'implemented' | 'reviewed'
    push_stage_commits INTEGER NOT NULL DEFAULT 0, -- boolean pushStageCommits in project/config/context JSON
    branch_name_format TEXT NOT NULL DEFAULT '', -- branchNameFormat in project/config/context JSON; empty = feat/{key_lower}
    -- Since #741 (migration 49, ADR 0054) the tracker columns above (issue_tracker,
    -- jira_project, github_repo, gitlab_project, board_id, tracker_columns,
    -- stage_columns, sprints, issue_types, auto_sync_*) are no longer the source:
    -- a project is read with its default tracker's values, and the columns are
    -- kept for the adoption step and a rollback until a later migration drops them.
    label TEXT NOT NULL DEFAULT '',               -- narrows the project to the tickets carrying it; empty = every ticket of its trackers
    default_tracker_id TEXT NOT NULL DEFAULT '',  -- where new tickets go; empty or unlinked = the first tracker
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tasks Table
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL DEFAULT 'default',  -- unread since #741: rows written since carry the sentinel 'tracker:<tracker id>'
    tracker_id TEXT NULL,                       -- the tracker the ticket belongs to (migration 49); unique with key (ux_tasks_tracker_key)
    key TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'to_clarify',
    priority TEXT NOT NULL DEFAULT 'medium',
    labels TEXT DEFAULT '[]',
    assignee TEXT DEFAULT '',
    assignee_avatar TEXT DEFAULT '',
    position INTEGER DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'local',
    external_id TEXT DEFAULT '',
    external_url TEXT DEFAULT '',
    branch_name TEXT,
    pr_url TEXT,                          -- the task's current pull request: always the last entry of pr_links
    pr_links TEXT NOT NULL DEFAULT '[]',  -- ordered set of [{url, branch}], oldest first; one ticket routinely produces several PRs
    repo_path TEXT NOT NULL DEFAULT '',  -- legacy per-ticket CWD, ignored by the agent and converted once per project (#456)
    repository TEXT NOT NULL DEFAULT '',  -- identity of the repository the ticket is pinned to, one of its project's (migration 19)
    changed_repositories TEXT NOT NULL DEFAULT '[]',  -- secondary repositories the ticket has a worktree in (migration 20)
    worktree_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    due_date DATETIME,
    FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

-- Task Activities Table (Background Jobs & Skill Runs)
CREATE TABLE IF NOT EXISTS task_activities (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    skill_name TEXT NOT NULL,
    action TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    summary TEXT DEFAULT '',
    output TEXT DEFAULT '',
    steps TEXT DEFAULT '[]',
    prompt TEXT DEFAULT '',
    error TEXT DEFAULT '',
    duration TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    run_project_id TEXT NULL,  -- the project a task run works for (#741); empty on other rows
    tracker_id TEXT NULL,      -- the tracker a synchronisation read (#741); empty on other rows
    FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_activities_task_id ON task_activities(task_id);
CREATE INDEX IF NOT EXISTS idx_activities_status ON task_activities(status);

-- Global Settings Table
CREATE TABLE IF NOT EXISTS settings (
    id TEXT PRIMARY KEY,
    ai_provider TEXT DEFAULT 'antigravity',
    -- Tracker connection parameters. They are typed in the interface and win
    -- over the server environment variables, which stay as a headless fallback.
    github_api_url TEXT NOT NULL DEFAULT '',
    github_token TEXT NOT NULL DEFAULT '',
    github_repo TEXT DEFAULT '',
    gitlab_url TEXT NOT NULL DEFAULT '',
    gitlab_project TEXT NOT NULL DEFAULT '',
    gitlab_token TEXT NOT NULL DEFAULT '',
    jira_project TEXT DEFAULT '',      -- default Jira project key
    jira_url TEXT DEFAULT '',          -- default Jira base URL
    jira_api_token TEXT NOT NULL DEFAULT '',
    spec_framework TEXT DEFAULT 'speckit',  -- 'speckit' | 'openspec'
    repo_path TEXT DEFAULT '.',
    auto_create_branch INTEGER DEFAULT 1,
    auto_create_worktree INTEGER DEFAULT 1,
    theme TEXT DEFAULT 'dark',
    accent_color TEXT DEFAULT 'indigo',
    language TEXT DEFAULT 'fr',
    detail_mode TEXT DEFAULT 'panel',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Saved board views (migration 4): a personal selection of projects and labels
-- laid over the all-projects board. Both lists are JSON arrays; name_key is the
-- trimmed, lower-cased name that keeps names unique per owner.
CREATE TABLE IF NOT EXISTS board_views (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    name_key TEXT NOT NULL,
    project_ids TEXT NOT NULL DEFAULT '[]',
    labels TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_board_views_user_name ON board_views (user_id, name_key);

-- Trackers (migration 49, #741, ADR 0054): one server-side source of tickets,
-- a Jira space, a GitHub repository, a GitLab project, or the local board of
-- one project. It holds the board mirror and the background sync settings.
CREATE TABLE IF NOT EXISTS trackers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL,                  -- 'jira' | 'github' | 'gitlab' | 'local'
    site TEXT NOT NULL DEFAULT '',           -- the tracker's own site, empty = the deployment's
    scope TEXT NOT NULL DEFAULT '',          -- Jira key, GitHub owner/repo, GitLab path, or the project id of a local board
    identity TEXT NOT NULL UNIQUE,           -- derived from provider, resolved site and scope
    board_id TEXT NOT NULL DEFAULT '',
    tracker_columns TEXT NOT NULL DEFAULT '[]',
    stage_columns TEXT NOT NULL DEFAULT '{}', -- the default status-to-stage mapping, an admin's
    sprints TEXT NOT NULL DEFAULT '[]',
    issue_types TEXT NOT NULL DEFAULT '[]',
    auto_sync_enabled INTEGER NOT NULL DEFAULT 0,
    auto_sync_interval_min INTEGER NOT NULL DEFAULT 5,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- The trackers a project selects its tickets from, in order.
CREATE TABLE IF NOT EXISTS project_trackers (
    project_id TEXT NOT NULL,
    tracker_id TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    -- The project's own stage -> columns mapping for the tracker (migration 50);
    -- '{}' reads the tracker's stage_columns.
    stage_columns TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (project_id, tracker_id)
);
CREATE INDEX IF NOT EXISTS idx_project_trackers_tracker ON project_trackers (tracker_id);

-- The id of a ticket merged away by the adoption step, and the ticket it now is.
CREATE TABLE IF NOT EXISTS task_aliases (
    old_id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL
);

-- The background sync pacing, per tracker (the per-project auto_sync_projects is no longer read).
CREATE TABLE IF NOT EXISTS auto_sync_trackers (
    tracker_id TEXT PRIMARY KEY,
    last_pass_at DATETIME,
    last_full_sync_at DATETIME
);

-- macros gains tracker_id TEXT NULL: a Jira epic is a record of its tracker.

-- Created at start by the idempotent adoption steps, after the duplicates are merged:
-- CREATE UNIQUE INDEX ux_tasks_tracker_key ON tasks (tracker_id, key);
-- CREATE UNIQUE INDEX ux_macros_tracker_key ON macros (tracker_id, key) WHERE tracker_id IS NOT NULL;
```

---

## 2. REST API Endpoints

### 2.1 Tasks API

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/tasks` | Returns array of all tasks. Filters: `projectId` (the project's members, see below), `q`, `status`, `priority`, `label`, `sprint`, `team`, `assignee` (`__unassigned__` for the work items nobody owns), `mine=1` (My Tasks: the tickets assigned to the caller, resolved on the server per tracker, see 2.3.1; combines with every other filter), `pinned=1`, `viewId` (one of the caller's saved views, see 2.3.0.2: it replaces `projectId`, and the other filters narrow it; `404` when the view is not the caller's). |
| `GET` | `/api/tasks/facets` | Filter values and counts of the board. Scope: `projectId`, or `viewId` as above. |
| `POST` | `/api/tasks` | Creates a new task bound strictly to `projectId`, on one of the project's trackers: `trackerId` (id or identity), else the project's default tracker. The labels get `#new` and, when the project has one, the project label. A `trackerId` that is not one of the project's answers `400`. |
| `GET` | `/api/tasks/{id}` | Fetches task detail with its activities. `{id}` may be the id of a ticket merged away at the upgrade to #741, resolved through `task_aliases`, or a key; a key that tickets of two trackers carry is refused rather than resolved to the first. |
| `PUT` | `/api/tasks/{id}` | Updates task fields (status, title, description, priority, etc.). |
| `DELETE` | `/api/tasks/{id}` | Deletes task and prunes associated Git worktree. |
| `POST` | `/api/tasks/{id}/skills/{skillId}` | Enqueues or immediately executes an AI skill on the task. |
| `POST` | `/api/tasks/{id}/run-skill` | Runs a skill on the task. Body `{skillId, prompt?, withComments?, mode?, force?, batchTaskIds?, projectId?}`. `projectId` is the project the run works for, the board's it was launched from; it must be one of the ticket's projects (`400` otherwise), it is recorded on the run (`runProjectId`) and the run's configuration, repositories and operations are that project's. Without it, a ticket of one project runs for that project; a ticket of several answers `409 {error, candidates: [{id, name}], unattended: false}` when the launch is interactive, so the caller asks which project and retries with it, and `400` with the same body and `unattended: true` when it is unattended (`mode: "autonomous"` or `batchTaskIds`; the project's default mode does not count); a ticket of no project answers `400` and has to be given a project's label first. Nothing is recorded before that choice. `mode` is the one-off execution mode override, `interactive` or `autonomous`; absent means no override, which is not the same as interactive. Any other value is rejected with `400`. A task already carrying an active run answers `409` with `{error, activeRunId}` and records nothing; `force` waives that refusal and only that one, for the active run's owner or an admin, and answers `403` for anyone else. A ticket of a running batch is busy the same way, and its `409` adds `batchLeadKey`. `batchTaskIds` launches a batch: `pickup_issues` only, at least two distinct tickets of the task's project, the first being the task itself, without `force`, else `400`; a ticket already busy refuses the whole batch with `409`. The members are recorded on the batch run and exposed as `batch` on each task while it runs (ADR 0034). |
| `POST` | `/api/tasks/{id}/advance` | Advances one workflow step, or the full chain with `{"auto": true}`. Body also accepts `mode`, the one-off override for the single step; a full chain run ignores it and is always autonomous. Body also accepts `projectId`, with the same rules and refusals as `run-skill`; a full chain is unattended. |
| `POST` | `/api/tasks/{id}/advance/confirm` | Closes an interactive step. A step the worker already transitioned is accepted as a no-op. |
| `POST` | `/api/tasks/{id}/comment` | Publishes a comment to the GitHub issue tracker. |
| `POST` | `/api/tasks/{id}/epic` | Queues the attachment to an epic (`202`, returns the activity to follow). |
| `GET` | `/api/tasks/{id}/diff` | Computes and returns the Git diff of the task branch vs `main`. |

**Tickets belong to trackers, and to projects by membership (#741, ADR 0054).**
There is one local issue per remote ticket, owned by its tracker, whatever projects show it.
A task carries `trackerId`, and `projectIds`, the projects it belongs to: those
selecting its tracker that have no label, or whose label it carries, compared
whole and regardless of ASCII case. `projectId` is computed: the project a
listing is scoped to, else the first of `projectIds`, empty for a ticket in no
project. A listing with `projectId` returns the project's members; without it,
the members of the caller's bookmarked projects, each ticket once. The `label`
filter still matches a part of a label, folded for ASCII case.

### 2.1.1 Teams API

A work item may carry a team, and it is never mandatory: a project can hold
tickets with no team at all. Team refresh reads the members from the project's
own tracker, for the trackers that have teams; a tracker without them answers an
explicit unsupported-capability error.

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/teams` | Teams carried by the project's work items (`?projectId=...&members=1`). |
| `GET` | `/api/teams/members` | People of one team, by its label (`?team=<name>`). |
| `GET` | `/api/teams/workload` | The team's work items grouped per person, plus the unassigned ones and the ones owned outside the team (`?projectId=...&team=<name>`). |
| `POST` | `/api/teams/refresh` | `{projectId, teamId}` re-reads one team's members from the tracker. |

### 2.2 Activities API

Every write on an existing work item goes through this queue: field sync,
assignment, epic attachment, epic split, roadmap horizon, epic priority,
epic quarter and epic readiness labels, and the free labels of an epic (`POST
/api/projects/{id}/macros/{key}/labels` with `{add, remove}`; a label of one of
the roadmap's axes is refused with `400`). A tracker call
takes seconds and a batch of them far longer, so the HTTP endpoints answer `202`
with the activity to follow, and the activity's steps carry what was attempted
and the tracker's own refusal when it fails.

An epic of one of a Jira project's roadmap projects (#632) is read with the
project's own epics and returned by `GET /api/projects/{id}/macros` with
`foreign: true` and its `origin` key. Nothing is queued for its horizon or its
free labels. Its priority and quarter are queued only when the project's
`roadmapAxisWrites` is on and the edit names one epic: a macro save carrying
`"bulk": true`, as the title seeding sends, keeps them in Sectile. The macro's
`axesWritable` says which applies. A slicing line whose `targetTrackerProject`
names a roadmap project creates its story in that Jira project, and the answer's
`task` carries no `id`: the story is not imported.

`POST /api/projects/{id}/macros/{key}/slicing` produces a macro's slicing from
`source` (`tasks`, `spec` or `stories`). For `tasks` and `spec`, the requesting
user's local agent reads the file in the workstation's specifications folder,
unless the body carries `content`, a file the user picked in the browser, with
its `fileName` (#735): the server then slices that text without asking the
agent, and the answer's `origin` reads `imported file: <fileName>`. Uploaded
content is refused with `400` above 1 MiB, when it is not UTF-8 text or holds a
NUL character, and with any source other than `tasks` or `spec`. An absent
body reads `tasks` through the agent; a body that is not valid JSON is refused.

The todos of a macro are copied on its tracker, one way (#663, ADR 0046): a
comment on a Jira epic, a block at the end of a GitHub milestone description.
Every save of the list queues that copy a few seconds after the last save, as an
`epic_todos` activity; `POST /api/projects/{id}/macros/{key}/todos-mirror`
queues one at once and answers `202` with the activity, or `400` with the reason
when the macro's list stays in Sectile (GitLab, local project, local key,
roadmap project's epic). Every macro the API returns carries `todosMirror`:
`kind` (`jira_comment`, `github_description`, or empty with a `reason`),
`upToDate`, the last `error` and the `credentialMissing` tracker it lacked a
token for, `writtenAt` and the `url` of the copy.

The framing of a Jira epic is copied the same way, as a second comment Sectile
owns (#636): a macro save carrying `framingComment`, unless it carries
`"bulk": true`, queues an `epic_framing` activity a few seconds after the last
save; `POST /api/projects/{id}/macros/{key}/framing-mirror` queues one at once
and answers `202`, or `400` with the reason when the framing stays in Sectile
(GitHub milestone, GitLab, local project, local key, roadmap project's epic).
An empty framing never creates the comment, and rewrites an existing one to say
there is none. Every macro the API returns carries `framingMirror`, shaped as
`todosMirror`, whose `kind` is `jira_comment` or empty.

`GET /api/projects/{id}/macros/framing-mirror` lists the macros whose framing
copy is missing or late, from local state only (#691), and `POST` on the same
path republishes them all: it queues one `epic_framing_bulk` activity, signed
by the caller, that writes them one after another without forcing a body
already written, and answers `202` with `{queued, skipped, activity}`.
`skipped` counts the copied epics whose framing is empty and was never
copied, for which no comment is created. With nothing pending it answers
`200` with `queued: 0` and no activity. Each failure is stored on its macro's
`framingMirror` and listed in the activity, which fails only when no epic was
written.

A project's `epicAxisPrefixes` (`{priority, quarter, readiness}`, #635) names
the label prefixes its epics carry each axis under; an empty field is the
default `priority:`, `quarter:` or `readiness:`. The import, the pushes, the
pending labels and the free label refusal all follow them, the epics of the
roadmap projects included; `roadmap:` and the bare `2026-Q3` stay fixed. A
`PATCH /api/projects/{id}` or a creation stores them trimmed, lower-cased and
without a leading `#`, and answers `400` with the reason for a prefix carrying
a space, emptied by that cleaning, overlapping another axis's prefix or
overlapping `roadmap:`. Changing one rewrites no label and no stored value.

A Jira project's `priorityMapping` (#679) holds how its ticket priority scheme
maps to Sectile's levels: `options`, in the scheme's order, each with its `id`,
`name`, `level`, and `guessed` (matched by rank only) or `manual` (set by a
person), and `preferred`, the option a level sends when several sure ones carry
it. It is empty until the first discovery, which runs on a full synchronisation
and on `POST /api/projects/{id}/priority-mapping/refresh` (a fresh read; answers
the project). A discovery adds new options and drops gone ones, never changing
a stored line. A `PATCH /api/projects/{id}` carrying `priorityMapping` changes
levels and preferred options only, makes each changed or confirmed line sure
and manual, and answers `400` for an option or a level the mapping does not
carry. Once a mapping exists, `PUT /api/tasks/{id}` changing the priority to a
level no sure line carries answers `422` with the accepted levels, writing
nothing; a creation goes out without that priority and its answer carries a
`priorityNotice`. MCP `update_task` and `create_task` behave alike.


| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/activities` | Lists recent activities (supports `?taskId=...&status=...`). |
| `GET` | `/api/activities/stats` | Returns aggregate counts (`total`, `queued`, `running`, `completed`, `failed`). |
| `POST` | `/api/activities/{id}/retry` | Re-enqueues a failed activity, for the project it ran for. Optional body `{projectId?}` names that project instead, with the same rules and refusals as `run-skill`: an activity that names no project, on a ticket of several, answers `409 {error, candidates, unattended: false}` until it is retried with one of them. |
| `POST` | `/api/activities/{id}/cancel` | Cancels a running or queued activity. |
| `DELETE` | `/api/activities/{id}` | Deletes an activity entry. |
| `DELETE` | `/api/activities` | Clears all completed and canceled activities. |

### 2.3 Projects API

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/projects` | Lists all projects with their task counters, counted over their members. |
| `POST` | `/api/projects` | Creates a new workspace project. |
| `PUT` | `/api/projects/{id}` | Updates project configuration, tracker selection and label, and paths. |
| `DELETE` | `/api/projects/{id}` | Deletes the project. The tickets of its own local tracker move to the default project; the tickets of the trackers it shared stay, shown by the other projects selecting them, else in their tracker's backlog. |
| `POST`, `PATCH`, `DELETE` | `/api/projects/{id}/sprints[/{sprintId}]?trackerId=` | Creates, changes or deletes a sprint on one of the project's trackers, named by id or identity (`400` for another), its default tracker without `trackerId`. |
| `POST` | `/api/projects/{id}/skills/install` | Installs default skills into `.gemini/` and `.agents/`. |
| `GET` | `/api/projects/{id}/skills-status` | Reports which workflow skills are scaffolded, per worktree. |
| `POST` | `/api/projects/{id}/install-skills` | Scaffolds the workflow skills into the repo and all its worktrees. |
| `POST` | `/api/projects/{id}/init-git` | Initializes a Git repository in the project working directory. |
| `PUT` | `/api/projects/{id}/skill-editor/{skillId}/mode` | Pins the skill's execution mode for this project. Body `{mode}`: `interactive`, `autonomous`, or empty to clear it and fall back to the project default. |
| `GET` | `/api/projects/{id}/spec-framework-status` | Per-framework SDD status for this project (see 2.5). |
| `POST` | `/api/projects/{id}/install-spec-framework` | Installs a SDD toolchain for this project (see 2.5). |

**What a project selects (#741, ADR 0054).** A project carries `trackers`, the
ordered list of `{trackerId, identity}` it selects its tickets from, `label`,
which narrows it to the tickets carrying it (empty shows every ticket of its
trackers), and `defaultTrackerId`, where its new tickets go. The creation and
the update accept the three: a tracker is named by id or identity (`400` for
one nobody recorded), duplicates are dropped, the list order is kept, and a
default that is not one of the trackers falls back to the first. The local
board of another project is refused with `400`. A creation must name a
recorded tracker: in `trackers`, or through its legacy tracker fields
(`issueTracker`, `jiraProject`, `githubRepo`, `gitlabProject`, …), which only
find a tracker by identity (`400` "tracker inconnu" when none matches). A
creation naming neither, or only the local board, is refused with `400`: no
project write from the API creates a tracker, not even a local board. An update
without `trackers` keeps the project's trackers; one whose legacy fields name a
provider or a scope joins the recorded tracker of that identity, and is refused
with `400` when there is none. The site fields alone (`trackerUrl`,
`githubApiUrl`, `gitlabUrl`) never move a project to another tracker, and the
code remote (`gitRemoteUrl`) names no tracker: no GitHub repository is derived
from it.

The tracker fields a project still returns (`issueTracker`, `boardId`,
`trackerColumns`, `sprints`, `issueTypes`, `autoSync*`, …) are read from its
default tracker; `stageColumns` is the mapping that applies to that tracker's
tickets in the project, the project's own else the tracker's. Each entry of
`trackers` also returns, read only, the tracker's `trackerColumns`, its own
mapping `trackerStageColumns`, the mapping that applies in the project
`stageColumns`, and `ownStageColumns` when that one is the project's. A tracker
is configured by an admin (2.3.0.3): a member's creation or update carrying
`boardId`, `trackerColumns`, `issueTypes`, `autoSyncEnabled` or
`autoSyncIntervalMin` is saved without them, and answers as if they were not
sent. An admin's still writes them to the project's default tracker. `sprints`
stays a member's.

The stage mapping is the project's own, per tracker (#741, ADR 0054), and a
member's to write as an admin's. An update may carry `trackerStageColumns:
{trackerId: {stage: [column]}}`: each tracker named gets that mapping, an empty
one going back to the tracker's, a tracker left out keeping its own. A stage
outside the six workflow stages, a column the tracker does not have or a
tracker the project does not select is refused with `400`. The legacy
`stageColumns` sets the default tracker's, leniently: the stages and columns the
tracker lacks are dropped, and a mapping then empty or equal to the tracker's
keeps the tracker's. Neither ever changes the tracker's own mapping.

Which mapping a ticket's stage follows is ADR 0054's rule: the project in
context when it selects the ticket's tracker, else the ticket's single project,
else the tracker's. A stage or column move names its context:
`POST /api/tasks/{id}/stage` and `POST /api/tasks/{id}/tracker-status` take an
optional `projectId`, and `PUT /api/tasks/{id}` an optional `stageProjectId`,
which, unlike `projectId`, moves nothing. The web sends the board's project when
the ticket belongs to it.

### 2.3.0 Current Account API

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/me` | (none) | Who is signed in, the sign-in mode and the role. Public: it is what the interface asks before anyone is signed in. |
| `PATCH` | `/api/me` | `{displayName}` | Renames the calling account and answers the same body as the `GET`. The account comes from the session, never from the payload, so no route renames another one. `401` without a session, `400` above 80 characters or on a line break. An empty name clears the choice and hands the account back to the name its sign-in supplies. |
| `GET` | `/auth/login?redirect=` | (none) | Starts a sign-in and comes back to `redirect`, a same-site path. With an identity provider, `302` to the provider (authorization code with PKCE, ADR 0008); without one, `302` to the interface's `/signin?redirect=` page for the local sign-in. |
| `GET` | `/auth/callback` | (query `state`, `code` or `error`) | Where the identity provider sends the browser back: opens a session, sets its cookie and redirects to the `redirect` given at the start. `400` on an unknown or expired `state`, `401` when the provider refused, `403` for a blocked account, `404` without a provider. |
| `POST` | `/auth/local` | `{email}` | The local sign-in of a deployment without an identity provider: opens a session for that e-mail and answers the account like `GET /api/me`. `400` on an invalid address, `403` for a blocked account, `404` when a provider is configured. |
| `GET` | `/auth/workstation?port=&state=` | (none) | The browser sign-in of a workstation (#717, ADR 0049): with a session, `302` to `http://127.0.0.1:<port>/callback?code=&state=` with a fresh pairing code the workstation redeems on `POST /api/v1/agent/pair`; without one, `302` to `/auth/login` and back. `400` on a port outside 1024-65535 or a malformed `state`, `403` for a blocked account. Only the session cookie counts, never a bearer key. See the [server/agent contract](contracts/server-agent-v1.md#workstation-api-keys-and-identity). |

A web session lasts at most 90 days from sign-in and ends sooner after 7 days
without use, counted from its last use (`web_sessions.last_seen_at`, or
`created_at` for a session never used). The session cookie's `Max-Age` is the
90 days; the idle rule is enforced on the server, so a session left idle keeps
a cookie that resolves to nobody and the interface asks to sign in again.
Signing out revokes the session at once.

The chosen name lives in `users.chosen_name`, not in `users.display_name`: the
latter is rewritten at every sign-in from the provider's claim, or from the
e-mail address for a local account, so a chosen name stored there would be
erased at the next visit. Every read of a user resolves
`COALESCE(NULLIF(chosen_name, ''), display_name)`.

### 2.3.0.1 Accounts API

The roster, the admin side of the interface. Everything else on the board,
projects included, is a member's to use. What is an admin's is the roster:
who exists, what role they hold, and whether their account still opens, the
admin page's summary of what the board is doing, and the deployment's tracker
access: the server credential of each provider, the Jira OAuth app (2.3.1) and
the trackers themselves (2.3.0.3).

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/users` | (none) | Every account with its role, its last sign-in, whether it is blocked, `lastActiveAt` (when one of its browser sessions last reached the server, absent when never) and `active` (seen within the active window, five minutes), plus `rolesFromProvider` when the identity provider supplies the roles. |
| `GET` | `/api/admin/stats` | (none) | `{users: {total, admins, blocked, active}, runs: {active, byStatus: {running, queued, pending}}, activeWindowSeconds, generatedAt}`. `active` counts the accounts with an unexpired, unrevoked session seen within the window; runs are the activities `activeRunPredicate` selects. Read from the database, so every server sharing it answers the same. |
| `PUT` | `/api/users/{id}` | `{role?, blocked?}` | Changes the role, the blocked state, or both. An absent field is left alone. `400` on neither field and on a role that is not `admin` or `member`; `404` on an unknown account; `409` on the last admin, on blocking or deleting your own account, and on the implicit account. |
| `DELETE` | `/api/users/{id}` | (none) | Removes the account, its sessions and its workstation keys. Same refusals as above. The tasks, comments and executions it owns stay on the board and read as having no owner. |

Blocking keeps everything the account owns and only closes the door: the open
sessions are revoked at once, the workstation keys stop authenticating, and the
next sign-in answers `403`. Unblocking gives all three back. Deleting is the
irreversible one, and is why the two are separate actions rather than a switch.

### 2.3.0.2 Saved Board Views API

A saved view is a named selection of projects and labels over the
all-projects board, and belongs to the account that created it. A ticket
belongs to a view when it sits in one of its projects and carries **at least
one** of its labels, compared whole and regardless of case (`Backend` matches
`backend`, not `backend-api`). A view without labels selects every ticket of
its projects. Another account's view answers exactly as a missing one.

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/me/board-views` | (none) | The caller's views, in creation order. |
| `POST` | `/api/me/board-views` | `{name, projectIds, labels}` | Creates a view (`201`). `400` for an empty name, no project, an unknown project, a name over 80 characters or more than 50 labels; `409` when another of the caller's views has the same name, compared trimmed and case-insensitively. Labels are trimmed and deduplicated regardless of case. |
| `GET` | `/api/me/board-views/{id}` | (none) | One view, or `404`. |
| `PATCH` | `/api/me/board-views/{id}` | any of `{name, projectIds, labels}` | Changes the fields sent; same errors as the creation. |
| `DELETE` | `/api/me/board-views/{id}` | (none) | Deletes the view (`204`); no ticket, label or project changes. |

Deleting a project removes it from every view that selected it; a view left
without project is kept and selects nothing until it is edited.

A view's projects select by membership (#741): a ticket sits in one of them
when the project selects its tracker and has no label or the ticket carries it.
The view's scope is the union of its projects' memberships, each ticket once,
and the view's own labels narrow it further.

### 2.3.0.3 Trackers API

A tracker is one server-side source of tickets (#741, ADR 0054): a Jira space,
a GitHub repository, a GitLab project, or the local board of one project. It is
synchronised in full whatever projects exist, holds the board mirror (board,
columns, default status-to-stage mapping, sprints, issue types) and the
background sync settings, and projects select their tickets from it (2.3). A
project may map the stages onto the tracker's columns its own way (2.3). A
local board is its project's own: no route below lists it.

**Member routes.** Any signed-in account. A member reaches a tracker's routes
when at least one project selects it (every member sees every project); an
admin always does.

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/trackers` | (none) | The trackers to pick from: `[{id, name, provider, site, scope, identity, trackerColumns?, stageColumns?}]`, with, read only and for a tracker a project selects (any tracker for an admin), the columns a project maps its stages onto and the tracker's own mapping; the rest of the board mirror is left out. |
| `POST` | `/api/trackers/{id}/sync` | (none) | Queues a synchronisation of the tracker: `202 {queued, activity}`. |
| `GET` | `/api/trackers/{id}/backlog` | (none) | The tracker's open tickets that no project shows: a ticket whose workflow stage is finished, by its workflow label or by its tracker status through the tracker's own stage mapping, is left out. Empty while a project without label selects the tracker. |
| `POST` | `/api/trackers/{id}/backlog/{taskId}/project` | `{projectId}` | Gives the ticket the project's label: the ticket joins the project at once, and the label write on the tracker is queued with the caller's own credential. `200 {task, activity}`; `404` when the ticket is not one of the tracker's; `409` when the project does not select the tracker or has no label. |

`404` on an unknown tracker, `403` for a member when no project selects it.

**Admin routes.** In the admin-only table: `401` without a session, `403` for a
member.

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/admin/trackers` | (none) | Every tracker, local boards excepted, with its board mirror and auto-sync settings. |
| `POST` | `/api/admin/trackers` | `{name, provider, site, scope, boardId?, trackerColumns?, stageColumns?, sprints?, issueTypes?, autoSyncEnabled?, autoSyncIntervalMin?}` | Records a tracker (`201`). The identity is derived from the provider, the site and the scope; `400` when a tracker of that identity exists or the payload is invalid. |
| `GET` | `/api/admin/trackers/{id}` | (none) | One tracker, or `404`. |
| `PUT` | `/api/admin/trackers/{id}` | the whole tracker | Rewrites it; the identity is recomputed. |
| `DELETE` | `/api/admin/trackers/{id}` | (none) | Deletes a tracker; `409` while a project selects it or tickets belong to it. |
| `GET` | `/api/admin/trackers/{id}/boards` | (none) | The tracker's boards. |
| `POST` | `/api/admin/trackers/{id}/board-columns` | `{boardId}` | Imports a board's columns onto the tracker and answers the tracker. |
| `GET` | `/api/admin/trackers/{id}/tracker-statuses` | (none) | The tracker's workflow statuses; for a GitHub tracker, the status options of its Projects (v2) board, else open and closed. |
| `GET` | `/api/admin/trackers/{id}/issue-types` | (none) | The tracker's work item types. |
| `GET` | `/api/admin/trackers/{id}/detected-statuses` | (none) | The statuses seen on the tracker's tickets. |

These routes replace the project ones that configured a project's tracker:
`/api/projects/{id}/boards`, `/api/projects/{id}/board-columns`,
`/api/projects/{id}/tracker-statuses`, `/api/projects/{id}/issue-types` and
`/api/projects/detected-statuses` are gone.

### 2.3.1 Personal Tracker Credentials API

A tracker credential may be personal, so a write carries the name of whoever
made it. The routes act on the signed-in caller only: the user comes from the
session, never from the payload, and no answer ever carries a token.

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/me/tracker-credentials` | (none) | What this person stored: tracker, site, e-mail, `account`, sealed, unlocked, `kind`. `account` is who the tracker confirmed the credential belongs to (a GitHub or GitLab login, a Jira display name), absent until it is confirmed. `kind` is `api_token`, or `oauth` for a Jira grant, which also carries `disconnected` and `grantedSites` and is never sealed. The answer adds `jiraOAuth: {configured, sites}`: whether people can connect Jira, and the configured Jira sites (the deployment's and every Jira project's) a grant must cover. |
| `PUT` | `/api/me/tracker-credentials` | `{tracker, siteUrl, email, token, passphrase}` | Stores or replaces one. A passphrase seals it. An empty `token` keeps the stored one, so the site, the e-mail and the sealing can change on their own; a sealed credential must be unlocked for that. Saving forgets the confirmed account, then asks the tracker for it again; a failed answer still saves the credential, with no account. |
| `DELETE` | `/api/me/tracker-credentials?tracker=` | (none) | Forgets one. `404` when there is none to forget. |
| `POST` | `/api/me/tracker-credentials/unlock` | `{tracker, passphrase}` | Supplies the sealing passphrase for this server's lifetime. `409` when the credential is not sealed. |
| `POST` | `/api/me/tracker-credentials/lock` | `{tracker}` | Forgets the derived key. |
| `POST` | `/api/me/tracker-credentials/jira/connect` | (none) | Starts a Jira consent (#654): `{authorizeUrl}`, Atlassian's consent screen, where the web sends the browser. Only a web session may start one, an agent key is refused `403`; `409 {code: "jira_oauth_not_configured"}` without an OAuth app. |
| `GET` | `/auth/jira/callback` | (query `state`, `code` or `error`) | Where Atlassian sends the browser back. Public like the rest of `/auth/`, it reads the web session itself and always redirects to `/?trackerCredentials=jira&jiraOAuth=<outcome>`, the outcome being `connected`, `cancelled`, `invalid` (missing, used, expired, or another session's or person's `state`), `no_site` (the grant covers no configured Jira site) or `unreachable`. Only `connected` stores anything. |
| `GET` | `/api/me/assignee-identities?projectId=\|viewId=` | (none) | Who My Tasks takes the caller to be: `{signedIn, fallback, trackers}`. `fallback` is the account's name and e-mail (the local profile's when signed out); `trackers` lists each non-local tracker of the tickets in scope as `{tracker, identity?, known}`. Same scope rules as `/api/tasks`, `404` on a view that is not the caller's. Answers signed-out callers too, and never reaches a tracker. |

**My Tasks (#468).** A ticket is the caller's when its assignee equals, trimmed
and ignoring the case of A-Z, the confirmed `account` of the caller's personal
credential for the ticket's tracker. On a tracker with no confirmed account,
and on local tickets, it is compared with the account's name and e-mail
instead, ignoring case and accents: a Jira display name "Sebastien FERRY"
matches an account named "Sébastien Ferry". The account is learnt when the credential is saved, or when the stored
one is checked with `POST /api/setup/tracker/check` and no typed token, on its
own site (GitHub and Jira); it is stored in clear in
`user_tracker_credentials.account`, so a sealed and locked credential keeps it.

Stored in `user_tracker_credentials`, encrypted with AES-256-GCM and bound to
`(user_id, tracker)` as additional authenticated data. The key is the server key
held outside the database, or one derived from the owner's passphrase with
Argon2id. A wrong passphrase and a missing record answer the same way.

**Jira grants (#654, ADR 0044).** A row of `kind = 'oauth'` holds, sealed under
the server key with `kind` added to its additional authenticated data, the JSON
`{refreshToken, accessToken, expiresAt, scope, sites: [{cloudId, url, name}]}`.
`site_url` and `email` are empty and `account` is the display name
`/rest/api/3/myself` answered through the grant. A Jira call made for its owner
goes to `https://api.atlassian.com/ex/jira/{cloudId}` with a Bearer token, the
`cloudId` being the one of the project's Jira site; a site the grant lacks fails
with the missing-credential error (`tracker_credential_missing`). An access
token within a minute of its expiry is refreshed first. The refresh is claimed
by a compare-and-set on `version`, which also sets `refresh_claimed_at`, so one
instance spends the rotating refresh token and the others wait for what it
writes. `invalid_grant` sets `disconnected_at`, and every later write fails with
the missing-credential error until the person connects again. The pending
consents live in `jira_oauth_flows`, the state and the web session hashed,
consumed by one statement.

**Jira OAuth app (admin).** `GET`, `PUT {clientId, clientSecret?, redirectUrl}`
and `DELETE` on `/api/admin/jira-oauth` answer
`{configured, clientId, secretSet, redirectUrl, source, unreadable?, updatedAt?}`,
`source` being `database`, `environment` or `none`. The secret is write-only:
an empty one keeps the saved secret, and the first save needs it. `redirectUrl`
must be absolute HTTPS, or HTTP on `localhost`. Stored in `tracker_oauth_apps`,
the secret sealed under the server key with a binding of its own; a saved app
wins over `SECTILE_JIRA_OAUTH_*` as a whole.

### 2.4 Tracker Synchronization API

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/sync/all` | (none) | Queues a sync of every tracker, local boards excepted. |
| `POST` | `/api/sync/github` | `{repo, projectId}` | Queues a GitHub repository sync. |
| `POST` | `/api/sync/jira` | `{projectKey, projectId}` | Queues a Jira project sync. |
| `POST` | `/api/sync/gitlab` | `{projectId}` | Queues a GitLab project sync. |
| `POST` | `/api/trackers/{id}/sync` | (none) | Queues a sync of one tracker (2.3.0.3): `202 {queued, activity}`. |
| `GET` | `/api/sync/auto?projectId=` | (none) | The background loop's state, `{enabled, intervalSec, running, lastRunAt?, lastError?, lastImported, passes, imported, backoffUntil?, trackers}`, where `trackers` is the pacing of each tracker the loop reads, or of the project's trackers with `projectId`: `[{trackerId, name, provider, enabled, intervalMin, lastPassAt?, lastFullSyncAt?}]`. |

The `POST /api/sync/*` routes return `{message, activity}`; the work runs on the
background job queue and its progress is readable through the Activities API.

**Synchronisation is per tracker (#741, ADR 0054).** A tracker is read in full,
whatever projects select it, and each ticket is written once, on its tracker.
A pass asked for a project (`projectId` on `github`, `jira` or `gitlab`) queues
one pass per tracker of the project and answers the first; the activity of each
names its tracker (`trackerId`) and stays attached to the project. The
background loop reads every tracker whose `autoSyncEnabled` is on, at its own
`autoSyncIntervalMin`, and keeps its pacing in `auto_sync_trackers`, so several
server instances still queue one pass per due tracker. The status-to-stage
mapping a sync applies to a ticket is that of the single project the ticket
belongs to, else the tracker's (ADR 0054).

### 2.5 Spec-Driven Design Toolchain API

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/spec-framework/status` | Query params: `projectId` or `repoPath`, optional `framework`. Reports CLI availability and initialization state from the connected local agent. |
| `POST` | `/api/spec-framework/install` | Installs and initializes GitHub Spec Kit or OpenSpec in a working directory. |

`GET /api/spec-framework/status` response:

```json
{
  "frameworks": [
    {
      "framework": "speckit",
      "frameworkLabel": "GitHub Spec Kit",
      "repoPath": "/Users/me/code/my-app",
      "cliAvailable": true,
      "cliCommand": "/Users/me/.local/bin/specify",
      "initialized": false,
      "markerPaths": [],
      "installHint": "uv tool install specify-cli --from git+https://github.com/github/spec-kit.git"
    }
  ]
}
```

`POST /api/spec-framework/install` request:

```json
{
  "framework": "openspec",
  "repoPath": "/Users/me/code/my-app",
  "projectId": "e2c1…",
  "aiAgent": "claude",
  "force": false
}
```

Response: note that `installed: false` still returns HTTP 200, because the
request was valid and `steps[]` carries the diagnosis:

```json
{
  "framework": "openspec",
  "frameworkLabel": "OpenSpec",
  "repoPath": "/Users/me/code/my-app",
  "installed": true,
  "alreadyInit": false,
  "version": "0.9.1",
  "markerPaths": ["/Users/me/code/my-app/openspec", "/Users/me/code/my-app/openspec/project.md"],
  "steps": [
    {
      "label": "Initialisation d'OpenSpec via npx",
      "command": "npx -y @fission-ai/openspec@latest init --yes",
      "success": true,
      "skipped": false,
      "output": "…"
    }
  ],
  "message": "OpenSpec initialisé dans /Users/me/code/my-app"
}
```

An unknown `framework` value returns HTTP 400. `force: true` re-runs the
initializer over an already-initialized directory instead of returning early.

---

## 3. Agent operations and consoles

The server's former `/ws/terminal` and terminal session endpoints return HTTP 410.
They cannot start or access a server shell. Agent-owned PTYs and console history
are available through the desktop companion's authenticated loopback connection.

Git, worktree, editor, CLI status and SDD requests keep their HTTP API surface but
execute through the matching agent. Use `projectId` and optional full `taskId`;
raw server paths are not interpreted as workstation paths. Legacy repository
path parameters only resolve an exact configured project identity. A missing
agent, disconnect or unconfirmed operation returns a structured `error` and never
falls back to execution on the server.

An autonomous run has no terminal for its output to live in, so the agent posts
what the CLI printed as it goes:

| Method | Path | Description |
| :--- | :--- | :--- |
| `POST` | `/api/v1/agent/run-output` | Agent-authenticated. Body `{taskId, runId, output}`. Appends captured output to the run activity, bounded; past the bound the record keeps its head and says it was truncated. |

The launch dispatch carries `mode` (`interactive` or `autonomous`) resolved by
the server. An empty value reads as interactive, so an older server keeps
working. The desktop's own `POST /desktop/tasks` accepts the same optional
`mode` and forwards it without interpreting it.

For transport addresses, authentication, operation envelopes, cancellation and
MCP tool ownership, see [the version 1 contract](contracts/server-agent-v1.md).
