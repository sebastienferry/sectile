## 1. Device connection lifecycle

- [x] 1.1 Change registry and recent-presence keys to user/device; keep project eligibility metadata and avoid blocking writes under registry lock.
- [x] 1.2 Resolve query/credential/unknown identity and pass device plus socket to every unregister path.
- [x] 1.3 Test distinct devices, same device across projects, different users sharing a device ID, identity fallbacks, late unregister, complete snapshots and unaffected keepalive on the other device.

- [x] 1.4 Adapt existing persisted presence to user/device keys with a numbered SQLite/PostgreSQL migration, deterministic legacy-row deduplication, schema parity and upgrade tests.
- [x] 1.5 Update the existing cluster directory and one-hop internal requests with device identity and connection generations; test same-device takeover, delayed close/disconnect, distinct devices on different replicas, liveness filtering and build compatibility.

## 2. Explicit command routing

- [x] 2.1 Add typed selection errors and shared selector; enforce multi-device ambiguity, explicit target validity and existing project compatibility.
- [x] 2.2 Add optional deviceId to request, Dispatch and Operation contracts and relevant API/MCP/gateway constructors; document fields and HTTP errors.
- [x] 2.3 Pin dispatch, confirmed launch, operations and reconnect waits to the selected device/connection; audit all dispatcher consumers and avoid ambiguous server fallback.
- [x] 2.4 Test zero/one/multiple devices across one and several replicas, wildcard and exact projects, unknown and other-user targets, three launch entry points, generic dispatch, no side effects on rejection and response isolation between devices.

## 3. Cancellation and launch ownership

- [x] 3.1 Track launch device in memory by run/user; preserve ownership across disconnect and unconfirmed launch, clean on terminal outcomes.
- [x] 3.2 Try recorded owner then bounded same-user fallback across local and remote routes; use independent connection-bound acknowledgments and shared cancellation deadline.
- [x] 3.3 Preserve authorization, force semantics and orphan diagnostics; do not close on one non-owner or incomplete evidence.
- [x] 3.4 Test known owner, owner disconnected, lost ownership after restart, non-owner replying before owner, all non-owners, silent/failed candidates, no agents, force, cancellation deadline and cross-user acknowledgment rejection.

## 4. Verification and publication

- [x] 4.1 Run focused handlers under race detector, make test and server/agent builds; run desktop unit tests when caller plumbing changes.
- [x] 4.2 Add the English CHANGELOG.md Unreleased entry, document coordinated server upgrade and confirm additive agent v1 contract compatibility and single-workstation regression behavior.
- [x] 4.3 Validate OpenSpec strictly, update the existing draft PR after implementation, and preserve branch and accepted artifacts. Leave review and merge for later workflow stages.

## Implementation notes (2026-10-11)

- Preserved all accepted clarification and specification commits and reused draft PR #817.
- Added the omitted task external-terminal suffix matches so the existing handler's device-aware selection is reachable. The disconnected skill-launch response now uses the specified HTTP 428; recent reconnection diagnostics retain their prior HTTP 409 behavior.
- Queued skill launches validate selection before recording a remote run, carry the selected device and acting user to the operation, and refuse ambiguous selection without dispatching. No tracker synchronization logic was changed.
- Cancellation keeps the recorded owner's acknowledgment alive during bounded concurrent fallback, including a slow owner; an unconfirmed cross-replica launch retains its ownership and open run.
- PostgreSQL timestamp generations are normalized to microseconds. Migration 51 and transactional takeover were exercised against an isolated PostgreSQL 17 database as well as SQLite.
- Validation: `make test` passed (Go suites, formatting, 732 web tests, TypeScript and lint); `go test -race ./internal/handlers/...` passed; `go vet ./...` passed; `make build-server build-agent` passed; desktop `npm test` passed (280 tests); strict OpenSpec validation and `git diff --check` passed. Existing web lint and bundle-size warnings remain; no web source was changed.
- Deployment requires updating server replicas together and reconnecting agents. A graphical device picker, new clustering architecture and persistent run-device ownership remain outside the accepted scope. Agent v1 targeting fields are additive.
