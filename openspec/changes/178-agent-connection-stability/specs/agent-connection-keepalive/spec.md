## ADDED Requirements

### Requirement: A keepalive reply is never lost to a concurrent write
The agent SHALL answer a server keepalive probe even while it is writing another message on the
same connection, and SHALL NOT let the reply be discarded because the connection's write path
is busy. A reply that still cannot be sent SHALL be logged with the reason.

#### Scenario: A probe arrives during a long write
- **GIVEN** a connected agent writing an operation result that takes longer than a second to flush
- **WHEN** the server sends a keepalive probe
- **THEN** the agent answers the probe
- **AND** the connection stays registered on the server

#### Scenario: Several probes arrive before the first reply is sent
- **GIVEN** a connected agent whose reply to a probe has not been written yet
- **WHEN** further probes arrive
- **THEN** the agent still answers
- **AND** it does not accumulate one pending reply per probe

#### Scenario: The reply cannot be written at all
- **GIVEN** a connection whose write path is unusable
- **WHEN** the agent fails to answer a probe
- **THEN** the failure is logged with the reason it could not be sent

### Requirement: The keepalive period and the drop deadline keep a stated margin
The agent heartbeat period and the server read timeout SHALL NOT be equal, and SHALL be
separated by a margin allowing several consecutive keepalives to be lost before a connection is
dropped. Each value SHALL carry a comment justifying the margin.

#### Scenario: An idle connection survives a lost keepalive
- **GIVEN** a connected, idle agent
- **WHEN** a single keepalive does not reach the server
- **THEN** the connection is not dropped

#### Scenario: A silent connection is still dropped
- **GIVEN** a connection that produces no frame at all
- **WHEN** the read timeout elapses
- **THEN** the server drops and unregisters it

### Requirement: A disconnection states its reason where the user can read it
When the server drops an agent connection for silence, it SHALL close it with a distinct code
and a reason naming the silence, and the agent SHALL log the code and reason it received rather
than reporting an anonymous abnormal closure.

#### Scenario: The server drops a silent agent
- **GIVEN** a connection that has produced no frame within the read timeout
- **WHEN** the server closes it
- **THEN** the close carries a code distinct from the session-rebound code and a reason naming the silence

#### Scenario: The agent reports why it was disconnected
- **GIVEN** an agent whose connection is closed with a code and a reason
- **WHEN** it detects the disconnection
- **THEN** it logs that code and reason before reconnecting

### Requirement: A local operation tolerates a reconnecting agent
A workspace operation addressed to a project whose agent is momentarily absent SHALL wait for an
agent to reconnect, bounded by both a short grace period and the caller's own deadline, before
failing. If no agent reconnects, the error SHALL state that the agent is reconnecting and that
the call can be retried. An operation for a project that has had no agent for a long time SHALL
NOT wait, and SHALL keep its immediate answer.

#### Scenario: The agent reconnects during the grace period
- **GIVEN** a stage transition requesting git evidence while the agent is reconnecting
- **WHEN** the agent reconnects within the grace period
- **THEN** the operation is sent to the reconnected agent and answered normally

#### Scenario: No agent reconnects
- **GIVEN** a workspace operation for a project with no connected agent
- **WHEN** the grace period elapses with no agent
- **THEN** the call fails with an error naming the reconnection and the retry

#### Scenario: The caller's deadline is shorter than the grace period
- **GIVEN** a caller whose remaining deadline is shorter than the grace period
- **WHEN** no agent reconnects
- **THEN** the call fails on the caller's deadline and does not wait beyond it

#### Scenario: The project never had an agent
- **GIVEN** a project whose slot has not held an agent recently
- **WHEN** a workspace operation is addressed to it
- **THEN** the call fails immediately without waiting out the grace period
- **AND** the error does not describe the agent as reconnecting
