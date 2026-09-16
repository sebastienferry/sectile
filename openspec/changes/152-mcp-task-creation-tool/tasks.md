# Tasks

- [x] Add the `create_task` tool to `internal/taskmcp/server.go` with an explicit input schema requiring `projectId` and `title`, delegating to `db.CreateTask` with `RequireRemoteCreation` forced true and returning `{"task": ...}`.
- [x] Extend the stdio proxy whitelist and catalog-size check in `cmd/agent/mcp.go` from eight tools to nine.
- [x] Extend `internal/mcptest/contract.go` so the shared naming contract covers `create_task` and rejects its legacy-prefixed name.
- [x] Cover the new tool in `internal/taskmcp/server_test.go`: creation on a tracker-backed project, rejection of a missing or blank `projectId`, rejection of a blank title, refusal on a tracker that cannot create remotely, and the enforced `to_clarify` status with the sole `new` label.
- [x] Update `README.md`, `docs/ARCHITECTURE.md` and `docs/contracts/server-agent-v1.md` from eight tools to nine.
- [x] Run `make test` and record the output.
