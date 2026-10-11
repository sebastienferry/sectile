## ADDED Requirements

### Requirement: Announce local project availability
Each connected workstation SHALL announce its complete set of authorized locally available projects. Available SHALL mean an accessible Git checkout resolved using existing local mappings or current-repository fallback, within the connection's project scope, and not locally disconnected. Availability SHALL NOT promise that CLI, credentials or build dependencies are ready.

#### Scenario: Different local checkouts
- **GIVEN** one user has device A with project P available and device B with project Q available
- **WHEN** both agents connect and announce their inventories
- **THEN** both remain connected and the server records P for A and Q for B

#### Scenario: Unavailable project
- **GIVEN** a project is disconnected, its Git checkout is inaccessible or it has no valid local mapping/fallback
- **WHEN** the agent evaluates its inventory
- **THEN** that project is absent without cloning or provisioning it

### Requirement: Refresh complete snapshots
The agent SHALL announce on each connection/reconnection, refresh following successful local project configuration changes, and revalidate external changes at least every 30 seconds. Each refresh SHALL replace the connection's entire inventory atomically, including an empty inventory. With a healthy relay, the server SHALL reflect changes within 40 seconds (30-second refresh plus a computation budget of 10 seconds).

#### Scenario: Mapping removed or checkout disappears
- **GIVEN** the workstation advertises P
- **WHEN** P is disconnected, its mapping is removed without a valid fallback, or its checkout disappears
- **THEN** the next refresh withdraws P within the healthy-relay update budget

#### Scenario: Nothing available
- **GIVEN** a workstation has no available projects or cannot reliably read its project configuration
- **WHEN** it refreshes its inventory
- **THEN** the server receives an explicit empty inventory and records no available projects for that connection

### Requirement: Track connected devices safely
The server SHALL associate availability with the authenticated user, device and live connection. Distinct devices SHALL coexist. Availability SHALL disappear when its owning connection unregisters, including keepalive teardown, and SHALL NOT persist as offline availability. Multiple live scopes for one device SHALL contribute a union without duplicate workstation entries.

#### Scenario: Replaced connection teardown
- **GIVEN** a new connection replaces the same user/device/scope session
- **WHEN** the old connection sends an inventory or unregisters late
- **THEN** the new connection and its inventory are unaffected

#### Scenario: Workstations connect to different server instances
- **GIVEN** A and B connect to different healthy instances of the upgraded server
- **WHEN** both advertise P and the user queries P on either instance
- **THEN** both devices appear once and neither connection replaces the other

#### Scenario: Server instance stops answering
- **GIVEN** a server instance owns A's relay and stops answering
- **WHEN** existing server-instance liveness detection marks it stale
- **THEN** A is no longer reported available even without graceful relay teardown

#### Scenario: One device disconnects
- **GIVEN** A and B both advertise P
- **WHEN** A disconnects and its connection is removed
- **THEN** P remains available on B and A is absent from P's connected workstation results

### Requirement: Query authorized project workstations
An authenticated caller SHALL be able to query the connected workstations advertising a project they can access. The response SHALL be isolated to that caller's user and contain unique devices. No available workstation SHALL produce an empty result. Inaccessible or unknown projects SHALL be refused.

#### Scenario: Overlapping availability
- **GIVEN** A and B advertise P and A has multiple connections advertising P
- **WHEN** their user queries P
- **THEN** A and B each appear once

#### Scenario: User isolation
- **GIVEN** another user has a workstation advertising P
- **WHEN** the caller queries workstation availability
- **THEN** that other user's workstation and inventory are not disclosed

### Requirement: Keep workstation details local
Inventory and availability responses SHALL contain project/device identifiers and connection metadata only. Repository paths, command templates, credentials and hardware/provider/tool inventories SHALL remain outside this feature. The server SHALL validate advertised projects against the authenticated user's project access. Invalid snapshots SHALL leave the previously accepted snapshot intact.

#### Scenario: Unauthorized inventory
- **GIVEN** a workstation submits a snapshot with an inaccessible project or malformed inventory
- **WHEN** the server validates it
- **THEN** the complete snapshot is rejected and cannot overwrite accepted availability

### Requirement: Preserve execution compatibility
This feature SHALL expose availability without automatically selecting a workstation from that inventory. Existing dispatch semantics SHALL remain available through a compatibility route. An operation, launch, cancellation or response SHALL remain tied to its actual owning connection. Legacy connections without inventory SHALL have unknown availability and SHALL NOT be reported as verified available for any project.

#### Scenario: Legacy wildcard agent
- **GIVEN** a connected legacy agent registers all/default and sends no inventory
- **WHEN** the user queries a project's available workstations
- **THEN** that registration alone contributes no verified workstation while legacy dispatch remains supported

#### Scenario: A new device connects during an operation
- **GIVEN** an operation was assigned to A
- **WHEN** B connects or becomes the compatibility route
- **THEN** the pending operation and its responses remain bound to A
