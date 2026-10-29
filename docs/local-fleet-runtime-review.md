# Independent review: Local/Fleet runtime plan

Date: 2026-10-29
Review status: amendments checked in a second independent review; no remaining
blocking plan defect identified. The user approved phase 0, a separate companion
and temporary internal HTTP/WebSocket transport, while requiring current OAuth
to remain functional.
Runtime extraction has not started.
Scope of the independent review: architecture and source inspection only. No implementation, branch change,
service restart, credential migration or real tracker action was performed.

## Verdict

The target is sound: share backend libraries and compose them with the execution
agent in a dedicated Local companion, preserving a lean Fleet executable.
Retaining HTTP/WebSocket contracts inside the combined process is an appropriate
first delivery. The original plan needed concrete ownership, migration and
identity guarantees before implementation.

The following findings were incorporated into
[the implementation plan](local-fleet-runtime-plan.md). These dispositions are
planning changes, not claims that the existing code already meets them.

## Required amendments

### P1: acquire exclusive store ownership before opening the database

Evidence: `internal/db/db.go:310` invokes interrupted-run recovery from DB open;
`internal/db/instances.go:140` changes SQLite run/presence state. The current
server binds its listener later in `cmd/server/main.go`. Tauri's
`src-tauri/src/local.rs:50` lock covers startup/pairing, not service lifetime.

Disposition: add a companion-owned lifetime lock before DB open, schema/recovery
or journal adoption, distinct from native startup coordination. Prove old
services are quiescent. Test concurrent starters and an old server still alive;
the losing process must not alter stored run state. Older launchers need a
version/discovery barrier because they do not know the new lock.

### P1: make migration transactional and bound the rollback promise

Evidence: `internal/db/db.go:319` starts the queue worker on open. Restoring
SQLite cannot undo a remote tracker write or a rotated OAuth refresh token.

Disposition: durable migration transitions and consistent backup generations
covering SQL/WAL, encryption key, identity/configuration, agent settings and run
snapshots/journals. Disable queue consumption, execution, synchronization,
remote validation and OAuth refresh before commit. Test crashes at each
transition. Snapshot rollback is allowed before commit; after side effects can
resume, stop safely and use explicit forward recovery instead of restoring a
potentially stale credential or execution snapshot.

### P1: persist and verify conversation connection identity

Evidence: `internal/agent/run_store.go:72` stores provider session/mode/folders
without backend/user identity. `src-tauri/src/agent.rs:28` can choose discovery by
manifest presence and fall back to Electron's shared connection file.

Disposition: stable connection identity binds mode, backend/installation and user,
and survives credential rotation. Persist it across manifest, discovery and
conversation history. Explicit Tauri connections never fall back to shared
Electron discovery. Verify provenance before adopting legacy history; ambiguous
records remain readable but cannot execute without explicit reassociation.

### P1: make configuration immutable and restart a composition responsibility

Evidence: `internal/localruntime/config.go` derives mode behavior from environment;
`internal/agentconfig/settings.go` has environment/shared-directory fallback;
`internal/agent/companion_mcp.go` reads the endpoint from environment. The agent's
existing restart path can respawn `os.Executable()` independently of the backend.

Disposition: validated immutable configuration and required Local private paths.
No process-global environment mutation to switch runtime behavior. The composite
owner handles signals, component failure, restart and ownership release. Resolve
auxiliary CLI roles before backend initialization. Test sanitized child
environment and deterministic restart configuration.

### P1: define complete stop/join semantics

Evidence: `DB.Close` drains with a bound but does not stop the queue worker;
`internal/db/db.go:4519` continues consuming queued work. Postback and wait
listeners currently lack unsubscribe; MCP registry stop signals workers without
an exposed join. Auto-sync currently runs until process termination.

Disposition: an explicit component ownership matrix defines admission fence,
cancellation, join and timeout behavior. Keep postbacks available while execution
stops, then flush and stop store users before closing SQL/releasing ownership.
Test full queues, concurrent postback, agent-requested restart and partial
startup. Separate bootstrap reachability from public readiness so an internal
WebSocket connection cannot deadlock startup.

### P2: separate credential policy changes from process embedding

The API-token path for fresh Local installations follows the requested initial
scope. Invalidating the existing working OAuth prototype during process migration
would be a separate behavioral change.

Disposition: preserve credentials and require an explicit compatibility or guided
token-migration decision for that installation. Do not silently change grant kind,
revoke access or make saved credentials unusable. Keep OAuth regression coverage
for Fleet and any supported compatibility profile.

User clarification during review: runtime topology and tracker authentication are
orthogonal. A future OAuth-only remote service could authorize a Local runtime
whose tracker requests, workflows and MCP remain on the workstation. That service
would also participate in Jira refresh, not only initial browser sign-in. Such a
service is outside this implementation plan.

## Sequencing and deferred work

1. Preserve the prototype baseline; define identity, configuration, ownership,
   migration and version contracts before extraction.
2. Extract the backend with the standalone server as its first consumer.
3. Complete lifecycle ownership and restart behavior.
4. Validate the companion with fresh disposable data and existing wire contracts.
5. Implement journaled migration and explicit native connections.
6. Resolve historical OAuth compatibility separately from topology.
7. Validate old/new supported Fleet combinations, Electron, fresh Local and
   migrated Local before removing the old Local launcher.

Single-listener unification, direct dispatch and a generic transport abstraction
remain deferred. The additional companion binary requires explicit packaging
and version compatibility, but protects Fleet's existing dependency boundary.

## Review limits

Source inspection does not prove shutdown, crash recovery, cross-version
compatibility or performance. Those remain implementation acceptance tests.
Phase 0 is now approved and recorded; implementation remains phased and subject
to the acceptance gates in the amended plan.

## Final review outcome

The reviewer confirmed that all five P1 findings are represented in the amended
plan. The user subsequently confirmed continued OAuth compatibility for the
current installation and the separate companion with temporary internal
HTTP/WebSocket transport. Detailed lock, identity-proof and journal specifications
are now recorded in the phase 0 contracts. This is architectural review of a plan,
not runtime verification or evidence that runtime guarantees are implemented.
