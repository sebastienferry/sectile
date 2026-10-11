## 1. Device connection lifecycle

- [ ] 1.1 Change registry and recent-presence keys to user/device; keep project eligibility metadata and avoid blocking writes under registry lock.
- [ ] 1.2 Resolve query/credential/unknown identity and pass device plus socket to every unregister path.
- [ ] 1.3 Test distinct devices, same device across projects, different users sharing a device ID, identity fallbacks, late unregister, complete snapshots and unaffected keepalive on the other device.

## 2. Explicit command routing

- [ ] 2.1 Add typed selection errors and shared selector; enforce multi-device ambiguity, explicit target validity and existing project compatibility.
- [ ] 2.2 Add optional deviceId to request, Dispatch and Operation contracts and relevant API/MCP/gateway constructors; document fields and HTTP errors.
- [ ] 2.3 Pin dispatch, confirmed launch, operations and reconnect waits to the selected device/connection; audit all dispatcher consumers and avoid ambiguous server fallback.
- [ ] 2.4 Test zero/one/multiple devices, wildcard and exact projects, unknown and other-user targets, three launch entry points, generic dispatch, no side effects on rejection and response isolation between devices.

## 3. Cancellation and launch ownership

- [ ] 3.1 Track launch device in memory by run/user; preserve ownership across disconnect and unconfirmed launch, clean on terminal outcomes.
- [ ] 3.2 Try recorded owner then bounded same-user fallback; use independent connection-bound acknowledgments and shared cancellation deadline.
- [ ] 3.3 Preserve authorization, force semantics and orphan diagnostics; do not close on one non-owner or incomplete evidence.
- [ ] 3.4 Test known owner, owner disconnected, lost ownership after restart, non-owner replying before owner, all non-owners, silent/failed candidates, no agents, force, cancellation deadline and cross-user acknowledgment rejection.

## 4. Verification and publication

- [ ] 4.1 Run focused handlers under race detector, make test and server/agent builds; run desktop unit tests when caller plumbing changes.
- [ ] 4.2 Confirm additive v1 contract compatibility and single-workstation regression behavior.
- [ ] 4.3 Validate OpenSpec strictly, update the existing draft PR after implementation, and preserve branch and accepted artifacts. Leave review and merge for later workflow stages.
