# Adjustment review — #814

## Outcome and owner decision

The owner instructed adjustment to follow main on 2026-10-11. Integrated
origin/main commit 97e83f1b579824fecae363bf06ee82b5c2a7f5c7 through a merge
on the published feat/814 branch. PR #818 was verified OPEN/draft and matching
sebastienferry/sectile and feat/814 before any changes. No replacement PR was
created. The current specification supersedes the earlier multiple-scope and
latest/local routing requirements.

Main's one connection per authenticated user/device, scope-change replacement,
explicit device selection with multiple connected workstations, run-owner
cancellation and generation guards are preserved. Inventory is advisory and
never selects a target. Registration publishes both inventory and main device
presence atomically; inventory/removal/forwarding retain exact session identity.

Reviewed the complete resulting diff against the revised specification,
including collection cancellation/bounds, settings withdrawal/recovery,
authenticated visibility/user isolation, device replacement, forwarding,
pending ownership, credential presence, instance cleanup and schema replay.

## Findings fixed

- Reconciled device/scope identity and routing with merged #813 following the
  owner's decision; late teardown cannot remove another device or successor.
- Moved inventory migration to 52, after main's device-key migration 51, and
  updated migration rewind fixtures for the combined schema.
- Retained main's device regression tests and restored authentication in three
  status fixtures lost during conflict resolution. The first full suite failed
  on those fixtures; the final complete suite passed after correction.
- Updated clarification, proposal, design, requirements, checklist, protocol
  contract and implementation report to describe the current behavior.

## Feedback dispositions

- PR conversation: retrieved successfully; no comments available.
- PR reviews: retrieved successfully; no reviews available.
- Inline review comments: retrieved successfully; none available.
- Ticket clarification/specification/implementation reports: retained as
  historical evidence; current behavior incorporates the owner's follow-main
  decision instead of the superseded routing design.
- Owner decision: implemented and regression-tested across local and shared
  scope replacement, explicit selection and pending operation ownership.

## Final validation and replay checklist

Go commands used GOCACHE=/tmp/sectile-814-go-cache. Local test servers require
permission outside the workspace sandbox. All final checks below exited 0.

- [x] `make test`: Go suite passed (agent 75.788s, db 22.459s, handlers 21.617s);
  732 web tests passed, zero failed; TypeScript and oxlint passed.
- [x] `make fmt-check`.
- [x] `go test -race ./internal/agent ./internal/handlers ./internal/db -run
  'Test.*Inventory|TestSharedWorkstation|TestDistinctDevice|TestWorkstation|TestFailedReplacement|Test.*Device|TestRelayInventory'`:
  all three packages passed (2.403s, 8.685s, 8.728s).
- [x] `go vet ./internal/agentprotocol ./internal/agent ./internal/handlers ./internal/db`.
- [x] `make build-server build-agent`.
- [x] With SECTILE_TEST_POSTGRES_DSN pointing to an isolated PostgreSQL 17
  database, `go test -count=1 ./internal/db -run
  'TestPostgresAgentPresence|TestPostgresSchemaIsCreatedFromEmpty|TestPostgresWorkstationSessions|TestPostgresPresenceDeviceUpgrade|TestPostgresDevicePresence'`:
  passed (0.899s). Temporary container removed afterwards.
- [x] `openspec validate 814-local-project-availability --strict`.
- [x] `git diff --check`.

Existing frontend lint and bundle-size warnings are in unchanged code.
GitHub reports no CI checks; the evidence above is local validation.

## Reviewer attention

Review atomic user/device replacement across project scopes in
internal/db/workstations.go, exact-session forwarding in agent_cluster.go,
and inventory-independent explicit selection in agent_selection.go.
All server replicas must upgrade before relying on shared inventory.
Dependencies/credentials, provisioning, automatic selection and offline history
remain outside scope. Merge, approval, ticket closure and cleanup belong to
the owner.

## Execution reporting limitation

The requested start_run reuse of ecde593b-0afd-41fc-8afa-b6c78e4cc701 was
refused because the preceding blocked invocation already finished it as failed.
The error explicitly requires a new run ID. No new run ID was substituted,
no database/tracker bypass was used, and the original completed activity was
not reopened. This report preserves the resumed work and validation evidence;
launcher activity reporting must be reconciled separately.
