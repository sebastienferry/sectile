## Why

Current Sectile operates as a single-binary application where the Go backend server, SQLite database, React Web UI, and AI CLI process runner all run locally on the developer's workstation. While ideal for single-developer local usage, this architecture prevents hosting Sectile centrally on a remote website/cloud instance for team collaboration or mobile/web management. 

To enable central web hosting without sacrificing local code privacy, local LLM execution, and local Git worktrees, Sectile needs a decoupled architecture: the Web UI and Database reside on a central remote server, while a lightweight local background agent daemon runs on the developer's workstation and executes LLM-driven workflow steps, local Git operations, and PTY shell commands locally over an outbound WebSocket relay.

## What Changes

- Add a local agent daemon command (`sectile agent --token <KEY>`) to the Sectile binary that establishes a persistent, outbound WebSocket connection (`wss://<remote-server>/ws/agent-connect`) to the remote Sectile hub.
- Create an Agent Dispatcher module in the remote backend (`internal/handlers/`) to route Web UI actions (`clarify`, `specify`, `code`, interactive shell) to the connected local agent.
- Implement user identity and session verification: command payloads are dispatched to a local agent only when `web_session.user_id == agent_session.user_id`.
- Implement a 1:1 active connection limit per user/project mapping (hard cap of 1–2 concurrent agent sessions per user) to guarantee deterministic command routing.
- Relay bi-directional PTY terminal input/output over WebSocket between the remote React Xterm.js interface and the local agent's `creack/pty` manager.
- Retain local-first single-binary mode as an offline fallback.

## Capabilities

### New Capabilities
- `remote-agent-decoupling`: Decoupled architecture supporting a central remote Web UX/Database control plane and a local edge execution worker daemon for LLM tasks, local Git worktrees, and PTY terminal streaming.

### Modified Capabilities
- `terminal-ux`: Extend terminal manager (`internal/terminal/`) to support streaming PTY sessions over remote WebSocket agent connections.

## Impact

- **Backend**: `cmd/server/main.go`, `internal/handlers/handlers.go`, `internal/handlers/agent_dispatcher.go`, `internal/terminal/terminal.go`, `internal/models/models.go`.
- **CLI**: `cmd/sectile/main.go` or `cmd/server/main.go` adding `sectile agent` daemon mode.
- **Frontend**: `web/src/components/` (Agent connection status indicator, remote WebSocket terminal binding).
- **Security & Network**: Authentication middleware for agent tokens and WebSocket connection handshake.
