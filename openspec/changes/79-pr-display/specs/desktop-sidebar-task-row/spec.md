## ADDED Requirements

### Requirement: Inline pull request control
The desktop sidebar SHALL show a single icon-only pull request control in its task row after the title/status area and before archive/menu when the task has a valid linked PR or MR.

#### Scenario: Linked task
- **GIVEN** a task with a valid pull request link
- **WHEN** the sidebar renders
- **THEN** the PR icon appears on the task's existing line without visible PR text or a separate PR line
- **AND** the icon is visible without hovering over the row.

#### Scenario: No linked pull request
- **GIVEN** a task without a valid linked PR, including a link removed on refresh
- **WHEN** the sidebar renders
- **THEN** no PR icon or placeholder appears for that task.

#### Scenario: Long task title at minimum sidebar width
- **GIVEN** a linked task with a long title and the sidebar at its supported minimum width
- **WHEN** the row renders
- **THEN** its title truncates while the PR and other controls remain inside the row
- **AND** the PR does not increase row height or wrap to another line.

### Requirement: Accessible independent navigation
The PR control SHALL retain an accessible name identifying the PR and task, a URL tooltip, visible keyboard focus, and the existing external navigation behavior.

#### Scenario: Activate the PR control
- **GIVEN** a linked task and a different selected execution
- **WHEN** the user clicks its PR icon or focuses it and presses Enter
- **THEN** the linked URL opens externally
- **AND** the selected execution remains unchanged.

#### Scenario: Existing controls
- **GIVEN** a sidebar with linked and unlinked tasks
- **WHEN** the user operates task ID, title/status, archive, or menu controls
- **THEN** each retains its existing action
- **AND** the selected-task toolbar retains its existing PR display.
