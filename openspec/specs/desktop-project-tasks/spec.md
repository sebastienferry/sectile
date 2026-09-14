# desktop-project-tasks Specification

## Purpose
TBD - created by archiving change 62-project-open-tasks. Update Purpose after archive.
## Requirements
### Requirement: Project open-task entry point
Each desktop sidebar project row SHALL offer an accessible icon action for browsing that project's open tasks, independently of project expansion and existing actions.

#### Scenario: Pointer or keyboard discovery
- **GIVEN** an expanded or collapsed project row
- **WHEN** the user hovers the row or focuses its controls
- **THEN** the open-task icon is visible and keyboard reachable
- **AND** activating it opens the project's task list without changing its collapsed state or launching an execution.

### Requirement: Browse and search open tasks
The task list SHALL load the selected project's unfinished tasks without requiring search text, showing task identity, title and current state.

#### Scenario: Initial list and search
- **GIVEN** a project with open and finished tasks
- **WHEN** the user opens its task list
- **THEN** only that project's open tasks are offered
- **AND** searching narrows results and clearing the search restores the open-task list.

#### Scenario: Loading, empty and failed requests
- **GIVEN** a task-list request
- **WHEN** it is pending, succeeds without matches, or fails
- **THEN** the list shows a loading message, an empty message, or a recoverable error respectively
- **AND** an older response cannot overwrite newer results or another project's dialog.

### Requirement: Explicit task pickup
Users SHALL be able to launch a listed task using a server-provided skill through the existing execution flow.

#### Scenario: Pick a task
- **GIVEN** a listed task and a configured local project
- **WHEN** the user selects a skill and activates Launch
- **THEN** the selected task identity, project and skill are submitted once to the existing launcher
- **AND** opening or searching the list alone submits no execution.

#### Scenario: Unconfigured project or failed launch
- **GIVEN** a missing local repository mapping or a launch failure
- **WHEN** the user views or launches a task
- **THEN** a missing mapping prevents submission and a failed submission remains visible for retry.

