## ADDED Requirements

### Requirement: A user creates an API key from the profile and sees it once
The web profile SHALL let a signed-in user create an API key with a label and an expiry of
90 days by default or none, SHALL display the key exactly once at creation, and SHALL store
only its hash. Keys SHALL carry the `sectile_` prefix.

#### Scenario: Create a key
- **GIVEN** a signed-in user on the profile
- **WHEN** they create a key labelled "laptop"
- **THEN** the key is shown once with its expiry date
- **AND** later listings show label, creation, last use and expiry but never the key

### Requirement: One key authenticates every machine surface
A valid API key presented as a bearer credential SHALL be accepted by the agent WebSocket
handshake, `/api/v1/agent/*`, `/mcp` on the server and the agent gateway. No surface SHALL
require a different credential.

#### Scenario: Direct MCP call without an agent
- **GIVEN** a key and no running local agent
- **WHEN** an MCP client calls `list_tasks` on the server's `/mcp` with the key
- **THEN** the call succeeds
- **AND** an agent-routed tool reports that no agent is connected

#### Scenario: Gateway accepts the key
- **GIVEN** a running agent started with a key
- **WHEN** a local process calls the gateway's `/mcp` with that key
- **THEN** the call is proxied to the server

### Requirement: Keys expire, can be renewed and can be revoked
An API key past its expiry SHALL be refused on every surface with the distinct reason
`API key expired`. Renewal SHALL extend the expiry by the key's period without changing
the secret. Revocation SHALL cut every surface at once.

#### Scenario: Expired key
- **GIVEN** a key whose expiry has passed
- **WHEN** the agent connects with it
- **THEN** the handshake is refused with `API key expired`
- **AND** the agent log names the expiry rather than an invalid token

#### Scenario: Renewal keeps configurations working
- **GIVEN** a key configured in a CLI and in an agent
- **WHEN** the user renews it
- **THEN** both keep working unchanged with the new expiry

### Requirement: A pairing code is a one-time exchange for a key
The agent CLI and the desktop connect screen SHALL exchange a pairing code once for a key
with the default expiry, store it on the workstation with owner-only permissions, and start
from it without any environment variable.

#### Scenario: Pair from the CLI
- **GIVEN** a pairing code from the profile
- **WHEN** the user runs `sectile-agent pair` then starts the agent
- **THEN** the agent connects with the stored key
- **AND** the code is spent

### Requirement: Existing workstations migrate without action
Device credentials issued before this change SHALL keep working as keys without expiry, and
SHALL be listed as such with the option to set one. `SECTILE_SERVER_TOKEN` SHALL be accepted
for one release with a startup warning, then removed.

#### Scenario: Upgrade with a paired desktop
- **GIVEN** a desktop paired before the upgrade
- **WHEN** the server restarts on the new version
- **THEN** the desktop reconnects unchanged
- **AND** the profile lists it with no expiry

### Requirement: Expiry is announced ahead
The agent SHALL log the remaining validity at connect when it is under ten days, and the
profile SHALL flag such keys.

#### Scenario: Nine days left
- **GIVEN** a key expiring in nine days
- **WHEN** the agent connects
- **THEN** the log states the remaining days
