# Implementation Tasks — #358 Handle creator of ticket from tracker

## Ordered Checklist

- [ ] **Phase 1: Models & Database Migration**
  - [ ] Add `Creator` and `CreatorAvatar` fields to `models.Task`, `models.CreateTaskRequest` in `internal/models/models.go`.
  - [ ] Add Migration 2 (`tasks.creator`) in `internal/db/migrations.go` with `ALTER TABLE tasks ADD COLUMN creator TEXT NOT NULL DEFAULT ''` and `ALTER TABLE tasks ADD COLUMN creator_avatar TEXT NOT NULL DEFAULT ''`.
  - [ ] Update all `SELECT`, `INSERT`, `UPDATE` queries on `tasks` in `internal/db/db.go` and `internal/db/postback.go`.
  - [ ] Add tests for Migration 2 and persistence in `internal/db/`.

- [ ] **Phase 2: Tracker Ingestion (GitHub & Jira)**
  - [ ] Update `GithubIssueItem` and `githubTask` in `internal/trackerapi/mapping.go` to capture `user.login` and `user.avatar_url`.
  - [ ] Add tests in `internal/trackerapi/client_test.go` covering GitHub creator mapping.
  - [ ] Update `jiraIssue.Fields` and `jiraTask` in `internal/trackerapi/jira_mapping.go` to parse `creator` and `reporter` (name and avatar).
  - [ ] Add tests in `internal/trackerapi/jira_test.go` covering Jira creator and reporter fallback mapping.

- [ ] **Phase 3: Local Task Attribution**
  - [ ] In `internal/handlers/handlers.go`, populate `creator` and `creatorAvatar` from authenticated principal user when creating local tasks.
  - [ ] Add/update handler tests for local task creation.

- [ ] **Phase 4: Frontend UI (Web)**
  - [ ] Add `creator` and `creatorAvatar` to `Task` interface in `web/src/types/index.ts`.
  - [ ] Update `TaskDetailModal.tsx` to display Creator (with avatar) in the task details/header area.
  - [ ] Update `ListView.tsx` to display Creator in list view rows.
  - [ ] Verify UI tests and types: `npx tsc --noEmit` and `npm test` in `web/`.

- [ ] **Phase 5: Verification & Integration**
  - [ ] Run full test suites: `go test ./...` and `web/` tests.
  - [ ] Verify diff against origin/main.
