## ADDED Requirements

### Requirement: The chained run is named full chain
The action that chains every remaining step of a task SHALL be named **full chain** in the UI,
the API and the settings, so that "autonomous" names only the execution mode of a single run.

#### Scenario: Two distinct choices in the same menu
- **WHEN** the user opens the task card's `...` menu
- **THEN** the single next step and its mode choice are distinct from the full chain action
- **AND** neither is labelled in a way that could be read as the other

### Requirement: A full chain run always runs autonomous
A full chain run SHALL run every step it enqueues in autonomous mode, whatever the resolved
mode of those skills would be for a single launch.

#### Scenario: An interactive skill inside a full chain run
- **GIVEN** a skill whose resolved mode is interactive
- **WHEN** a full chain run reaches that step
- **THEN** the step runs headless with no terminal window

### Requirement: The full chain stop stage is a project setting
A project SHALL expose the stage at which its full chain runs stop, either `implemented` or
`reviewed`, defaulting to `reviewed`. The full chain run SHALL stop when the task reaches that
stage, and SHALL refuse to start on a task already at or past it.

#### Scenario: Stopping at the default stage
- **GIVEN** a project that has not changed the setting
- **WHEN** a full chain run reaches the reviewed stage
- **THEN** the chain stops and no further step is enqueued

#### Scenario: Stopping before the pull request
- **GIVEN** a project whose stop stage is implemented
- **WHEN** a full chain run reaches the implemented stage
- **THEN** the chain stops and no further step is enqueued

#### Scenario: Starting on a task already at the stop stage
- **GIVEN** a project whose stop stage is implemented and a task already at that stage
- **WHEN** the user starts a full chain run on it
- **THEN** the run is refused, saying the rest needs a human review

#### Scenario: An unrecognised stop stage
- **GIVEN** a project whose stored stop stage is empty or unrecognised
- **WHEN** a full chain run is started
- **THEN** it stops at the reviewed stage
