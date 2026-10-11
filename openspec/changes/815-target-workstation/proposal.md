## Why
Task launches must reach the workstation holding the user's local work. Current routing rejects multiple connections before project inventory filtering, Desktop omits its device, and activity summaries lose the selected workstation. The owner approved deterministic routing and refusal when recorded affinity is unavailable in clarification Round 2 (docs/clarifications/815.md).

## What Changes
- Accept targetDeviceId as an alias of deviceId, rejecting conflicting values.
- Restrict task launch candidates to the authenticated user's online workstations reporting the resolved project in their inventory.
- Prefer the latest successfully launched workstation for this task, project and user; refuse automatic routing if that affinity is unavailable. Without affinity, choose the most recently connected eligible workstation, breaking ties by device ID.
- Pin Desktop task launches to its own paired workstation.
- Persist the target on the execution run and launch activity and display it in activity history.
- Preserve exact-route dispatch, reconnect grace, authorization, busy/force and device-specific engines.

## Capabilities
### New Capabilities
- task-workstation-routing: explicit and automatic task routing, Desktop pinning, and durable target history.

### Modified Capabilities
None.

## Impact
Go request models, handler selection, activity persistence and migrations, Desktop agent forwarding, React activity rendering and regression tests. Older clients with unknown or empty inventories cease to qualify. No new service is required.

## Non-goals
Macro dispatch, worktree transfer, a Web workstation picker, primary-workstation settings, distributing batch members, and implementation during specification.

## Open Questions
None. Round 2 supersedes the ticket's loose fallback wording: connection recency applies only when affinity does not exist.
