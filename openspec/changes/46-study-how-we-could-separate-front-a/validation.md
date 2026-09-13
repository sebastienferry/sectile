# MCP and agent configuration validation

Validated on September 12, 2026, on the assigned issue 46 branch.

- `go test ./cmd/server ./internal/...`: all packages passed.
- `go vet ./...`: passed.
- `go build -o /tmp/taskflow-architecture ./cmd/server`: passed with the built UI embedded.
- `cd web && npm test`: 10 tests passed, 0 failed.
- `npx tsc --noEmit -p tsconfig.app.json`: passed.
- `npx oxlint src`: exited successfully with warnings in unchanged frontend files.
- `npm run build`: passed; Vite reported the existing large-bundle warning.
- `openspec validate 46-study-how-we-could-separate-front-a --strict`: passed.
- `git diff --check`: passed.

Integration coverage includes MCP discovery and all five tools, missing task and
invalid transition inputs, multiline comment content, pinned bearer credentials,
Origin rejection, gateway credential injection, a real stdio subprocess, API-only
configuration preparation, local override precedence, worktree branch guards,
unknown contract versions, path traversal/symlink rejection, local skill edit
preservation, and rollback when a stage report cannot be persisted.

The existing workspace-mutating skill test was removed. Generated skill contracts
are checked in memory, and scaffold behavior is tested in temporary repositories.
Worktree comparisons use filesystem identity to handle macOS `/var` and
`/private/var` aliases correctly.

The local TaskFlow API at port 8090 was unavailable, so the standalone issue stage
transition remains pending. No substitute database was opened to record it.

## External terminal and automatic MCP bootstrap follow-up

- All Go packages pass, including native configuration merge tests for six providers.
- External launch tests cover requests without a skill ID, explicit commands,
  selected skills, agent success/failure confirmation, and immediate launcher errors.
- Script execution tests verify literal shell values, a command executing once,
  and launcher-script cleanup without opening a desktop window.
- Automatic bootstrap tests verify gateway refresh, preservation of unrelated
  settings/servers, and rejection of malformed configuration without overwriting it.
- Desktop windows and authenticated live LLM sessions were not launched by these tests.
  Existing server/agent processes must restart with the rebuilt binary to use the changes.

## Native-client simplification

Removed the experimental Electron client and its companion endpoints. Preserved
its superseded source outside the repository in a temporary archive. No live
user task, process or tracker record was changed during this simplification.

`go test ./...` and `go vet ./...` pass. The native pickup test verifies Codex and
Claude project MCP bootstrap and pickup command construction. The stdio integration
test exercises real task description/comment reads, comment creation and stage
transition through the local agent gateway to an isolated server database.
Native LLM execution still requires the user's authenticated CLI and trust approvals;
no paid LLM request was made. Standalone tracker transition remains pending until
an authenticated TaskFlow MCP connection is available in this development session.

## Native stage ownership regression

Native web dispatch now records `agent_launch` and waits for a local launch
acknowledgement. Historical activities with the exact generated local-agent
launch action no longer trigger the managed-result gate; real managed stages
still block standalone transitions. Tests cover both cases and the HTTP/WS
acknowledgement path. Known legacy curl transition footers are migrated to MCP
without replacing the surrounding skill body. The affected #48 worktree's skill
copies were updated; its task status and tracker were not changed.

## Stabilized v1 contract

The normative contract is `docs/contracts/server-agent-v1.md`. Regression tests
cover configuration validation, fresh downloads, descriptive API errors, terminal
precedence, provider/template override isolation, all-skill prevalidation,
managed-content backup and refresh, safe retirement, and rejection of unsafe
manifest ownership. The current whole Go test suite passes. No live settings,
tracker records or running client sessions were synchronized during validation.
