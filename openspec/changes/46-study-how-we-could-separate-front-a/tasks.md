## 1. Backend & Protocol Implementation

- [x] 1.1 Create `AgentDispatcher` in `internal/handlers/agent_dispatcher.go` to manage connected agent WebSockets and dispatch task commands.
- [x] 1.2 Implement `/ws/agent-connect` endpoint in `internal/handlers/handlers.go` with token-based handshake authentication.
- [x] 1.3 Add session guard verifying `web_session.user_id == agent_session.user_id` before dispatching remote UI actions.
- [x] 1.4 Implement 1:1 active connection limit per user/project mapping with session rebound closing logic (`code 4001`).

## 2. Local Agent Daemon Implementation

- [x] 2.1 Add `taskflow agent` sub-command in CLI entrypoint (`cmd/server/main.go` or `cmd/taskflow/main.go`) accepting `--token` and `--url`.
- [x] 2.2 Implement outbound `wss://` client with exponential backoff auto-reconnect.
- [x] 2.3 Connect local daemon to local `terminal.Manager` (`internal/terminal/terminal.go`) and Git worktree isolation engine.
- [x] 2.4 Implement local handler for executing LLM workflow steps (`clarify`, `specify`, `code`) in local worktrees.

## 3. Frontend UI Implementation

- [x] 3.1 Add "Local Agent" status indicator badge (`Connected` / `Disconnected`) in main navigation header.
- [x] 3.2 Update `InteractiveTerminal.tsx` to stream PTY terminal frames through the remote WebSocket relay.
- [x] 3.3 Display disconnected agent warning modal when a user attempts to trigger a workflow step without an active local agent.

## 4. Testing & Validation

- [x] 4.1 Write Go unit tests in `internal/handlers/agent_dispatcher_test.go` for connection registration, auth matching, and dispatching.
- [x] 4.2 Test outbound WebSocket connection, auto-reconnect, and PTY stream relay.
- [x] 4.3 Validate OpenSpec change specification (`openspec validate 46-study-how-we-could-separate-front-a --strict`).
