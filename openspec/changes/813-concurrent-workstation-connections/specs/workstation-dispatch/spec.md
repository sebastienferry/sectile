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

### Requirement: Device-aware existing replica routing
Existing persisted presence and cross-instance forwarding SHALL preserve independent user/device connections. Selection SHALL consider all live devices of the user across replicas and SHALL route a command once to the instance holding the selected device. Existing internal authentication, project eligibility and selected-device capability checks SHALL remain enforced.

#### Scenario: Distinct devices on different instances
- **GIVEN** device A is connected to instance 1 and device B to instance 2 for the same user
- **WHEN** either instance receives a dispatch without deviceId
- **THEN** it returns the same selection-required error and both connections stay active.

#### Scenario: Remote explicit dispatch
- **GIVEN** the selected device is held by another live instance
- **WHEN** a command targets its deviceId
- **THEN** the receiving instance forwards once to that device and preserves its confirmation or typed failure.

#### Scenario: Cross-instance same-device rebound
- **GIVEN** an existing connection for a user/device on instance 1
- **WHEN** that same user/device reconnects on instance 2
- **THEN** only the displaced connection receives Session Rebound and distinct devices remain online.

#### Scenario: Delayed cross-instance cleanup
- **GIVEN** a displaced device connection has already been replaced again
- **WHEN** its delayed close or disconnect notification arrives
- **THEN** the newer connection and its live presence remain intact.

#### Scenario: Existing presence upgrade
- **GIVEN** an existing store with project-keyed presence rows
- **WHEN** it is upgraded to the device-aware server
- **THEN** one row per user/device is retained with its latest connection metadata, other devices remain distinct, and disconnected records do not become connected.

#### Scenario: Selected device lacks capability
- **GIVEN** the explicitly selected device cannot execute the requested operation
- **WHEN** the command is dispatched
- **THEN** existing compatibility diagnostics are returned without selecting another workstation.
