## Context

Source: `docs/clarifications/813.md`, accepted Round 2, GitHub #813 and the owner's subsequent instruction to start from GitHub main. Baseline: `889e78c9` (main). Assigned branch: `clarify_issue_sectile_task`. Existing registry, persisted `agent_presence` and recent-presence history use `(UserID, ProjectID)`; `AgentConn` already stores DeviceID. Pending operation, launch and task-pull responses already bind to the selected connection. `RunLaunch` currently records user ownership but no device. Preserve those response guards and owner/admin authorization.

## Decisions

### Registry and lifecycle

Change `agentKey` to `(UserID, DeviceID)`. Retain ProjectID on AgentConn as eligibility metadata. Resolve connection device identity in order: nonempty query `deviceId`, authenticated `credential.Device.ID`, then `unknown`. Legacy unknown identities occupy one slot per user. Replace only that slot, close the old socket with 4001, and unregister by device key plus socket pointer. Never hold the registry lock during potentially blocking socket writes; install the replacement atomically and close the displaced connection after unlocking. Update all cleanup and keepalive paths, recent-presence bookkeeping and tests together.

### Selection contract

Introduce one error-returning selection helper taking user, project and optional device. Project compatibility remains exact project or wildcard `all`, `default`, empty. Count distinct live devices across the local registry and existing shared-store directory, deduplicated by user/device, before project eligibility filtering: if more than one exists, omitted device means selection-required. Explicit device must belong to that user and cover the project; never fall back to another device for an invalid target. One device plus omitted selection preserves current eligibility behavior. No device preserves existing disconnected behavior.

Use HTTP 428 for omitted selection with multiple connected devices, HTTP 400 for explicit unknown/ineligible device, and existing 428 disconnected responses. Error text requests `deviceId` and lists the authorized user's eligible device IDs in stable order; do not expose other users' devices. Typed errors distinguish ambiguity from disconnection, preventing local/server fallback or creation of a run before selection succeeds.

Add optional `deviceId` to run-skill, HandleAgentDispatch, LaunchTaskExternalTerminal request structures, `agentconfig.Dispatch` and `agentprotocol.Operation`. Thread it through direct and operation-based entry points, including MCP/local gateway calls where applicable. Select once and send on that exact AgentRoute (local AgentConn or remote AgentLocation) rather than selecting again between lookup and send. Ordinary messages are never broadcast. Reconnection waits retain the requested device and propagate selection errors immediately; they cannot select a different workstation after churn. Audit every Lookup/Dispatch/DispatchAndWait/WaitForAgent consumer, including terminal input/resize and status reporting: informational reads enumerate devices; command routes use explicit selection.

### Run ownership and cancellation

Keep an in-memory `(runID -> userID, deviceID)` launch-ownership map in AgentDispatcher. Populate before sending a run launch, from the resolved AgentRoute; remove on confirmed completion/definite launch failure, preserve on unconfirmed launch, and do not discard merely on socket disconnect. No persistent run-device column is required: the existing presence-key migration is independent of run ownership. Following server restart, historic launches have unknown device ownership and use fallback.

Cancellation targets the recorded device for the run owner first when connected. If ownership is unknown, that device is disconnected, or it returns RunNotOwned, send cancel to a snapshot of that user's project-compatible routes, including devices on other replicas. Use separate message IDs and existing connection-bound response guards. Fan out within the existing shared 15-second context so a silent non-owner cannot prevent an owner from confirming. Avoid sending twice to the initially attempted connection.

Success from any owning agent wins. RunNotOwned from one device is inconclusive while other candidates remain. Close as orphan only when a nonempty complete candidate set all confirms RunNotOwned and no known owner is disconnected. Any timeout, disconnection or failed send without successful cancellation leaves the run open and returns the existing gateway error unless force was explicitly requested. No candidates also requires force. Preserve force notes that the process may still run, owner/admin checks and run/task validation. Clean pending responses and ownership entries on terminal outcomes.

### Existing persisted presence and replica forwarding

Adapt `internal/db/presence.go` and the existing `AgentDirectory` interface to enumerate live user/device locations, resolve an explicit device, and record presence by `(user_id, device_id)` with `project_id` retained as metadata. Keep server-instance liveness filtering, reaping, shared-store enablement and internal bearer authorization unchanged. Selection must consider remote devices even when a local device exists; otherwise different replicas would silently dispatch to different workstations.

