# Tasks

- [x] Extract shared initialization and structured step outcomes.
- [x] Add guarded desktop initialization action and Electron provider argument.
- [x] Add provider selection, status feedback and retry UI.
- [x] Cover success, provider isolation, failures, no-skills providers and retries.
- [x] Update README and changelog.
- [x] Run relevant tests, build and static analysis; review full diff.

## Validation

- `go test ./internal/agent ./internal/agentconfig`: passed (agent 21.121s, agentconfig 0.907s).
- `go vet ./internal/agent ./internal/agentconfig`: passed.
- `go build -o /tmp/sectile-agent-389 ./cmd/agent`: passed.
- `npm run build` in desktop: passed.
- `npm test` in desktop: 95 passed, zero failures.
- `node --test tests/provider-init.ui.cjs tests/mcp-settings.ui.cjs` in desktop: 2 passed, zero failures.
- Reviewed the complete diff, verified no PR feedback was pending and no missing commits from `sectile/main`.

The desktop HTTP tests exercise fresh skills on retry, invalid and missing provider refusal, active execution and unmapped checkout guards, MCP/skills partial failures, and MCP-only providers. Provider-scoped regression tests preserve other installed providers while retiring stale selected-provider skills.
