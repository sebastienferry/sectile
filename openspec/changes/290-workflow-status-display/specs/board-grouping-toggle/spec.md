## ADDED Requirements

### Requirement: Single shared grouping toggle
Every view offering the choice between the status grouping and the agentic workflow grouping SHALL render the same shared toggle component rather than its own copy of the control.

#### Scenario: Board view renders the shared toggle
- **GIVEN** the user opens the Board view
- **WHEN** the board toolbar is rendered
- **THEN** the grouping switch is the shared toggle component
- **AND** no view-local grouping buttons are rendered.

#### Scenario: Backlog view renders the shared toggle
- **GIVEN** the user opens the Backlog view
- **WHEN** the backlog toolbar is rendered
- **THEN** the grouping switch is the same shared toggle component as the Board view
- **AND** no view-local grouping buttons are rendered.

### Requirement: Identical option order, labels and icons
The shared toggle SHALL present the workflow option first and the status option second, with the same localized labels, the same icons and the same tooltips in every view.

#### Scenario: Reading order is stable across views
- **GIVEN** the shared toggle is rendered in the Board view and in the Backlog view
- **WHEN** the user compares both toolbars
- **THEN** the first button is the agentic workflow option in both
- **AND** the second button is the status option in both
- **AND** both buttons carry the same label text and the same icon in both views.

#### Scenario: Only density may differ
- **GIVEN** the Backlog toolbar requests the compact size and the Board toolbar the regular size
- **WHEN** both toggles are rendered
- **THEN** only icon size and button padding differ
- **AND** order, labels, icons, tooltips and active-state styling are identical.

### Requirement: Shared grouping state
The shared toggle SHALL read and write the application-wide `boardGrouping` state, so that switching the grouping in one view is reflected in the other.

#### Scenario: Switching in the backlog is visible on the board
- **GIVEN** the grouping is set to status
- **WHEN** the user selects the workflow option in the Backlog view
- **AND** the user then opens the Board view
- **THEN** the Board view is grouped by workflow stage
- **AND** the workflow option is shown as active in both toolbars.

#### Scenario: The active option is marked
- **GIVEN** the current grouping is the workflow grouping
- **WHEN** the toggle is rendered
- **THEN** the workflow button carries the active styling
- **AND** the status button carries the inactive styling.
