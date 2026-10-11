# Implementation report — #814

## Result

Agents report complete connection-scoped inventories of accessible local Git
projects. Distinct devices remain connected for one user. The authenticated
status API can query unique workstations by project across live server replicas.
Availability remains advisory and does not select execution targets.

The accepted proposal, design, requirements and clarification report were
preserved. The existing `feat/814` branch and draft PR #818 were reused.

## Changed files

| File | Change and purpose |
| --- | --- |
| `internal/agentprotocol/inventory.go` | Versioned inventory payload, canonical IDs and bounded validation. |
| `internal/agentprotocol/inventory_test.go` | Empty, duplicate, malformed, null, oversized and unsupported snapshots. |
| `internal/agentprotocol/message.go` | Document the additive relay message. |
| `internal/agent/agent_inventory.go` | Scoped local collection; coalesced configuration events; cancellable 30-second refresh with a 10-second computation budget. Global failure withdraws availability. |
| `internal/agent/agent_inventory_test.go` | Mapping, fallback, scope, disconnected/deleted checkout, corrupt settings, discovery failure/recovery, periodic refresh and canceled worker coverage. |
| `internal/agent/agent.go` | Send the initial snapshot before readiness; tie the worker to the original relay's lifetime while preserving heartbeat timing. |
| `internal/agent/agent_desktop.go` | Notify after successful mapping/disconnect changes. |
| `internal/handlers/agent_dispatcher.go` | Separate live sessions from compatibility routes; replace only the same device/scope; retain guarded teardown even after replacement; fall back to another live route. |
| `internal/handlers/agent_inventory.go` | Authorize complete snapshots; publish shared state before memory updates; expose unique workstations; maintain batched heartbeat presence and log directory failures. |
| `internal/handlers/agent_cluster.go` | Forward exact session IDs; refuse stale work without retargeting; close only the replaced session; reconnect after failed directory registration. |
| `internal/handlers/handlers.go` | Handle inventory frames; require authentication and user isolation for status; filter authorized project availability. |
| `internal/handlers/agent_inventory_test.go` | Real relay clients with disjoint/overlapping inventories, multiple scopes, empty withdrawals, rejected snapshots, legacy unknown state, reconnect and privacy. |
| `internal/handlers/agent_inventory_internal_test.go` | Two instances on a shared directory; exact forwarding, stale close/reply guards, original operation ownership, timestamp aggregation and failed directory writes/registration. |
| `internal/handlers/agent_dispatcher_test.go` | Update status fixtures to authenticate and replacement fixtures to use the same device. |
| `internal/db/migrations.go` | Migration 51 adds the ephemeral workstation session table and identity/user/instance indexes. |
| `internal/db/workstations.go` | Atomic identity claim; guarded inventory/ping/removal; live-instance queries and inventory-independent route lookup. |
| `internal/db/workstations_test.go` | SQLite and PostgreSQL session coexistence, replacement, empty snapshots, guarded teardown, route fallback and stale-instance exclusion. |
| `internal/db/presence.go` | Carry session IDs and preserve exact/default/all/empty compatibility precedence. |
| `internal/db/instances.go` | Remove workstation rows on single-process restart and dead-instance reclamation. |
| `internal/db/credentialunlocks.go` | Count all live devices when retaining credential unlocks and preserve departure timestamps. |
| `internal/db/migrate.go` | Document exclusion of ephemeral relay sessions from database export. |
| `internal/db/migrations_test.go` | Remove the new table when fixtures rewind the schema for migration replay. |
| `docs/contracts/server-agent-v1.md` | Wire examples, refresh budget, user isolation, shared-server migration, rolling-upgrade limits and routing compatibility. |
| `CHANGELOG.md` | English entry describing connected workstation availability. |
| `openspec/changes/814-local-project-availability/tasks.md` | Record verified checklist completion. |

## Verification

All final commands exited 0. Go checks used `GOCACHE=/tmp/sectile-814-go-cache`;
local test servers required access outside the workspace sandbox.

- `make fmt-check`: no formatting violations.
- `make test`: `go test ./...` passed, including:

  ```text
  ok  tasks/internal/agent          77.506s
  ok  tasks/internal/agentprotocol  (cached)
  ok  tasks/internal/db             (cached)
  ok  tasks/internal/handlers       19.607s
  # tests 732
  # pass 732
  # fail 0
  ```

  TypeScript and oxlint also exited 0. Existing React lint warnings are in
  unchanged frontend files; no frontend source changed in this PR.
- `go test -race ./internal/agent ./internal/handlers ./internal/db -run
  'Test.*Inventory|TestSharedWorkstation|TestDistinctDevice|TestWorkstation'`:
  all three packages passed (`2.430s`, `5.513s`, `3.305s`).
- Final handler race rerun including `TestFailedReplacement` passed (`6.514s`).
- `go vet ./internal/agentprotocol ./internal/agent ./internal/handlers ./internal/db`:
  no diagnostics.
- PostgreSQL 17: `TestPostgresAgentPresence`,
  `TestPostgresSchemaIsCreatedFromEmpty` and `TestPostgresWorkstationSessions`
  all passed (`ok tasks/internal/db 0.764s`) against an isolated temporary
  container. SQLite migration replay and lifecycle tests passed in the full suite.
- `make build-server build-agent`: both binary builds succeeded; Vite built
  successfully with its existing bundle-size warning.
- `openspec validate 814-local-project-availability --strict`:
  `Change '814-local-project-availability' is valid`.
- `git diff --check`: no diagnostics.

Real WebSocket coverage verifies disjoint/overlapping device inventories,
legacy unknown state, rejected snapshots, empty withdrawal, reconnect and
privacy. Refresh-worker tests exercise configuration signals, checkout removal,
periodic recovery and cancellation. Shared-directory tests verify replica-wide
queries, stale forwarded requests, exact rebind closure, original operation
ownership and published-state preservation on failed writes.

Initial test runs exposed sandbox restrictions and migration/status fixture
updates; those were repaired. A new corrupt-settings test initially failed
because its recovery tried to overwrite malformed settings through the guarded
writer; the fixture now removes the corrupt file before recovery. No remaining
failure is suppressed or reported as passing.

## Deliberate exclusions and deployment limit

No workstation picker, automatic target selection, dependency/credential
readiness probe, cloning, hardware inventory or offline history was added.
All server replicas must be upgraded before simultaneous device retention is
guaranteed. Legacy agents keep dispatch compatibility with unknown inventory.
The PR stays draft; implementation does not mark the task reviewed or merge it.