Append a numbered migration after the current latest migration (50 at this baseline). Rebuild/rekey the existing `agent_presence` table for SQLite and PostgreSQL through the repository's dialect migration mechanism; never edit already-applied migrations or create a second directory table. Retain one row per user/device, normalizing empty legacy device identities to `unknown`. When several old project rows map to one device, retain the newest connected_at row, with deterministic project_id ordering to break ties. Preserve its instance, timestamps and project metadata; disconnected rows remain disconnected. Update baseline schema parity fixtures as required by the migration tests. Exercise upgrade from existing populated stores and fresh initialization on both engines.

Registration across replicas upserts only the same user/device slot and closes only its previous holder. Carry deviceId and the displaced connection generation (connected_at, serialized consistently) in internal close requests. Compare both before closing so a delayed close cannot kill a newer same-device reconnect. Disconnection updates must match instance and generation as well as user/device; an old socket cannot mark a replacement offline. Make takeover serialization atomic through the repository's transactional database primitives, rather than reading a prior holder then racing an upsert.

Include deviceId on internal dispatch, dispatch-and-wait, operation and pull requests. Internal handlers resolve only that local device, retain project validation and never forward again. Relay typed selection errors without losing their public HTTP meaning. Preserve operation-support/build compatibility checks on the selected route; an outdated selected device must fail rather than switch to another workstation. Cancellation fallback snapshots and deduplicates live routes across replicas and uses the existing one-hop relay, shared deadline and correlation guards. A replica without in-memory launch ownership uses fallback, including after restart.

Mixed old/new server processes cannot safely share the new device-keyed presence contract: deploy updated replicas together, reconnect agents, and document this upgrade constraint rather than promising rolling compatibility with the old project-keyed upsert. Agent clients retain the additive optional v1 targeting contract.

## Target Files

- `internal/db/presence.go`, `migrations.go`, presence/migration/schema parity tests: adapt existing presence keys and upgrade stores.
- `internal/handlers/agent_cluster.go`, cluster tests: explicit one-hop device forwarding, generation-safe takeover and remote enumeration.
- `internal/handlers/agent_dispatcher.go`, `agent_operations.go`: registry, selection, pinned sends, recent presence and ownership/cancel coordination.
- `internal/handlers/handlers.go`, `remote_run.go`, relevant agent API/MCP operation constructors: request decoding, identity fallback, error mapping and launch/cancel plumbing.
- `internal/agentconfig/config.go`, `internal/agentprotocol/operations.go`, `docs/contracts/server-agent-v1.md`: additive optional targeting contract; preserve v1 and legacy single-device callers.
- Existing dispatcher, connection, operation, launch and remote-run handler tests; add focused regression cases alongside them.

## Rejected Alternatives

- LastSeen/map-order selection: violates the owner's explicit-selection decision.
- Project in the uniqueness key: permits duplicate same-device connections and contradicts global user/device rebound.
- New presence database or cluster service: unnecessary; adapt the existing infrastructure and let memory run ownership fall back after restart.
- Treating first RunNotOwned as orphan: can close a run still executing elsewhere.
- Re-resolving a device after selecting it: can send a validated command to a different workstation.

## Validation and Risks

### Main baseline reconciliation

The owner selected GitHub main after the original publication blocker. The three accepted artifact commits were replayed onto main without changing implementation code. The original clarification remains preserved as historical evidence; its assertion that presence/cluster code is absent is superseded by the baseline correction. Preserve current-main infrastructure. The minimal key migration above adapts existing behavior rather than introducing the new persistence/cluster system rejected in Round 2.

Use real test WebSockets for close-code and concurrency assertions, table-driven routing/error tests and handler cancellation tests with controlled responses. Verify no run creation or message on rejected selections. Run `go test -race ./internal/handlers/...`, `make test`, `make build-server build-agent` and desktop unit tests if shared caller plumbing is changed. Strictly validate this OpenSpec change. Preserve the existing client-owned run close path. The implementation adds an English user-facing entry under CHANGELOG.md Unreleased describing simultaneous workstations and required selection when several are connected. Selection is an intentional API behavior change only for multi-device users; clients must pass `deviceId`. Legacy devices lacking both identity sources still share the unknown slot. Memory ownership does not survive restart, so fallback cancellation remains mandatory.
