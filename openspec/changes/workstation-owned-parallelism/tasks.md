# Tasks

## 1. Web
- [x] 1.1 Remove the parallelism picker, its state and its payload field from
      `web/src/components/ProjectModal.tsx`.
- [x] 1.2 Remove `parallelism` from the `Project` type in `web/src/types/index.ts`.

## 2. Server
- [x] 2.1 Remove `Parallelism` from `models.Project`, `CreateProjectRequest` and
      `UpdateProjectRequest`, and delete `models.NormalizeParallelism`.
- [x] 2.2 Remove the `parallelism` column from the projects schema, its migration
      and every read and write in `internal/db`.
- [x] 2.3 Run the server-side skill-job limiter at one worker per project and
      delete `GetProjectParallelism`.
- [x] 2.4 Remove `Parallelism` from `agentconfig.Config`, from the project
      configuration built in `internal/db/agentconfig.go` and from the Sectile MCP
      project payload.

## 3. Agent and desktop
- [x] 3.1 Move the ceiling to `agentconfig.MaxParallelism` and make
      `ExecutionLimit` read the local override alone, defaulting to one.
- [x] 3.2 Drop `inheritParallelism` and `parallelismOverride` from the desktop
      mapping API, and clamp the override against `agentconfig.MaxParallelism`.
- [x] 3.3 Remove the **Inherit from server** control for parallel executions in
      the desktop renderer and rewrite its hint.

## 4. Verification
- [x] 4.1 `go build ./...`, `go vet ./...`, `go test ./...`.
- [x] 4.2 Web: `node --test tests/*.test.mjs`, `tsc --noEmit`, `oxlint src`.
- [x] 4.3 Desktop: `tests/console.ui.cjs` covers the picker without a reset
      control and a workstation value surviving a server refresh.
- [x] 4.4 Update `README.md`, `desktop/README.md`, `docs/ARCHITECTURE.md` and
      `docs/contracts/server-agent-v1.md`.
