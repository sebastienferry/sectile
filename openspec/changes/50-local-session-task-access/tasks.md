# Tasks

## 1. Task read degrades instead of failing
- [ ] 1.1 In `internal/taskmcp/server.go`, return the task when `GetTaskByID` succeeds even if `GetTaskComments` fails, carrying the failure as `commentsError`.
- [ ] 1.2 Keep the not-found and empty-`taskKey` errors as hard failures.
- [ ] 1.3 Test: a tracker-backed task with no tracker credential still returns the task plus a non-empty `commentsError`.
- [ ] 1.4 Test: an unknown task key still errors.

## 2. Bounded project context
- [ ] 2.1 Project `AgentConfig` down to identity, execution settings, framework, PR stage and skill references in the `get_project_context` tool.
- [ ] 2.2 Keep `id`, `directory`, `command` and resolved paths per skill; drop `content` and `commandContent`.
- [ ] 2.3 Test: the marshalled payload contains no skill body and stays far below the previous size, while preserving `projectId`, `specFramework` and `prCreationStage`.

## 3. Verification
- [ ] 3.1 `openspec validate 50-local-session-task-access --strict`
- [ ] 3.2 `go build ./...`, `go vet ./...`, `go test ./internal/... ./cmd/...`
