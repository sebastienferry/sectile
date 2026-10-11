## Why

Distinct workstations authenticated as the same user currently replace each other's WebSocket connections. Their reconnect loops interrupt dispatch and execution. Accepted clarification Round 2 for GitHub #813 requires concurrent device connections and explicit dispatch selection.

## What Changes

- Rebound only for the same user and device, regardless of project registration.
- Preserve single-device dispatch; require a device selection when multiple workstations are connected.
- Accept optional `deviceId` on skill, generic dispatch, external terminal and associated operation contracts.
- Cancel on the known launch device or search the user's connected devices for the run owner.
- Keep connection snapshots complete and delayed cleanup safe.

## Capabilities

### New Capabilities
- `workstation-dispatch`: explicit workstation targeting and cancellation across concurrent devices.

### Modified Capabilities
- `remote-agent-decoupling`: registration and rebound become device-aware.

## Impact

Server dispatcher, registration/dispatch handlers, shared dispatch and operation contracts, contract documentation and automated tests. No presence table, schema migration, distributed cluster, tracker synchronization change or agent execution/worktree redesign. A graphical device picker is outside this server task; callers can supply the optional field and receive actionable selection errors.

## Open Questions

None. Round 2 supersedes Round 1's proposed automatic most-recent-device routing.
