## ADDED Requirements

### Requirement: Selected execution status belongs in the task footer
The desktop SHALL show the selected execution's run state and skill result in the bottom task status bar, preserving their existing meanings and accessibility.

#### Scenario: Status changes through polling
- **GIVEN** a selected execution
- **WHEN** its run state or server-reported skill result changes
- **THEN** the footer shows the updated indicators and the header contains neither indicator
- **AND** sidebar indicators and console output retain their existing behavior

#### Scenario: Narrow window
- **GIVEN** a selected execution at a narrow desktop window width
- **WHEN** both status indicators and workflow status are displayed
- **THEN** the indicators remain visible within the workspace and the footer stays below the console

### Requirement: Current-step labels do not repeat the execution title
The desktop SHALL hide the decorative current-step badge for active or submitted executions while retaining accessible action names and available next-step labels.

#### Scenario: Execution is active or submitted
- **GIVEN** a task with an active or submitted execution
- **WHEN** its workflow actions are rendered
- **THEN** the Current badge is hidden and the current-step action retains its accessible name

#### Scenario: A next step is available
- **GIVEN** a task with an available next step and no active or submitted execution
- **WHEN** its workflow actions are rendered
- **THEN** the Next badge and launch action remain visible
