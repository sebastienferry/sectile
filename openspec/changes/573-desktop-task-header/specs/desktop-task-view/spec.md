## ADDED Requirements

### Requirement: Two-row task header
The desktop SHALL show the task title on the left of the first row, with execution history and the Next badge aligned right. It SHALL show the worktree path and its affordances on the left of the second row, with the existing action bar aligned right.

#### Scenario: Selected task
- **GIVEN** a selected task with a worktree and available next step
- **WHEN** the task header renders
- **THEN** the title, execution history, and Next badge appear in the first row
- **AND** the path, its affordances, and existing actions appear in the second row
- **AND** their labels, visibility, order, and keyboard behavior remain available

#### Scenario: Narrow desktop window
- **GIVEN** a narrow window and long task title or path
- **WHEN** the header renders
- **THEN** the title and path may truncate while visible actions remain within the window without overlap
- **AND** the status footer remains below the content panel

### Requirement: Independent execution views
The desktop SHALL allow Console and Changes to be toggled independently while keeping at least one visible. Console alone SHALL be the default. A single selected view SHALL fill the panel. When both are selected, Console SHALL be left of Changes with a visible separator adjustable by pointer and keyboard.

#### Scenario: Toggle views
- **GIVEN** Console alone is visible
- **WHEN** the user selects Changes
- **THEN** both panes are visible side by side with both toggles pressed
- **AND** the separator can be moved by pointer or keyboard
- **AND** deselecting either view expands the other to fill the panel
- **AND** deselecting the sole visible view leaves it visible

#### Scenario: Execution and diff updates
- **GIVEN** Changes is visible alongside or instead of Console
- **WHEN** the selected execution changes, a diff refresh completes, or the agent disconnects
- **THEN** the diff presents the selected execution's current data or error state
- **AND** stale responses cannot replace newer data
- **AND** console output and attachment remain available when Console is shown again
