## ADDED Requirements

### Requirement: Explicit target compatibility
Task skill launches SHALL accept optional targetDeviceId and legacy deviceId as equivalent target selectors. Matching nonempty values SHALL be accepted; conflicting nonempty values SHALL be refused without launching. A supplied target SHALL override task affinity and SHALL never be silently replaced.

#### Scenario: Alias matrix
- **GIVEN** an eligible device A
- **WHEN** a request supplies targetDeviceId=A, deviceId=A, or both equal A
- **THEN** only A receives the launch

#### Scenario: Conflicting aliases
- **GIVEN** targetDeviceId=A and deviceId=B, with A different from B
- **WHEN** the request is submitted
- **THEN** it is refused before any launch activity, execution run or dispatch is created

#### Scenario: Explicit invalid target
- **GIVEN** a target belonging to another user, offline, with unknown or empty inventory, or not reporting the resolved project
- **WHEN** it is explicitly requested
- **THEN** launch is refused without selecting an alternative or exposing another user's devices

### Requirement: Project and user eligibility
A task target SHALL be online, belong to the authenticated launch user, and have a reported inventory containing the resolved run project. Registration wildcard scope alone SHALL NOT establish eligibility. Local and cross-instance routing SHALL apply the same eligibility rules.

#### Scenario: Filter before ranking
- **GIVEN** several connected devices but only one reports the resolved project for the launch user
- **WHEN** a launch has no target and no recorded affinity
- **THEN** that device is selected regardless of unrelated connections

#### Scenario: No eligible device
- **GIVEN** no online device of the launch user reports the resolved project
- **WHEN** launch is requested without an explicit target or recorded affinity
- **THEN** it is refused without recording launch side effects

### Requirement: Deterministic automatic routing
Without an explicit target, routing SHALL use the latest successfully launched workstation recorded for the same task, resolved project and launch user. If that recorded workstation is unavailable or no longer serves the project, automatic launch SHALL be refused and require an explicit target, even when a sole alternative exists. Without recorded affinity, routing SHALL choose the eligible device with the latest connection time and use ascending device ID as a stable tie-breaker. Failed and unconfirmed launches and records without a device SHALL NOT create affinity.

#### Scenario: Eligible affinity wins
- **GIVEN** A is the latest successfully launched workstation for this task, project and user and is eligible, and B connected more recently
- **WHEN** launch has no target
- **THEN** A receives it

#### Scenario: Unavailable affinity blocks fallback
- **GIVEN** recorded affinity A is offline or no longer reports the project and B is eligible
- **WHEN** launch has no target
- **THEN** launch is refused and requires an explicit target

#### Scenario: Explicit override of unavailable affinity
- **GIVEN** recorded affinity A is unavailable and B is eligible
- **WHEN** B is explicitly requested
- **THEN** B receives the launch

#### Scenario: Recency and stable tie-break
- **GIVEN** no recorded affinity and eligible devices A and B
- **WHEN** automatic routing is requested
- **THEN** the later connected device is selected, or the lexically smaller device ID if connection times match

#### Scenario: Affinity isolation and failed attempts
- **GIVEN** historical runs for another task, project or user, failed or unconfirmed launches, and legacy records without devices
- **WHEN** routing looks for affinity
- **THEN** those records are excluded, and a newer excluded record does not hide an older matching successful launch

### Requirement: Desktop local execution
Desktop task launches SHALL explicitly target Desktop's own paired device using its authenticated forwarding path, including launches with force. An unavailable or ineligible local device SHALL cause refusal rather than dispatch to another workstation.

#### Scenario: Multiple Desktop devices
- **GIVEN** Desktop runs on eligible A while eligible B has task affinity or a newer connection
- **WHEN** Desktop launches a task skill
- **THEN** only A receives it

#### Scenario: Local device cannot launch
- **GIVEN** Desktop's device is missing its paired identity, offline, or does not report the project
- **WHEN** Desktop launches
- **THEN** it refuses rather than requesting automatic routing

### Requirement: Exact target and launch safety
Routing SHALL validate the target before launch side effects and preserve the chosen device through reconnect grace and cross-instance dispatch. Failure or an unconfirmed launch SHALL NOT trigger dispatch elsewhere. Existing authorization, project resolution, busy/force and device-specific engine behavior SHALL be preserved. A batch SHALL remain on its selected lead workstation.

#### Scenario: Selected device disconnects
- **GIVEN** A was selected and then disconnects while B remains online
- **WHEN** bounded reconnect grace or dispatch runs
- **THEN** only a qualifying reconnection of A may receive the command, and failure does not send it to B

#### Scenario: Remote replica and engine
- **GIVEN** selected A is connected to another server replica with its own engine settings
- **WHEN** launch proceeds
- **THEN** the exact route to A and A's engine resolution are used

#### Scenario: Busy and batch behavior
- **GIVEN** an active task run or a batch request
- **WHEN** launch is attempted
- **THEN** existing busy and force authorization remain effective, and accepted batch members execute together on the lead's chosen device

### Requirement: Durable workstation history
The selected device identity SHALL be recorded on both the execution run and its launch activity before dispatch. Activity history SHALL display that identity after completion, disconnection, server restart and summary updates. Legacy records without a device SHALL remain unknown without guessing from summaries or current connections.

#### Scenario: Completion does not erase the target
- **GIVEN** a run and launch activity targeted A
- **WHEN** launch summaries, status, waiting state, engine or completion are updated and history is reloaded
- **THEN** both records still identify A and the history displays it

#### Scenario: Historical unknown record
- **GIVEN** an activity created before device recording
- **WHEN** migration and history rendering occur
- **THEN** its identity remains empty or visibly unknown and is not inferred

### Requirement: User-facing release documentation
Implementation SHALL add an English Unreleased changelog entry describing deterministic task workstation routing, Desktop local execution and recorded workstation history, including the reported-inventory eligibility change.

#### Scenario: Implementation delivery
- **WHEN** this behavior is implemented
- **THEN** the user-visible changes are documented under Unreleased without creating a release or changing versions
