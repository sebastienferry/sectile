# desktop-task-selection Specification

## Purpose
TBD - created by archiving change 65-selected-task-display. Update Purpose after archive.
## Requirements
### Requirement: Background-only task selection
The desktop sidebar SHALL identify the selected task execution with a highlighted background and no selection-specific visible border, without changing row dimensions.

#### Scenario: Selected task
- **GIVEN** a task execution in the desktop sidebar
- **WHEN** the execution is selected
- **THEN** its selection button has a highlighted background without a selection border
- **AND** its dimensions remain unchanged

#### Scenario: Selection moves
- **GIVEN** a selected execution and another task
- **WHEN** the other task is selected
- **THEN** only the newly selected task retains the selection background

### Requirement: Keyboard focus remains visible
Task controls SHALL retain visible keyboard focus and existing actions.

#### Scenario: Keyboard navigation
- **GIVEN** a task in the sidebar
- **WHEN** the user navigates to its selection button using the keyboard
- **THEN** focus remains visible and activating it selects its execution

