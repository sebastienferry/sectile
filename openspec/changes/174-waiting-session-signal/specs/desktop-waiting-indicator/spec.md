## ADDED Requirements

### Requirement: Waiting is a distinct run state in the UI
A run waiting for user input SHALL be presented as waiting, distinctly from running, queued and
finished, wherever a run indicator is shown.

#### Scenario: A run is waiting
- **GIVEN** a task has a run marked as waiting
- **WHEN** the task is displayed on a card, a list row or the task detail
- **THEN** the indicator shows the waiting state
- **AND** its appearance differs from the running state

#### Scenario: Waiting takes precedence
- **GIVEN** a task has one waiting run and one running run
- **WHEN** the indicator is displayed
- **THEN** the waiting state is shown
- **AND** the accessible label reports both runs

#### Scenario: The session resumes
- **GIVEN** a task whose run was displayed as waiting
- **WHEN** the run is no longer marked as waiting and is still executing
- **THEN** the indicator returns to the running state

#### Scenario: No waiting run
- **GIVEN** no run of the task is marked as waiting
- **WHEN** the indicator is displayed
- **THEN** the existing running, queued and cancelled precedence is unchanged

### Requirement: How long the session has been waiting
The waiting presentation SHALL report how long the run has been waiting.

#### Scenario: Reading the wait duration
- **GIVEN** a run has been waiting for several minutes
- **WHEN** the user hovers or focuses the indicator
- **THEN** the elapsed waiting time is reported

#### Scenario: Waiting without a usable timestamp
- **GIVEN** a run is marked as waiting but carries no usable start of wait
- **WHEN** the indicator is displayed
- **THEN** the waiting state is still shown
- **AND** no duration is reported

### Requirement: Finding the waiting sessions
The activities view SHALL let the user reach the runs that are waiting for input.

#### Scenario: Filtering on waiting runs
- **GIVEN** several runs are executing and one is waiting
- **WHEN** the user filters the activities on the waiting state
- **THEN** only the waiting runs are listed
- **AND** each names its task

### Requirement: One vocabulary of run states
The glyph and colour of a run state SHALL be defined once and read by every surface that shows that
state, so no surface can drift from another.

#### Scenario: The badge and the notification agree
- **GIVEN** a run in a given state
- **WHEN** the task list badge and the desktop notification are produced
- **THEN** both take the glyph of that state from the shared definition

#### Scenario: A state changes appearance
- **GIVEN** the shared definition of run states
- **WHEN** a state's glyph or colour is changed in it
- **THEN** every surface showing that state changes with it
- **AND** no surface keeps the former appearance

### Requirement: Nothing platform-specific in the application
The waiting state SHALL be produced and rendered without any dependency on a facility specific to
one operating system, and without any external binary.

#### Scenario: The application runs on another platform
- **GIVEN** Sectile runs on a platform other than macOS
- **WHEN** a run is reported as waiting
- **THEN** the waiting state is stored and rendered normally
- **AND** the notification is raised through that platform's own facility

#### Scenario: No external notifier is required
- **GIVEN** a workstation with no notification binary installed by the user
- **WHEN** a run is reported as waiting
- **THEN** the notification is still raised
