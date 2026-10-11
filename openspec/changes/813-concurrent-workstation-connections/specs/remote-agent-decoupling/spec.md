## MODIFIED Requirements

### Requirement: Outbound Agent WebSocket Registration
The Sectile server SHALL accept outbound WebSocket connection requests at `/ws/agent-connect` with valid authentication and SHALL distinguish simultaneous workstations by authenticated user and effective device identity. The local connection registry SHALL remain in memory. Existing persisted presence and cross-instance routing SHALL be retained and SHALL use the same user/device identity.

#### Scenario: Successful agent connection registration
- **GIVEN** a local daemon with a valid user authentication token
- **WHEN** it connects to `/ws/agent-connect`
- **THEN** the server registers its user/device connection and reports it as connected.

#### Scenario: Distinct workstations connect concurrently
- **GIVEN** a user with an active device A connection
- **WHEN** device B connects for the same user, including with the same wildcard project
- **THEN** both devices remain connected without Session Rebound and both appear in connection snapshots.

#### Scenario: Rebinding existing agent session
- **GIVEN** an active connection for user U and device A
- **WHEN** another connection registers for U and A, even with a different project
- **THEN** the old connection receives `4001 Session Rebound` and the new connection replaces it without disturbing other devices.

#### Scenario: Delayed disconnect cleanup
- **GIVEN** a device connection has been replaced
- **WHEN** the old connection unregisters later
- **THEN** the replacement remains registered and other devices remain connected.

#### Scenario: Missing query device identity
- **GIVEN** a valid connection request without deviceId
- **WHEN** the server registers the connection
- **THEN** it uses the authenticated credential's device identity if available, otherwise `unknown`.

#### Scenario: User isolation
- **GIVEN** user A has an active device named X
- **WHEN** user B connects a device also named X
- **THEN** neither connection replaces the other.
