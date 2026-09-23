# Implementation Tasks — #358 Handle creator of ticket from tracker

## Ordered Checklist

- [x] **Phase 1: Models & Database Migration**
  - [x] Add `Creator` and `CreatorAvatar` fields to `models.Task`, `models.CreateTaskRequest` in `internal/models/models.go`.
  - [x] Add Migration 2 (`tasks.creator`) in `internal/db/migrations.go` with `ALTER TABLE tasks ADD COLUMN creator TEXT NOT NULL DEFAULT ''` and `ALTER TABLE tasks ADD COLUMN creator_avatar TEXT NOT NULL DEFAULT ''`.
  - [x] Update all `SELECT`, `INSERT`, `UPDATE` queries on `tasks` in `internal/db/db.go` and `internal/db/postback.go`.
  - [x] Add tests for Migration 2 and persistence in `internal/db/`.

- [x] **Phase 2: Tracker Ingestion (GitHub & Jira)**
  - [x] Update `GithubIssueItem` and `githubTask` in `internal/trackerapi/mapping.go` to capture `user.login` and `user.avatar_url`.
  - [x] Add tests in `internal/trackerapi/client_test.go` covering GitHub creator mapping.
  - [x] Update `jiraIssue.Fields` and `jiraTask` in `internal/trackerapi/jira_mapping.go` to parse `creator` and `reporter` (name and avatar).
  - [x] Add tests in `internal/trackerapi/jira_test.go` covering Jira creator and reporter fallback mapping.

- [x] **Phase 3: Local Task Attribution**
  - [x] In `internal/handlers/handlers.go`, populate `creator` and `creatorAvatar` from authenticated principal user when creating local tasks.
  - [x] Add/update handler tests for local task creation.

- [x] **Phase 4: Frontend UI (Web)**
  - [x] Add `creator` and `creatorAvatar` to `Task` interface in `web/src/types/index.ts`.
  - [x] Update `TaskDetailModal.tsx` to display Creator (with avatar) in the task details/header area.
  - [x] Update `ListView.tsx` to display Creator in list view rows.
  - [x] Verify UI tests and types: `npx tsc --noEmit` and `npm test` in `web/`.

- [x] **Phase 5: Verification & Integration**
  - [x] Run full test suites: `go test ./...` and `web/` tests.
  - [x] Verify diff against origin/main.
