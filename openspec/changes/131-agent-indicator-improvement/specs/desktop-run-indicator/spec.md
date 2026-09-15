## ADDED Requirements

### Requirement: Single remote run indicator
A task SHALL display at most one remote run indicator, rendered as an icon without visible text, whose state is running, queued or cancelled.

#### Scenario: A run is executing
- **GIVEN** a task has a remote run with status `running`
- **WHEN** the task is displayed on a card, a list row or the task detail
- **THEN** a single running icon is shown
- **AND** no remote run text label is rendered

#### Scenario: Running takes precedence over queued
- **GIVEN** a task has one `running` remote run and one `queued` remote run
- **WHEN** the indicator is displayed
- **THEN** only the running icon is shown
- **AND** its accessible label reports both runs

#### Scenario: No remote run
- **GIVEN** a task has no remote run that is running, queued or recently cancelled
- **WHEN** the task is displayed
- **THEN** no indicator is rendered

### Requirement: Cancellation from the indicator
The indicator SHALL offer cancellation in place for runs the user owns, without a separate button.

#### Scenario: Hovering a cancellable indicator
- **GIVEN** the indicator shows a running run started by the user's own agent
- **WHEN** the user hovers or focuses the indicator
- **THEN** the indicator presents a stop control labelled with the skill being stopped

#### Scenario: Triggering cancellation
- **GIVEN** the user hovers a cancellable indicator on a task card
- **WHEN** the user activates it
- **THEN** the cancellation request is sent for the displayed runs
- **AND** the surrounding card does not react to the click

#### Scenario: Run that cannot be cancelled
- **GIVEN** the indicator shows a run that was not started by the user's own agent
- **WHEN** the user hovers the indicator
- **THEN** no stop control is offered and the state remains readable

### Requirement: Observable cancellation outcome
A cancelled remote run SHALL remain visible as a cancelled indicator for a short period after it ends.

#### Scenario: A run has just been cancelled
- **GIVEN** a remote run reached status `canceled` less than the visibility window ago
- **WHEN** the task is displayed and no run is running or queued
- **THEN** a cancelled icon is shown

#### Scenario: The window has elapsed
- **GIVEN** a remote run reached status `canceled` longer ago than the visibility window
- **WHEN** the task is displayed
- **THEN** no indicator is rendered
