## ADDED Requirements

### Requirement: Project New task offers both entry paths
The desktop project New task action SHALL offer Run an existing ticket and Quick add task before creating a ticket or launching an execution.

#### Scenario: Open and dismiss the choice
- **GIVEN** a project in the desktop sidebar
- **WHEN** the user opens its New task action and dismisses the dialog
- **THEN** both entry options have been offered and no ticket or execution is created

#### Scenario: Run an existing ticket
- **GIVEN** the user opens New task for a project
- **WHEN** they select Run an existing ticket
- **THEN** they can search that project's tickets and explicitly launch a selected skill using the existing launch flow

### Requirement: Quick add preserves the target project and explicit launch
Quick add entered from a project's New task action SHALL preselect that project, even when a different project's execution is selected. Successful creation SHALL offer the existing separate Launch task action.

#### Scenario: Create then launch
- **GIVEN** a different project's execution is selected
- **WHEN** the user opens New task on the target project and selects Quick add task
- **THEN** that target project is preselected
- **AND** submitting valid task details creates a ticket in that project
- **AND** creation does not implicitly start an execution
- **AND** the user can choose Launch task and explicitly launch a skill for the created ticket

#### Scenario: Command palette compatibility
- **GIVEN** a project is selected
- **WHEN** the user opens Quick add task from the command palette
- **THEN** the current project is preselected as before
