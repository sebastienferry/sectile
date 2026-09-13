## ADDED Requirements

### Requirement: Confirmed workstation disconnection
The desktop SHALL offer Remove from desktop in a project's Local settings, require confirmation naming the project and explaining preservation of repository and server data, and disconnect only the selected project after successful confirmation.

#### Scenario: Confirm removal of an idle project
- **GIVEN** a locally configured project with no unfinished executions
- **WHEN** the user confirms Remove from desktop
- **THEN** its workstation repository mapping and per-project execution overrides are removed
- **AND** the project and its consoles disappear from the desktop sidebar.

#### Scenario: Cancel removal
- **GIVEN** the removal confirmation is open
- **WHEN** the user cancels
- **THEN** configuration, visibility, and executions remain unchanged.

#### Scenario: Failed removal
- **GIVEN** a project is connected
- **WHEN** removal cannot be persisted or the local agent rejects the request
- **THEN** the desktop displays an actionable error and does not claim success
- **AND** the project's previous persisted configuration and visibility remain intact.

### Requirement: Active executions prevent removal
Removal SHALL be rejected while any execution of that project is queued, preparing, running, or awaiting confirmed exit. Removal SHALL NOT stop or cancel executions automatically.

#### Scenario: Unfinished execution
- **GIVEN** the project has an execution that has not finished exiting, including one displayed as terminal
- **WHEN** removal is requested
- **THEN** the request is rejected with guidance to stop or wait for executions
- **AND** the project stays connected and the execution is not canceled by removal.

#### Scenario: Other projects are active
- **GIVEN** only other projects have unfinished executions and no configuration operation prevents safe removal
- **WHEN** removal of the idle project is confirmed
- **THEN** that project's removal succeeds without stopping other projects.

#### Scenario: Removal races a new execution
- **GIVEN** a launch and removal target the same idle project concurrently
- **WHEN** the operations are processed
- **THEN** either the launch is admitted and removal is rejected, or removal succeeds and the launch is rejected
- **AND** both operations cannot succeed together.

### Requirement: Disconnection persists until explicit re-add
A disconnected project SHALL remain disconnected across desktop and agent restarts and SHALL reject new local executions until explicitly added again. Automatic repository detection and legacy configuration SHALL NOT reconnect it.

#### Scenario: Restart with a matching repository
- **GIVEN** a disconnected project's repository still exists and matches the agent's repository identity or legacy configuration
- **WHEN** the desktop or agent restarts
- **THEN** the project remains disconnected and hidden
- **AND** a new execution request fails without starting repository preparation or an execution.

#### Scenario: Existing installation without removals
- **GIVEN** an installation has existing project mappings and no recorded disconnections
- **WHEN** the updated agent starts
- **THEN** those projects retain their existing resolution and launch behavior.

#### Scenario: Explicit re-add
- **GIVEN** a disconnected project is available in server project discovery and has retained local console history
- **WHEN** the user chooses it in Add project and successfully saves a valid local repository configuration
- **THEN** it becomes connected and visible again, and eligible for subsequent execution
- **AND** previously removed workstation overrides are not silently restored.

#### Scenario: Failed re-add
- **GIVEN** a project is disconnected
- **WHEN** its new repository configuration is invalid or cannot be saved
- **THEN** it remains disconnected and ineligible for execution.

### Requirement: Preserve data and unrelated configuration
Disconnection SHALL preserve server projects, tracker tasks and their assignments, repository files, worktrees, deployed tooling, other workstation projects, connection settings, and finished console records under existing retention rules.

#### Scenario: Disconnect a project with finished history
- **GIVEN** the project has finished console records and repository tooling
- **WHEN** removal succeeds
- **THEN** those records are hidden rather than deleted by removal
- **AND** repository, server, tracker, and unrelated workstation data are unchanged.

#### Scenario: Restore visibility during the agent lifetime
- **GIVEN** a disconnected project's finished history is still retained
- **WHEN** the project is explicitly added again
- **THEN** its history is available under normal visibility rules
- **AND** separately archived tasks remain archived.

### Requirement: Consistent desktop state and recovery
The desktop SHALL reflect authoritative disconnection state independently of console history changes, prevent stale automatic selection, and allow repeat removal safely. Unsupported agent versions SHALL produce an actionable compatibility error.

#### Scenario: Remove the selected project
- **GIVEN** a console from the project being removed is selected
- **WHEN** removal succeeds
- **THEN** the desktop detaches the console and clears its terminal, selected project, title, directory, history selector, and execution actions
- **AND** polling does not automatically select a hidden console.

#### Scenario: Preserve another project's selection
- **GIVEN** the selected console belongs to another project
- **WHEN** removal succeeds
- **THEN** the selected console remains attached and usable.

#### Scenario: State changes without new runs
- **GIVEN** a project's disconnection state changes while the run list stays unchanged
- **WHEN** the desktop refreshes local-agent state
- **THEN** visibility and Add project availability reflect the new state, including history-only project groups.

#### Scenario: Retry after a lost response
- **GIVEN** an idle project was successfully disconnected but the client did not receive the response
- **WHEN** removal is retried
- **THEN** it succeeds without restoring configuration or deleting retained data.

#### Scenario: Older local agent
- **GIVEN** the local agent does not support project removal
- **WHEN** the user attempts removal
- **THEN** the desktop explains that the agent must be updated or restarted
- **AND** it does not hide the project as though disconnection succeeded.
