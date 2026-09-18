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
    repo_paths TEXT NOT NULL DEFAULT '[]',  -- known working directories, auto-fed when a ticket pins a new CWD
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
    issue_tracker TEXT NOT NULL DEFAULT 'local',  -- 'github' | 'jira' | 'local'
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
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tasks Table
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL DEFAULT 'default',
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
    repo_path TEXT NOT NULL DEFAULT '',  -- per-ticket CWD override; empty means inherit the project, then the global setting
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
```

---

## 2. REST API Endpoints

### 2.1 Tasks API

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/tasks` | Returns array of all tasks. Filters: `projectId`, `q`, `status`, `priority`, `label`, `sprint`, `team`, `assignee` (`__unassigned__` for the work items nobody owns), `pinned=1`. |
| `POST` | `/api/tasks` | Creates a new task bound strictly to `projectId`. |
| `GET` | `/api/tasks/{id}` | Fetches task detail with its activities. |
| `PUT` | `/api/tasks/{id}` | Updates task fields (status, title, description, priority, etc.). |
| `DELETE` | `/api/tasks/{id}` | Deletes task and prunes associated Git worktree. |
| `POST` | `/api/tasks/{id}/skills/{skillId}` | Enqueues or immediately executes an AI skill on the task. |
| `POST` | `/api/tasks/{id}/run-skill` | Runs a skill on the task. Body `{skillId, prompt?, withComments?, mode?}`. `mode` is the one-off execution mode override, `interactive` or `autonomous`; absent means no override, which is not the same as interactive. Any other value is rejected with `400`. |
| `POST` | `/api/tasks/{id}/advance` | Advances one workflow step, or the full chain with `{"auto": true}`. Body also accepts `mode`, the one-off override for the single step; a full chain run ignores it and is always autonomous. |
| `POST` | `/api/tasks/{id}/advance/confirm` | Closes an interactive step. A step the worker already transitioned is accepted as a no-op. |
| `POST` | `/api/tasks/{id}/comment` | Publishes a comment to the GitHub issue tracker. |
| `POST` | `/api/tasks/{id}/epic` | Queues the attachment to an epic (`202`, returns the activity to follow). |
| `GET` | `/api/tasks/{id}/diff` | Computes and returns the Git diff of the task branch vs `main`. |

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
assignment, epic attachment, epic split, roadmap horizon labels. A tracker call
takes seconds and a batch of them far longer, so the HTTP endpoints answer `202`
with the activity to follow, and the activity's steps carry what was attempted
and the tracker's own refusal when it fails.


| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/activities` | Lists recent activities (supports `?taskId=...&status=...`). |
| `GET` | `/api/activities/stats` | Returns aggregate counts (`total`, `queued`, `running`, `completed`, `failed`). |
| `POST` | `/api/activities/{id}/retry` | Re-enqueues a failed activity. |
| `POST` | `/api/activities/{id}/cancel` | Cancels a running or queued activity. |
| `DELETE` | `/api/activities/{id}` | Deletes an activity entry. |
| `DELETE` | `/api/activities` | Clears all completed and canceled activities. |

### 2.3 Projects API

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/projects` | Lists all projects with their task counters. |
| `POST` | `/api/projects` | Creates a new workspace project. |
| `PUT` | `/api/projects/{id}` | Updates project configuration, tracker binding, and paths. |
| `DELETE` | `/api/projects/{id}` | Deletes project and its associated tasks. |
| `POST` | `/api/projects/{id}/skills/install` | Installs default skills into `.gemini/` and `.agents/`. |
| `GET` | `/api/projects/{id}/skills-status` | Reports which workflow skills are scaffolded, per worktree. |
| `POST` | `/api/projects/{id}/install-skills` | Scaffolds the workflow skills into the repo and all its worktrees. |
| `POST` | `/api/projects/{id}/init-git` | Initializes a Git repository in the project working directory. |
| `PUT` | `/api/projects/{id}/skill-editor/{skillId}/mode` | Pins the skill's execution mode for this project. Body `{mode}`: `interactive`, `autonomous`, or empty to clear it and fall back to the project default. |
| `GET` | `/api/projects/{id}/spec-framework-status` | Per-framework SDD status for this project (see 2.5). |
| `POST` | `/api/projects/{id}/install-spec-framework` | Installs a SDD toolchain for this project (see 2.5). |

### 2.3.1 Personal Tracker Credentials API

A tracker credential may be personal, so a write carries the name of whoever
made it. The routes act on the signed-in caller only: the user comes from the
session, never from the payload, and no answer ever carries a token.

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/me/tracker-credentials` | (none) | What this person stored: tracker, site, e-mail, sealed, unlocked. |
| `PUT` | `/api/me/tracker-credentials` | `{tracker, siteUrl, email, token, passphrase}` | Stores or replaces one. A passphrase seals it. |
| `DELETE` | `/api/me/tracker-credentials?tracker=` | (none) | Forgets one. `404` when there is none to forget. |
| `POST` | `/api/me/tracker-credentials/unlock` | `{tracker, passphrase}` | Supplies the sealing passphrase for this server's lifetime. `409` when the credential is not sealed. |
| `POST` | `/api/me/tracker-credentials/lock` | `{tracker}` | Forgets the derived key. |

Stored in `user_tracker_credentials`, encrypted with AES-256-GCM and bound to
`(user_id, tracker)` as additional authenticated data. The key is the server key
held outside the database, or one derived from the owner's passphrase with
Argon2id. A wrong passphrase and a missing record answer the same way.

### 2.4 Tracker Synchronization API

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/sync/all` | (none) | Queues a sync of every configured project across all trackers. |
| `POST` | `/api/sync/github` | `{repo, projectId}` | Queues a GitHub repository sync. |
| `POST` | `/api/sync/jira` | `{projectKey, projectId}` | Queues a Jira project sync. |

All four return `{message, activity}`; the work runs on the background job queue
and its progress is readable through the Activities API.

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
