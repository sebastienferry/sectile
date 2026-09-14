# desktop-task-header Specification

## Purpose
TBD - created by archiving change 82-task-tty-header-title. Update Purpose after archive.
## Requirements
### Requirement: Selected task identity and title
The desktop TTY header SHALL display the selected task's key, or full task ID when no key exists, followed by its effective title and the selected execution's skill. The effective title SHALL prefer a nonblank local task name over a nonblank tracker title. An unavailable title or a title identical to the displayed identity SHALL NOT introduce an empty or duplicate title segment.

#### Scenario: Tracker title available
- **GIVEN** a selected execution for task `#82` titled `Add task title` with skill `specify` and no local name
- **WHEN** the header is displayed
- **THEN** it reads `#82 · Add task title · specify`.

#### Scenario: Local name overrides tracker title
- **GIVEN** a selected task has a persisted local name and a tracker title
- **WHEN** the header is displayed or tracker metadata refreshes
- **THEN** it uses the local name while retaining task identity and execution skill.

#### Scenario: No usable title or key
- **GIVEN** a selected execution has no usable title
- **WHEN** the header is displayed
- **THEN** it shows the task key and skill without an empty title segment
- **AND** if the key is missing, the full task ID is used instead.

#### Scenario: Name equals identity
- **GIVEN** the effective title is identical to the displayed task key or ID
- **WHEN** the header is displayed
- **THEN** the identity appears only once, followed by the skill.

#### Scenario: No execution selected
- **GIVEN** no execution is selected
- **WHEN** the header is displayed
- **THEN** it prompts the user to select an execution and contains no previous task title.

### Requirement: Title follows current task metadata and selection
The header SHALL update when selected-task metadata arrives or changes and after local rename without requiring reselection. Historical executions SHALL display the current effective task title and the selected historical execution's skill. Title-only updates SHALL preserve console connection and output.

#### Scenario: Delayed metadata
- **GIVEN** an execution is selected before its task title is available
- **WHEN** task metadata arrives
- **THEN** the fallback header gains the effective title automatically
- **AND** the terminal connection and existing output remain unchanged.

#### Scenario: Changed or cleared tracker title
- **GIVEN** the selected task has a previously fetched title and no local name
- **WHEN** a successful metadata refresh supplies a changed title or no usable title
- **THEN** the header shows the changed title or the identity/skill fallback respectively.

#### Scenario: Metadata request fails
- **GIVEN** a metadata request for the selected task fails
- **WHEN** the header renders
- **THEN** a local name or previously known title for that same task remains usable
- **AND** without either, the identity/skill fallback is shown without interrupting console use.

#### Scenario: Switch tasks while metadata is pending
- **GIVEN** metadata for task A is pending and the user selects task B
- **WHEN** task A's response arrives
- **THEN** the header continues to identify task B with task B's effective title or fallback and selected skill.

#### Scenario: Rename locally
- **GIVEN** a task is selected
- **WHEN** the user saves a local name through the existing rename action
- **THEN** the header immediately uses that name and retains identity and selected skill
- **AND** the local name remains effective after reopening the application.

#### Scenario: Select a historical execution
- **GIVEN** a task has several executions with different skills
- **WHEN** the user selects an older execution
- **THEN** the header keeps that task's current effective title and shows the older execution's skill.

### Requirement: Readable and safe header presentation
The header SHALL display task-derived content as literal text. Long titles SHALL remain on one visible line with truncation, expose their full header text on hover and to assistive technology, and preserve toolbar control usability at supported narrow window widths.

#### Scenario: Long title with toolbar controls
- **GIVEN** a long title, a narrow supported desktop window, and visible execution-history, PR, relaunch, export, and stop controls as applicable
- **WHEN** the header renders
- **THEN** the title truncates without overlapping or pushing controls outside the window
- **AND** the full header text is available on hover and to assistive technology
- **AND** the controls remain keyboard-operable.

#### Scenario: Title contains markup-like content
- **GIVEN** a task title contains HTML-like text
- **WHEN** the header renders
- **THEN** that content appears as literal text and does not create markup or execute code.

