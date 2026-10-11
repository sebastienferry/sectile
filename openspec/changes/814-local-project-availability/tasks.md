# Implementation checklist

## 1. Protocol and collection
- [ ] Add versioned inventory types and parsing bounds; test empty snapshots, duplicates, malformed/null payloads, oversized lists and unsupported versions.
- [ ] Build inventory using authorized discovery and existing local root resolution in all/default/concrete modes. Test mappings, fallback, invalid/deleted Git roots, disconnected projects and global discovery/settings failure. Assert no path/secret enters serialized inventory.
- [ ] Send connection snapshot, configuration-triggered refresh and 30-second revalidation with a bounded cancellable worker. Test empty withdrawal, external edits, recovery, reconnect and no stale writes; preserve heartbeat deadlines.

## 2. Server lifecycle and compatibility
- [ ] Store simultaneous device connections and atomic per-connection authorized snapshots; test A-only/B-only and overlapping availability, same-slot replacement, duplicate scopes, empty withdrawal and unknown legacy state.
- [ ] Implement deduplicated AvailableWorkstations with authenticated project visibility and exact-connection stale update/unregister protection. Test user isolation and teardown/timeout.
- [ ] Preserve latest-connection compatibility routes and wildcard ordering; audit all dispatcher callers, snapshot/pull consumers and cancellation/PTY flows. Bind each launch/operation/result to one exact connection. Test registration during dispatch and late replies from replaced sessions.

- [ ] Add the ephemeral shared-session directory migration and guarded update/remove operations. Preserve compatibility routes, credential presence and instance cleanup; test SQLite migration, PostgreSQL presence and two-server same-device replacement versus distinct-device coexistence.
- [ ] Bind forwarded operations to exact connection IDs; test stale replies/teardown and failed directory writes without switching the run owner.

## 3. Query and documentation
- [ ] Extend authenticated agent status with inventory fields and project-filtered availableWorkstations; test unauthorized/unknown project, foreign-user inventory, empty responses, aggregation, ordering and old field compatibility, including existing build announcements and cross-server results.
- [ ] Add an English user-facing changelog entry under Unreleased during implementation for connected workstation availability; this specification-only PR changes no runtime behavior.
- [ ] Update server-agent contract with wire examples, update/freshness semantics, readiness limits, legacy behavior and routing non-goal.

## 4. Acceptance validation
- [ ] Use two real relay clients for the same user with disjoint then overlapping project inventories; verify both remain connected and the project query returns the expected unique devices.
- [ ] Remove a mapping, delete a checkout and disconnect a device; verify each withdrawal within the specified timing. Reconnect the same device and prove old teardown cannot remove the new snapshot.
- [ ] Exercise a legacy relay alongside a reporting relay and another user; verify existing dispatch ownership and privacy.
- [ ] Run `go test ./internal/agentprotocol ./internal/agent ./internal/handlers` and race-enabled focused lifecycle tests; run repository `make fmt-check`, `make test` and relevant build/lint checks before implementation publication.
- [ ] Run `openspec validate 814-local-project-availability --strict`. Keep the specification PR draft; review and merge remain later workflow steps.
