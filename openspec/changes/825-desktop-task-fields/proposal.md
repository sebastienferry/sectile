# Desktop task fields (#825)

## Why
The shared Desktop creation/editing page exposes an unnecessary Assignee control, places pull requests before the description, and cannot set issue type or priority. The owner accepted both modes and tracker-discovered types in clarification round 2 (docs/clarifications/825.md).

## What Changes
- Remove Assignee and its outgoing form writes without clearing existing assignments.
- Order fields: Project, Tracker where applicable, Title, Issue type, Priority, Description, Pull requests.
- Discover issue types for the selected tracker, preserving defaults and loaded types with explicit loading, unsupported and retry states.
- Persist type and priority through Desktop, the local agent and existing server contracts.
- Preserve description editing, PR behavior, dirty-state protection and the existing English Desktop surface.

## Capabilities
### New Capabilities
- `desktop-task-fields`: shared task form layout, tracker-specific type selection, priority, persistence and recovery.

### Modified Capabilities
None; this change defines the field behavior in a dedicated capability.

## Impact
Desktop renderer and Electron bridge, local agent creation/edit contracts and discovery bridge, authorized server discovery routing, associated tests, and a user-facing Unreleased changelog entry during implementation. No database migration, web form changes, assignment changes elsewhere, workflow changes, tracker configuration changes or priority mapping changes.
