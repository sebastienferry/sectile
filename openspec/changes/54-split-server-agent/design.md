# Design

## Boundaries
`cmd/server` starts only the control plane. `cmd/agent` starts the agent daemon by default and owns `mcp` (stdio bridge) and its internal execution command. Move agent source and its tests out of `cmd/server`. Shared protocol types belong in a dependency-light package, not server handlers, so importing a message envelope does not link the database into the agent.

The server remains the owner of tracker queues and workflow transitions. Replace GitHub/Linear CLI transports with native REST/GraphQL adapters and remove Jira CLI fallbacks, retaining existing Jira HTTP behavior. Use explicit repository identity rather than Git discovery on the server. Authentication failures must remain visible and queued writes must not be falsely completed. Tracker synchronization must work without a connected workstation.

Local workspace and execution capabilities are routed to the authenticated project's agent. A disconnected agent yields an actionable error; never fall back to server filesystem operations. Preserve task/project identity, cancellation, request correlation, timeout and completion semantics. Server-side transitions use returned evidence and tracker information rather than opening an agent checkout. LLM digest generation follows the same agent-only execution rule.

SQLite and server configuration/UI assets are permitted server filesystem operations. Repository contents, worktrees, CLI credential stores and provider subprocesses are agent-owned. Agent configuration generation must point to `taskflow-agent mcp`; Electron must discover and spawn the agent executable without the old `agent` subcommand.

## Build and distribution
`make server` builds the UI and server only. `make agent` builds the agent without frontend dependencies. `make start` runs the agent; `make serve` runs the server. Release builds emit both binaries per supported target. Electron packages `taskflow-agent` as its extra resource. Do not produce unified `taskflow` or `sectile` shims.

## Rejected alternatives
- Two copies of the current executable: retains all server execution paths and couples packaging.
- A server mode flag around the unified entrypoint: fails the explicit independent-binary requirement.
- Delegating tracker CLI calls to a workstation: makes control-plane synchronization dependent on an online agent.
- Silent removal of existing tracker or workspace features: violates the responsibility split by turning migration into feature loss.

## Validation
Build both commands independently; exercise server routes without Git/CLIs or a local checkout. Cover tracker pagination, authentication/errors and mutations with HTTP fixtures; verify agent disconnects do not trigger local fallback. Replay agent lifecycle, console, cancellation and MCP catalog tests after moving them. Verify Electron executable resolution and release names. Run Go tests/vet, web tests/typecheck/lint/build, desktop UI tests/build and OpenSpec strict validation.

## Baseline
Assigned branch: `feat/54`, fast-forwarded to `origin/main` at `ed4052d`. Pre-existing generated skill/configuration changes are preserved and excluded from ticket commits. The full `go test ./...` baseline passed, including terminal tests (22.774s). A bounded isolated terminal run and a Bash comparison also passed; no shell workaround is required.
