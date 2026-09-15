## ADDED Requirements

### Requirement: An unconfirmed launch does not close the run
When a launch confirmation does not arrive before the caller's deadline, the system SHALL leave
the remote run open so the agent can finish it, and SHALL NOT record the run as failed. A launch
the agent reports as failed SHALL still close the run as failed.

#### Scenario: Confirmation does not arrive
- **GIVEN** a skill launched on a connected agent
- **WHEN** the confirmation does not arrive before the deadline
- **THEN** the remote run is still open
- **AND** the caller is told the launch was not confirmed

#### Scenario: The agent finishes a run whose launch was never confirmed
- **GIVEN** a launch that was not confirmed and a skill that then runs to completion
- **WHEN** the agent finishes that run
- **THEN** the run is recorded with the status the agent reports
- **AND** the request is not refused as already finished

#### Scenario: The agent reports the launch failed
- **GIVEN** a skill launched on a connected agent
- **WHEN** the agent reports that the launch failed
- **THEN** the run is closed as failed with the reason the agent gave

#### Scenario: The agent disconnects before confirming
- **GIVEN** a skill launched on a connected agent
- **WHEN** the connection closes before any confirmation
- **THEN** the caller is told the agent disconnected before confirming

### Requirement: A timeout names what it was waiting for
An operation or launch that ends without a confirmation SHALL report the action it was waiting
for, how long it actually waited, and which agent it was waiting on. The same detail SHALL be
recorded server-side.

#### Scenario: A workspace operation is not confirmed
- **GIVEN** a workspace operation sent to a connected agent
- **WHEN** it is not confirmed before the deadline
- **THEN** the error names the action, the elapsed wait and the agent device

#### Scenario: A launch is not confirmed
- **GIVEN** a skill launched on a connected agent
- **WHEN** the confirmation does not arrive before the deadline
- **THEN** the error names the skill, the elapsed wait and the agent device

#### Scenario: The wait is cut short
- **GIVEN** an operation whose caller cancels before the deadline
- **WHEN** the operation ends
- **THEN** the reported elapsed wait is the time actually waited, not the deadline
