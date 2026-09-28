## ADDED Requirements

### Requirement: A recovered agent retries promptly
The local agent SHALL count consecutive failed connection attempts separately from completed WebSocket sessions, while retaining bounded retry delay after a loss.

#### Scenario: Repeated failures before a session
- **GIVEN** repeated attempts fail before a WebSocket session is established
- **WHEN** the agent schedules another attempt
- **THEN** the delay increases up to its existing upper bound

#### Scenario: Established session closes
- **GIVEN** earlier connection failures or losses
- **WHEN** the agent establishes a WebSocket session and later loses it
- **THEN** its next retry starts at the initial delay, without inheriting earlier attempts

### Requirement: Launches tolerate brief agent reconnection
Sectile SHALL give a recently connected agent a short opportunity to return before refusing a new launch, within the caller's deadline and before recording a run.

#### Scenario: Agent returns within the grace period
- **GIVEN** an agent was recently connected and is temporarily absent
- **WHEN** a user launches a skill and the agent reconnects within the grace period
- **THEN** Sectile dispatches the launch once to the recovered agent and reports its confirmation

#### Scenario: Agent does not return
- **GIVEN** an agent was recently connected and is temporarily absent
- **WHEN** the grace period or caller deadline ends
- **THEN** Sectile refuses the launch with a French reconnection message and records no run or launch activity

#### Scenario: Agent has never connected
- **GIVEN** no agent has connected for the project recently
- **WHEN** a user launches a skill
- **THEN** Sectile refuses immediately with its existing connection guidance

#### Scenario: Confirmation is lost after dispatch
- **GIVEN** a launch was sent to an agent
- **WHEN** its confirmation is lost with the connection
- **THEN** Sectile does not automatically dispatch the same launch again

### Requirement: Disconnect diagnostics reflect received evidence
The agent SHALL distinguish a transport loss without a close frame from a received server close reason.

#### Scenario: Abnormal close
- **GIVEN** the WebSocket ends with synthetic code 1006 and unexpected EOF
- **WHEN** the agent reports the loss
- **THEN** its French runtime message does not claim that the server deliberately disconnected it

#### Scenario: Explicit close reason
- **GIVEN** the server sends a close frame with a code and reason
- **WHEN** the agent reports the loss
- **THEN** it preserves the received code and reason
