# Tasks

## 1. Task read degrades instead of failing
- [x] 1.1 In `internal/taskmcp/server.go`, return the task when `GetTaskByID` succeeds even if `GetTaskComments` fails, carrying the failure as `commentsError`.
- [x] 1.2 Keep the not-found and empty-`taskKey` errors as hard failures.
- [x] 1.3 Test: a tracker-backed task with no tracker credential still returns the task plus a non-empty `commentsError`.
- [x] 1.4 Test: an unknown task key still errors.

## 2. Bounded project context
- [x] 2.1 Project `AgentConfig` down to identity, execution settings, framework, PR stage and skill references in the `get_project_context` tool.
- [x] 2.2 Keep `id`, `directory` and `command` per skill plus the skill-file directories; drop `content` and `commandContent`.
- [x] 2.3 Test: the marshalled payload contains no skill body and stays far below the previous size, while preserving `projectId`, `specFramework` and `prCreationStage`.

## 3. Verification
- [x] 3.1 `openspec validate 50-local-session-task-access --strict`
- [x] 3.2 `go build ./...`, `go vet ./...`, `go test ./internal/... ./cmd/...`
