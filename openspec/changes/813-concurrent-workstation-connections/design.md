## Context

Source: `docs/clarifications/813.md`, accepted Round 2 and GitHub #813. Assigned branch: `clarify_issue_sectile_task`. Existing registry and recent-presence history use `(UserID, ProjectID)`; `AgentConn` already stores DeviceID. Pending operation, launch and task-pull responses already bind to the selected connection. `RunLaunch` currently records user ownership but no device. Preserve those response guards and owner/admin authorization.

## Decisions

### Registry and lifecycle

Change `agentKey` to `(UserID, DeviceID)`. Retain ProjectID on AgentConn as eligibility metadata. Resolve connection device identity in order: nonempty query `deviceId`, authenticated `credential.Device.ID`, then `unknown`. Legacy unknown identities occupy one slot per user. Replace only that slot, close the old socket with 4001, and unregister by device key plus socket pointer. Never hold the registry lock during potentially blocking socket writes; install the replacement atomically and close the displaced connection after unlocking. Update all cleanup and keepalive paths, recent-presence bookkeeping and tests together.

### Selection contract

Introduce one error-returning selection helper taking user, project and optional device. Project compatibility remains exact project or wildcard `all`, `default`, empty. Count the user's active devices before project eligibility filtering: if more than one exists, omitted device means selection-required. Explicit device must belong to that user and cover the project; never fall back to another device for an invalid target. One device plus omitted selection preserves current eligibility behavior. No device preserves existing disconnected behavior.

Use HTTP 428 for omitted selection with multiple connected devices, HTTP 400 for explicit unknown/ineligible device, and existing 428 disconnected responses. Error text requests `deviceId` and lists the authorized user's eligible device IDs in stable order; do not expose other users' devices. Typed errors distinguish ambiguity from disconnection, preventing local/server fallback or creation of a run before selection succeeds.

Add optional `deviceId` to run-skill, HandleAgentDispatch, LaunchTaskExternalTerminal request structures, `agentconfig.Dispatch` and `agentprotocol.Operation`. Thread it through direct and operation-based entry points, including MCP/local gateway calls where applicable. Select once and send on that exact AgentConn rather than selecting again between lookup and send. Ordinary messages are never broadcast. Reconnection waits retain the requested device and propagate selection errors immediately; they cannot select a different workstation after churn. Audit every Lookup/Dispatch/DispatchAndWait/WaitForAgent consumer, including terminal input/resize and status reporting: informational reads enumerate devices; command routes use explicit selection.

### Run ownership and cancellation

Keep an in-memory `(runID -> userID, deviceID)` launch-ownership map in AgentDispatcher. Populate before sending a run launch, from the resolved AgentConn; remove on confirmed completion/definite launch failure, preserve on unconfirmed launch, and do not discard merely on socket disconnect. No database changes. Following server restart, historic launches have unknown device ownership and use fallback.

Cancellation targets the recorded device for the run owner first when connected. If ownership is unknown, that device is disconnected, or it returns RunNotOwned, send cancel to a snapshot of that user's project-compatible connections. Use separate message IDs and existing connection-bound response guards. Fan out within the existing shared 15-second context so a silent non-owner cannot prevent an owner from confirming. Avoid sending twice to the initially attempted connection.

Success from any owning agent wins. RunNotOwned from one device is inconclusive while other candidates remain. Close as orphan only when a nonempty complete candidate set all confirms RunNotOwned and no known owner is disconnected. Any timeout, disconnection or failed send without successful cancellation leaves the run open and returns the existing gateway error unless force was explicitly requested. No candidates also requires force. Preserve force notes that the process may still run, owner/admin checks and run/task validation. Clean pending responses and ownership entries on terminal outcomes.

## Target Files

- `internal/handlers/agent_dispatcher.go`, `agent_operations.go`: registry, selection, pinned sends, recent presence and ownership/cancel coordination.
- `internal/handlers/handlers.go`, `remote_run.go`, relevant agent API/MCP operation constructors: request decoding, identity fallback, error mapping and launch/cancel plumbing.
- `internal/agentconfig/config.go`, `internal/agentprotocol/operations.go`, `docs/contracts/server-agent-v1.md`: additive optional targeting contract; preserve v1 and legacy single-device callers.
- Existing dispatcher, connection, operation, launch and remote-run handler tests; add focused regression cases alongside them.

## Rejected Alternatives

- LastSeen/map-order selection: violates the owner's explicit-selection decision.
- Project in the uniqueness key: permits duplicate same-device connections and contradicts global user/device rebound.
- New presence database or cluster service: outside accepted scope; memory ownership falls back after restart.
- Treating first RunNotOwned as orphan: can close a run still executing elsewhere.
- Re-resolving a device after selecting it: can send a validated command to a different workstation.

## Validation and Risks

### Publication blocker discovered during specification

On 2026-10-11, fetching the verified GitHub repository found `origin/main` at `889e78c9` and the assigned branch at `bd6e76bb`, with 516 commits on main absent from this branch and two accepted clarification commits unique to this branch. Current main includes `agent_cluster.go` and persisted presence, contradicting the accepted clarification's repository findings. This specification targets the assigned worktree and its accepted clarification. Do not publish a PR with the old code tree as deletions against main, silently rebase, remove existing cluster/presence code, or advance the task while its configured specification PR remains blocked. Resolve the intended code baseline first; targeting current main requires renewed clarification of how the existing persistence and cluster behavior fits the accepted scope. Existing artifacts and code must remain available for retry.

Use real test WebSockets for close-code and concurrency assertions, table-driven routing/error tests and handler cancellation tests with controlled responses. Verify no run creation or message on rejected selections. Run `go test -race ./internal/handlers/...`, `make test`, `make build-server build-agent` and desktop unit tests if shared caller plumbing is changed. Strictly validate this OpenSpec change. Selection is an intentional API behavior change only for multi-device users; clients must pass `deviceId`. Legacy devices lacking both identity sources still share the unknown slot. Memory ownership does not survive restart, so fallback cancellation remains mandatory.
