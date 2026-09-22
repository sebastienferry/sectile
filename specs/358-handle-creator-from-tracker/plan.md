# Architecture & Technical Plan — #358 Handle creator of ticket from tracker

## Stack & Target Files

- **Backend (Go)**:
  - `internal/models/models.go`: Add `Creator` and `CreatorAvatar` fields to `Task` and `CreateTaskRequest`.
  - `internal/db/migrations.go`: Add schema Migration 2 (`tasks.creator`) under ADR 0021.
  - `internal/db/db.go`: Update all `tasks` explicit column lists, `INSERT`, `UPDATE`, and `Scan` routines for `creator` and `creator_avatar`.
  - `internal/db/postback.go`: Update `UpdateTask` postback query.
  - `internal/trackerapi/mapping.go`: Add `User` to `GithubIssueItem` and map to `creator` and `creatorAvatar`.
  - `internal/trackerapi/jira_mapping.go`: Add `Creator` and `Reporter` to `jiraIssue.Fields` and map to `creator` and `creatorAvatar`.
  - `internal/handlers/handlers.go`: For local task creation, attribute `creator` and `creatorAvatar` from authenticated principal if available.

- **Frontend (React / TypeScript)**:
  - `web/src/types/index.ts`: Add `creator?: string; creatorAvatar?: string` to `Task`.
  - `web/src/components/TaskDetailModal.tsx`: Render Creator with Avatar in metadata section.
  - `web/src/components/ListView.tsx`: Render Creator in task list row.

- **Tests**:
  - `internal/trackerapi/client_test.go`: Verify GitHub issues map `creator` and `creatorAvatar`.
  - `internal/trackerapi/jira_test.go`: Verify Jira issues map `creator` and `creatorAvatar` (and reporter fallback).
  - `internal/db/migrations_test.go`: Verify Migration 2 applies cleanly to SQLite and PostgreSQL.
  - `internal/db/db_test.go` (or dedicated test): Verify `ImportOrUpdateTasks`, `CreateTask`, `GetTasks`, `GetTaskByID` persist and read `creator` and `creator_avatar`.
  - `web/src/components/__tests__/`: Verify UI rendering of creator and avatar.

---

## Data Contracts & Schemas

### 1. Database Schema Migration 2
```sql
ALTER TABLE tasks ADD COLUMN creator TEXT NOT NULL DEFAULT '';
ALTER TABLE tasks ADD COLUMN creator_avatar TEXT NOT NULL DEFAULT '';
```

### 2. Go Models
```go
type Task struct {
    // ... existing fields ...
    Creator       string     `json:"creator,omitempty"`
    CreatorAvatar string     `json:"creatorAvatar,omitempty"`
    // ...
}
```

### 3. GitHub API Ingestion
```json
{
  "user": {
    "login": "octocat",
    "avatar_url": "https://avatars.githubusercontent.com/u/1"
  }
}
```

### 4. Jira Cloud REST API Ingestion
```json
{
  "fields": {
    "creator": {
      "accountId": "5b10ac8d82e05b22cc7d4ef5",
      "displayName": "Ada Lovelace",
      "avatarUrls": {
        "48x48": "https://avatar-management--avatars.us-west-2.prod.public.atl-paas.net/..."
      }
    },
    "reporter": {
      "accountId": "...",
      "displayName": "...",
      "avatarUrls": { ... }
    }
  }
}
```
Resolution logic:
`displayName` of `creator`, fallback to `accountId` of `creator`, fallback to `displayName` of `reporter`, fallback to `accountId` of `reporter`.
Avatar: `avatarUrls["48x48"]` (or first non-empty url) of `creator`, fallback to `reporter`.

---

## SQL Synchronization Checklist (per MEMORY.md)
All queries selecting or modifying `tasks` table columns must be updated:
1. `initSchema` in `internal/db/db.go`: Leave `tasks` baseline v1 untouched per ADR 0021; migration 2 adds columns.
2. `ImportOrUpdateTasks`:
   - `INSERT INTO tasks (..., creator, creator_avatar)`
   - `UPDATE tasks SET ..., creator = CASE WHEN ? != '' THEN ? ELSE creator END, creator_avatar = CASE WHEN ? != '' THEN ? ELSE creator_avatar END`
3. `CreateTask`:
   - `INSERT INTO tasks (..., creator, creator_avatar)`
4. `UpdateTask`:
   - Preserves `creator` and `creator_avatar`
5. `GetTasks`, `GetTaskByID`, and all select queries:
   - Include `creator, creator_avatar` in column list and `Scan(...)`.
