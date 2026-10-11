## Context
The accepted baseline is docs/clarifications/815.md Round 2. The code already has deviceId, exact AgentRoute dispatch, per-user connection deduplication, reported project inventories, cross-instance presence and device-specific engine resolution. It lacks durable device fields and deterministic task affinity. OpenSpec is initialized in openspec/config.yaml. The assigned specifications and code worktree are the same, branch feat/815.

## Goals / Non-Goals
Implement the behavior specified separately in specs/task-workstation-routing/spec.md. Do not extend macro routing, add settings or a picker, transfer worktrees, change cancellation ownership, or distribute batches.

## Decisions
### Request normalization and Desktop forwarding
Add TargetDeviceID to models.RunSkillRequest with JSON targetDeviceId; retain DeviceID and deviceId. Normalize by trimming strings, treating empty values as absent, accepting equal values, and rejecting unequal nonempty values with HTTP 400. Do this before recording or dispatching anything. In internal/agent/agent_desktop.go, forward d.link.deviceID as targetDeviceId alongside the resolved projectId. Refuse a missing paired identity locally rather than forwarding an empty selector. Do not trust a renderer-provided target to override this pin.

### Task launch selection without changing unrelated dispatch
Keep selection task-aware at the task run-skill boundary in internal/handlers/handlers.go and introduce a task-launch selection helper in internal/handlers/agent_selection.go. Audit callers before changing generic SelectRoute behavior: macro, cancellation and other commands are outside scope. Build candidates from canonical active, deduplicated routes and reported inventory, using internal/handlers/agent_inventory.go and internal/db/workstations.go conventions. Carry connection time, inventory state and project IDs for both local and remote routes; never trust registration scope as an inventory substitute. Rank connection time descending then device ID ascending. Do not rank LastPingAt, which represents heartbeat activity rather than a connection.

Return the selected exact AgentRoute and reuse it for engine resolution and dispatchAndWaitRoute. A replaced session must resolve to the canonical route for the same device, never its displaced connection. Reconnect grace may wait for the same selected/explicit/affinity device only; require a freshly reported qualifying inventory before dispatch. Do not choose a disconnected historical device automatically when no affinity exists. On lookup/storage failure, return HTTP 500 before creating records rather than pretending affinity is absent. Preserve existing project, auth and busy/force validation order where possible.

### Durable successful-launch affinity
Add device_id TEXT NOT NULL DEFAULT '' to task_activities through the existing SQLite/PostgreSQL-compatible migration mechanism. Expose DeviceID as deviceId in models.TaskActivity and web/src/types/index.ts. Add DeviceID to db.RunLaunch so StartAgentRun writes the selected device onto the remote_run row. The agent_launch row also records device ID, launch user and resolved run project (currently omitted on that row).

Use completed agent_launch records as confirmation of successful dispatch, scoped by task_id, run_project_id and user_id, with nonempty device_id. Order by created_at descending then id descending for a stable tie, ignoring remote_run completion status: a successfully launched run that later fails still established local work affinity. Failed and unconfirmed agent_launch rows do not qualify. This avoids a new success flag and avoids associating launches by parsing output. Ensure all launches that qualify have received dispatch acknowledgment; do not treat an unconfirmed launch later reporting completion as a successful launch without explicit confirmation. Legacy rows are left empty rather than backfilled. Consider an index matching task/project/user and successful launches if needed by the query plan.

Write the device before dispatch and propagate it through all activity reads/scanners, inserts, notifications, batch member reports, status synchronization and launch finish paths. A reused run must preserve its original identity; generic start_run or completion calls cannot overwrite it with an empty device. Check internal/db/remoterun.go, internal/db/db.go and activity SQL found via repository search, not just the task history endpoint. Persistence failures prevent sending a command; do not ignore AddTaskActivity errors in this launch path. Retain current handling of failed versus unconfirmed dispatch.

### Errors and display
Conflicting aliases and invalid explicit targets use HTTP 400. Unavailable recorded affinity and no eligible automatic target use HTTP 428 with a French explanation; unavailable affinity asks for targetDeviceId explicitly. Existing busy conflicts remain HTTP 409 and dispatch failures HTTP 502. Do not disclose device IDs owned by other users. Keep existing request validation errors intact unless touched for this feature.

Render the structured device on latest output and past history in web/src/components/TaskDetailModal.tsx; update shared run metadata rendering (including RemoteRunBadge) where appropriate. Show the raw device ID reliably; a friendly label is optional and must not replace the identity or require online lookups. Use existing localization conventions and avoid inventing a device for empty historical values. Add regression coverage using the repository's existing UI test conventions.

## Alternatives Rejected
- Always choose the newest device: loses task-local work when affinity exists.
- Fall back when affinity is unavailable: explicitly rejected by the owner.
- Primary-workstation settings or a Web picker: outside approved scope.
- Wildcard registration or unknown-inventory compatibility: rejected by the owner.
- Replace deviceId instead of aliasing it: breaks existing callers unnecessarily.
- Parse activity summaries or use only in-memory run ownership: summaries change and memory cannot survive restart.
- Use remote_run completed status as dispatch success: execution outcome is distinct from launch confirmation.

## Risks / Rollout
Older clients must announce project inventories to launch. Update existing dispatch tests to announce inventory where their target is intended to qualify, and add negative legacy cases rather than weakening eligibility. Activity schema changes must work on both supported database engines; historical values stay empty. Selection and inventory can change between validation and send, so exact-route validity and no-redispatch behavior need race coverage. No worktree migration occurs when an explicit override changes workstation.

## Validation Strategy
Cover alias combinations, explicit targeting, eligibility exclusions, affinity scope and ranking, successful versus failed/unconfirmed launch records, stable ties, Desktop forwarding, replica routing, reconnects, busy/force and batch behavior. Test migration/read/write/finish preservation and activity rendering after reload and disconnection. Run OpenSpec strict validation now; during implementation run make test, affected database/handler/agent tests (including race coverage), web build and Desktop tests/build when its frontend changes. No production implementation is part of this stage.

## Open Questions
None.
