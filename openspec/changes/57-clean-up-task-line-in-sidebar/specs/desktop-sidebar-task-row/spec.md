## ADDED Requirements

### Requirement: Single-line task row layout in desktop sidebar
The desktop sidebar SHALL render each task execution as a unified inline row within the `.local-task` container containing the task ID, title, status icon, inline PR icon (when linked), and action controls.

#### Scenario: Task with all elements displayed
- **GIVEN** a local task with an associated pull request
- **WHEN** the project group renders the task in the desktop sidebar
- **THEN** the task ID, title, status icon, and PR icon are rendered sequentially within the same `.local-task` row
- **AND** no separate block-level PR indicator is appended outside the row.

#### Scenario: Task without a pull request
- **GIVEN** a local task without an associated pull request
- **WHEN** the task is rendered in the desktop sidebar
- **THEN** no PR icon is rendered
- **AND** the title, status icon, and action controls remain properly aligned.

### Requirement: Interactive controls on the task row
Each interactive control on the task row SHALL maintain its designated action without event interference.

#### Scenario: Clicking task ID
- **GIVEN** a task row in the desktop sidebar
- **WHEN** the user clicks the task ID button
- **THEN** the task is opened in the TaskFlow web interface
- **AND** the console execution is not selected.

#### Scenario: Clicking title and status area
- **GIVEN** a task row in the desktop sidebar
- **WHEN** the user clicks the title or status area
- **THEN** the execution console is selected and focused in the terminal.

#### Scenario: Clicking PR icon
- **GIVEN** a task row with a linked pull request
- **WHEN** the user clicks the PR icon button
- **THEN** the external pull request URL is opened via the system default browser
- **AND** the button provides the accessible name matching `Open PR ... for ...`.
