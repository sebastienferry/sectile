## ADDED Requirements

### Requirement: Explicit workstation dispatch
Skill launch, generic agent dispatch, external terminal launch and associated operations SHALL accept optional `deviceId`. Commands SHALL reach only an eligible device of the authorized target user. Eligibility SHALL preserve exact project and `all`, `default`, empty project registrations. Multiple connected devices for that user SHALL require explicit selection; recent activity SHALL NOT choose the device.

#### Scenario: Single-workstation compatibility
- **GIVEN** one connected project-compatible device for the user
- **WHEN** a command omits deviceId
- **THEN** that device alone receives the command with existing successful behavior.

#### Scenario: Selection required
- **GIVEN** multiple devices connected for the user
- **WHEN** a command omits deviceId
- **THEN** the request fails with HTTP 428 and an actionable request to supply deviceId, without launching a run or sending a command.

#### Scenario: Explicit device selected
- **GIVEN** multiple connected devices and a project-compatible device A belonging to the authorized user
- **WHEN** the command specifies A's deviceId
- **THEN** only A receives it and another device's response cannot confirm it.

#### Scenario: Invalid selection
- **GIVEN** an explicit deviceId is unknown, belongs only to another user, or does not cover the project
- **WHEN** a command targets it
- **THEN** the request fails with HTTP 400 and a selection diagnostic without sending to another device or exposing another user's devices.

#### Scenario: No connected device
- **GIVEN** no connected device for the authorized user
- **WHEN** a launch omits deviceId
- **THEN** existing disconnected HTTP 428 behavior prompts the user to start an agent and no server-side execution substitutes for it.

#### Scenario: Device changes during dispatch
- **GIVEN** a command has selected a device
- **WHEN** connections change before sending or during a reconnect wait
- **THEN** the command does not migrate to a different device and an unconfirmed launch remains unconfirmed.

### Requirement: Cancellation finds the run owner
Cancellation SHALL retain existing owner/admin authorization and run/task checks. It SHALL target the recorded launch device when connected, otherwise search the user's connected project-compatible devices. A non-owner reply SHALL NOT establish that another device's run stopped. Fallback SHALL share the existing bounded cancellation deadline.

#### Scenario: Recorded owner connected
- **GIVEN** the launch device is known and connected
- **WHEN** an authorized caller cancels the run and that device confirms
- **THEN** the server closes the run as canceled without requiring workstation selection.

#### Scenario: Fallback owner responds
- **GIVEN** launch device ownership is unknown or disconnected, or the recorded device denies ownership
- **WHEN** fallback cancellation obtains a non-owner reply from A and a cancellation confirmation from B
- **THEN** the server accepts B's confirmation and closes the run only after confirmation, regardless of response order.

#### Scenario: Confirmed orphan
- **GIVEN** a nonempty complete candidate set and no known disconnected owner
- **WHEN** every candidate confirms RunNotOwned
- **THEN** the server may close the run with an orphan explanation rather than claiming an observed process stop.

#### Scenario: Incomplete cancellation evidence
- **GIVEN** no cancellation confirmation
- **WHEN** a candidate times out, disconnects, fails, no device is available, or the known owner is disconnected
- **THEN** the run stays open and cancellation returns an error unless force was explicitly requested.

#### Scenario: Forced close
- **GIVEN** cancellation cannot be confirmed and the authorized caller explicitly requests force
- **WHEN** the server closes the run
- **THEN** its note states that the local process may remain running.

#### Scenario: Slow non-owner
- **GIVEN** fallback includes a silent device and a reachable owner
- **WHEN** the owner confirms within the shared cancellation deadline
- **THEN** the silent device does not prevent successful cancellation.
