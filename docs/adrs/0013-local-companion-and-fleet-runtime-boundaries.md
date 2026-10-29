# ADR 0013: Local companion and Fleet runtime boundaries

- Date: 2026-10-29
- Status: Accepted target; phase 0 recorded, implementation pending

## Context

ADR 0003 deliberately reused separate Go server and agent processes to build the
standalone prototype. The user now approved one Local Go companion while keeping
Fleet's agent lightweight and retaining current OAuth behavior.

## Decision

Compose the backend and execution agent as libraries in a separate Local
executable. Keep internal HTTP/WebSocket contracts for the first implementation.
Tauri remains the native UI and narrow command boundary. Fleet continues using
an independently operated server and its existing lightweight agent.

Use explicit mode/connection identity, private paths and capabilities rather
than inferring Local from a loopback URL. Keep authentication separate from mode:
API tokens are the fresh Local starting path, and the existing configured OAuth
installation retains login, reconnect and refresh support during migration.

Tauri is the future primary board for agentic operations. The backend is headless
in the target distribution, retaining API, MCP, auth and the domain logic behind
them. Planning/roadmapping separation is independent and must not remove existing
APIs or macro/workflow data during the runtime extraction.

## Migration and compatibility

The [reviewed plan](../local-fleet-runtime-plan.md) and the Go repository's
`docs/desktop-runtime-contracts.md` define the phased extraction, lifetime store
ownership, conversation identity, shutdown and durable migration journal.
Automatic snapshot rollback ends at commit; later failures recover forward.
Preserve Electron's shared MCP registration and supported Fleet contracts.

The [phase 0 baseline](../phase0-runtime-baseline.md) records source checkpoints,
branches, tests and measurements. The [V1 migration plan](../sectile-v1-migration-plan.md)
sets the product scope. ADR 0003 still describes the currently deployed
implementation until the combined runtime is delivered.

## Consequences

The Local companion includes backend dependencies; the Fleet agent does not.
A third build artifact requires explicit packaging/version compatibility.
One long-lived Local Go process is the target, while provider/guard/MCP bridge
children remain normal. Memory and startup improvements must be measured.
No service is restarted or data migrated by this architecture decision itself.
