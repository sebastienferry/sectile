# Technical design

## Existing behavior
The agent resolves local mappings/fallbacks and Desktop persists project settings. Main's merged #813 dispatcher and agent_presence retain one connection per authenticated user/device and require explicit device selection with multiple workstations. Pending operations, launch confirmations and cancellation already retain their owner. On 2026-10-11 the owner instructed adjustment to follow this main behavior.

## Inventory protocol
Add `project_inventory` to `internal/agentprotocol/message.go`. Payload: `{ "schemaVersion": 1, "projectIds": ["project-id"] }`; `projectIds` is required and an empty array is meaningful. No paths, credentials, command templates, OS or provider metadata. Use authenticated connection user and device identity, never payload-supplied identity. Deduplicate IDs and sort agent output. Limit to 4096 IDs and 1 MiB per inventory message; reject malformed, missing/null arrays and unsupported versions without changing the accepted snapshot or disconnecting the healthy relay. Enforce project visibility using the same server authorization/project discovery rules as the authenticated project list. Reject the complete snapshot if any ID is unauthorized or unknown; report an error without echoing sensitive values. Subsequent valid inventories can recover.

The agent computes the inventory in every connection mode from authorized discovered projects, honoring any explicit project connection scope. All/default modes may report all locally available authorized projects; a concrete project connection reports only that project. Reuse `localProjectRoot` without `allowUninitialized`. Omit individually unavailable projects. If discovery/settings reading fails globally, do not send a partial snapshot: withdraw availability with a valid empty inventory and log locally; retry on the next refresh. Availability is advisory, not an execution guarantee.

Send once after WebSocket registration and before advertising readiness; serialize through the existing link writer. Recompute after successful Desktop mapping/disconnect writes and every 30 seconds to catch external settings edits, folder disappearance and project visibility changes. Coalesce configuration events; do not run Git checks inside the heartbeat writer. Resend the complete inventory each refresh, even unchanged, so it can recover from server rejection. Bound one computation to 10 seconds with context cancellation; abandon work for a closed connection and never write a result through a replacement connection. Reconnection starts with unknown inventory until its first accepted snapshot. Existing heartbeat/keepalive timing stays intact.

## Live registry and compatibility
Store one live connection per authenticated user/device, replacing that device even when its project scope changes. Distinct devices coexist; missing device IDs use the legacy unknown sentinel. Inventories belong to exact server-generated connection IDs. Replaced connections cannot publish snapshots or unregister successors. AvailableWorkstations returns only live devices whose accepted snapshot advertises the project. Single-instance inventory stays in memory; shared deployments publish it in the ephemeral directory.

Preserve main’s #813 execution selection: require an explicit deviceId when multiple devices are connected, validate the selected project scope before side effects, and never choose by inventory or recency. A single eligible device remains compatible with legacy dispatch. Resolve each operation once; pending results, cancellation, PTY and pulls remain bound to the owning session.

Legacy agents without `project_inventory` remain connected and dispatch-compatible, with inventory unknown. Neither concrete nor wildcard registration is evidence of verified local availability. Old servers ignore the additive message using their existing unknown-message handling; ordinary heartbeat/launch behavior continues.

## Query contract
Extend existing authenticated `GET /api/agent/status` with optional `projectId`. Unfiltered responses retain the existing `agents` shape/fields, restricted to the requesting user, and add per-connection `inventoryState` (`unknown` or `reported`), `availableProjectIds` (empty for unknown) and `inventoryUpdatedAt` (omitted for unknown). With `projectId`, validate project visibility (403 for inaccessible/unknown), return the user's connected agents advertising that project, and add `availableWorkstations`: deduplicated `{deviceId, connectedAt, lastPingAt}` entries sorted by deviceId. Return the device’s current connection and last-ping timestamps. No cross-user query parameter; derive identity from the existing request auth context, respecting the repository's implicit-user deployment mode. Authorized projects with no availability return empty arrays. Keep consumers tolerant of added fields and never return local paths. Preserve existing build/version/operation announcement fields. Use `webPrincipal(r)` for caller identity and existing project-access helpers; anonymous requests return 401 even if a surrounding middleware is relaxed.

## Shared-server deployments
Main migration 51 keys agent_presence by user/device. Preserve it for routing and credential presence. Migration 52 adds agent_workstation_presence with a connection_id primary key and unique user/device identity, including project scope, instance, timestamps and inventory. Atomically update both directories when a device connects, returning the displaced session. Updates and teardown compare exact connection and instance identity.

Write an accepted authorized inventory atomically to this row before exposing it from shared queries; on directory write failure log and keep the prior published state, retrying on refresh. Queries join the existing live `server_instances` cutoff and exclude unregistered or stale sessions; crashed instances are hidden at the existing instance-dead threshold and removed by the existing reaper. Extend single-process/instance cleanup, and credential unlock/presence consumers, so other live devices are not forgotten when one compatibility route ends. No offline availability result or permanent inventory history is introduced. Shared status queries aggregate these rows across instances without forwarding queries through arbitrary project routes. Heartbeats update last_ping_at using existing batching/cadence patterns; do not add a DB write for every relay frame.

Keep agent_presence synchronized with each device session in the same registration transaction; teardown marks only the matching device generation disconnected. Forwarded requests carry device identity, generation and connection_id, refusing replaced sessions. Main’s explicit selection and run-owner cancellation remain intact. Upgrade all server replicas before relying on shared inventory; agents can upgrade independently.

Use the project's next migration version, not a hardcoded number. Cover SQLite migrations and PostgreSQL integration, fake directory interfaces, two-instance rebind/query tests and stale-instance expiry. This technical migration is necessary for existing replicated deployment support and does not expand product scope.

## Target files
- `internal/agentprotocol/message.go`: payload and version definitions.
- `internal/agent/agent.go`, `agent_config.go`, `agent_desktop.go`: collection, per-connection refresh lifecycle and settings events.
- `internal/handlers/agent_dispatcher.go`, `agent_operations.go`, `handlers.go`: inventory session lifecycle and main device routing, exact-connection dispatch and status filtering.
- `internal/handlers/agent_cluster.go`, `internal/db/presence.go`, `migrations.go`, `instances.go`, `credentialunlocks.go`: shared live inventory, route preservation, guarded cleanup and migration.
- Existing agent configuration, disconnect, keepalive, dispatcher, operation, launch, auth and reconnect tests, plus focused inventory tests.
- `docs/contracts/server-agent-v1.md`: protocol, freshness, compatibility and status contract.

## Rejected alternatives
Paths in query strings contradict the local-path contract. Full readiness probing widens the owner's agreed scope. Inventory-based launch selection belongs to another ticket. A database registry unnecessarily persists offline availability. Keeping a single user/project connection cannot represent multiple devices. Reusing wildcard registration as availability produces false positives.

## Risks and validation
Audit every dispatcher consumer because routing and presence now differ. Race-test stale unregister/update and concurrent snapshots. Test real WebSocket interleavings for launch ownership, status isolation and reconnect. Do not infer task readiness from this inventory. No unresolved design or product decisions.
