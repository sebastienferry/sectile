## ADDED Requirements

### Requirement: The autonomous run always runs headless
An autonomous run SHALL run every step it enqueues in non-interactive mode, whatever the
resolved mode of those skills would be for a single launch.

#### Scenario: An interactive skill inside an autonomous chain
- **GIVEN** a skill whose resolved mode is interactive
- **WHEN** an autonomous run reaches that step
- **THEN** the step runs headless with no terminal window

### Requirement: The autonomous stop stage is a project setting
A project SHALL expose the stage at which its autonomous runs stop, either `implemented` or
`reviewed`, defaulting to `reviewed`. The autonomous run SHALL stop when the task reaches that
stage, and SHALL refuse to start on a task already at or past it.

#### Scenario: Stopping at the default stage
- **GIVEN** a project that has not changed the setting
- **WHEN** an autonomous run reaches the reviewed stage
- **THEN** the chain stops and no further step is enqueued

#### Scenario: Stopping before the pull request
- **GIVEN** a project whose stop stage is implemented
- **WHEN** an autonomous run reaches the implemented stage
- **THEN** the chain stops and no further step is enqueued

#### Scenario: Starting on a task already at the stop stage
- **GIVEN** a project whose stop stage is implemented and a task already at that stage
- **WHEN** the user starts an autonomous run on it
- **THEN** the run is refused, saying the rest needs a human review

#### Scenario: An unrecognised stop stage
- **GIVEN** a project whose stored stop stage is empty or unrecognised
- **WHEN** an autonomous run is started
- **THEN** it stops at the reviewed stage
