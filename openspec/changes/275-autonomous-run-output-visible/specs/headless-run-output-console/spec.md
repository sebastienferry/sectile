## ADDED Requirements

### Requirement: The desktop shows a headless run's output
The desktop console pane SHALL display the accumulated output of a selected headless run,
instead of a notice saying the output is recorded elsewhere. The pane SHALL keep saying that a
headless run cannot be typed into.

#### Scenario: Selecting a running headless run
- **GIVEN** a headless run that has already printed output
- **WHEN** the user selects it in the desktop
- **THEN** the pane shows everything the run has printed so far
- **AND** it states that the run is autonomous and takes no input

#### Scenario: Output arriving while the run is watched
- **GIVEN** a selected headless run that is still executing
- **WHEN** the run prints more output
- **THEN** the new output appears in the pane without the user reselecting the run
- **AND** the text already shown is not reprinted

#### Scenario: A finished headless run
- **GIVEN** a headless run that has exited
- **WHEN** the user selects it
- **THEN** the pane shows the run's complete output, including the tail printed before the exit

#### Scenario: A headless run that printed nothing yet
- **GIVEN** a headless run that has produced no output
- **WHEN** the user selects it
- **THEN** the pane says the run is autonomous and has printed nothing yet
- **AND** it does not report a missing console as an error

#### Scenario: The pane refuses input
- **GIVEN** a selected headless run
- **WHEN** the user types into the console pane
- **THEN** nothing is sent to the run

### Requirement: A run with no console for another reason keeps its own notice
Replacing the headless notice SHALL NOT change what the pane says for a queued run, a preparing
run, or a run whose session is genuinely missing.

#### Scenario: A queued run
- **GIVEN** a run whose status is queued or preparing
- **WHEN** the user selects it
- **THEN** the pane shows the waiting-for-a-console notice, as before

#### Scenario: An interactive run whose session is gone
- **GIVEN** a non-headless run with no session
- **WHEN** the user selects it
- **THEN** the pane still directs the user to the task activity and the agent log
