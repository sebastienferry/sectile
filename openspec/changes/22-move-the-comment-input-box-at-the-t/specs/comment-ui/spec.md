## Purpose

Functional specification and acceptance criteria for top-positioned comment input box in TaskFlow task modal UI.

## ADDED Requirements

### Requirement: Top-positioned Comment Input Box in Task Modal
The application SHALL render the comment input editor at the top of the comments section, directly above the list of task comments.

#### Scenario: Submitting a new comment from top-placed input box
- **GIVEN** a task modal open with the comments tab/section active
- **WHEN** the user types text in the comment input box at the top and clicks "Publier" (or presses `Cmd/Ctrl + Enter`)
- **THEN** the system SHALL post the new comment to the backend/tracker
- **AND** the comment input box SHALL reset to an empty state
- **AND** the newly published comment SHALL immediately appear at the top of the comments list below the input box.

#### Scenario: Displaying existing comments below top input box
- **GIVEN** a task with one or more existing comments
- **WHEN** the user views the task comments section
- **THEN** the comment input box SHALL be displayed directly beneath the section header
- **AND** the scrollable list of existing comments SHALL be rendered below the comment input box.
