## ADDED Requirements

### Requirement: Membership follows orchestration responsibility

Desktop Orchestration SHALL present configured orchestration agents and genuine dedicated orchestration sessions. Ordinary ticket workers SHALL NOT appear as agents or associated sessions, including workers launched by an orchestrator. Task presence, skill name, prompt content, activity status and launch origin SHALL NOT establish orchestration responsibility. Missing or unrecognized responsibility evidence SHALL NOT grant membership.

#### Scenario: Ordinary workers do not enable a disabled project section

- **GIVEN** a connected project with orchestration disabled and only ordinary ticket executions
- **WHEN** a manual clarify or another workflow execution is preparing, pending, queued, running or waiting
- **THEN** that project has no Orchestration section and those executions remain ordinary workers

#### Scenario: Launched workers are not dedicated sessions

- **GIVEN** an enabled project with configured orchestration agents
- **WHEN** an agent evaluation launches ticket workers whose prompts name that policy and whose activity IDs occur in its evaluation report
- **THEN** those workers appear neither as standalone agents nor as policy sessions and do not determine session status or session actions

#### Scenario: Legacy and unrelated sessions do not gain responsibility

- **GIVEN** restored or newly discovered records without authoritative orchestration responsibility, including consoles, discussions and macro executions
- **WHEN** Desktop refreshes Orchestration
- **THEN** none is promoted to an orchestration session through its name, prompt, skill, session ID or active state

### Requirement: Configured policies and evaluation state remain usable

An enabled connected project's configured agents SHALL remain accessible when idle and when no genuine dedicated session exists. Existing configuration, instructions and evaluation controls and available evaluation results SHALL be preserved. A policy evaluation SHALL NOT be represented as a persistent session merely because it launches work.

#### Scenario: Idle enabled project

- **GIVEN** an enabled connected project with saved agent definitions and no dedicated session
- **WHEN** the user opens Orchestration
- **THEN** the definitions, configuration and instruction controls remain available with an accurate empty-session state

#### Scenario: Evaluation completes without a dedicated session

- **GIVEN** a configured policy with no dedicated session
- **WHEN** evaluation is pending, succeeds with launched/skipped results, or fails
- **THEN** Desktop shows the corresponding evaluation state or report while the session list remains empty and launched workers supply no session console or stop targets

### Requirement: Genuine sessions retain access and correct association

A genuine dedicated session SHALL be associated only with its own project and stable orchestration-agent identity. Connected disabled projects SHALL remain visible while such a session is active or waiting, even if its definition has been removed. Ordinary workers and finished history alone SHALL NOT retain a disabled project section. Existing disconnected-project exclusion and finished-session history bounds SHALL be preserved.

#### Scenario: Disabled project with a genuine live session

- **GIVEN** a connected project with a genuine dedicated orchestration session
- **WHEN** orchestration is disabled while that session is active or waiting
- **THEN** its Orchestration section and session access remain available without promoting any ordinary worker

#### Scenario: Last genuine session ends

- **GIVEN** a disabled project with one genuine live session and ordinary workers
- **WHEN** the dedicated session completes, fails or is canceled and no genuine live session remains
- **THEN** that project's Orchestration section disappears while the workers remain available normally

#### Scenario: Stable association and project isolation

- **GIVEN** genuine sessions for different projects and agent identities with possibly matching display names
- **WHEN** an agent is renamed or the view refreshes
- **THEN** each session remains associated only with its original project and stable agent identity

#### Scenario: Disconnected project

- **GIVEN** a disconnected project with configured agents or genuine sessions
- **WHEN** Orchestration renders
- **THEN** the project remains excluded under the existing disconnection policy

### Requirement: State refreshes preserve the responsibility boundary

Every refresh, state transition and restored snapshot SHALL preserve membership rules. Session-derived waiting, failure, authentication and stopping state and console/stop actions SHALL use only genuine eligible sessions. A stale worker-backed selection SHALL cease to expose an orchestration overview or worker actions.

#### Scenario: Mixed activity changes state

- **GIVEN** configured agents, a genuine dedicated session and ordinary workers in the same project
- **WHEN** workers change between queued, preparing, running, waiting, failed, canceled and completed states
- **THEN** they remain excluded from orchestration rows, histories, session status and actions while genuine session state remains accurate

#### Scenario: Waiting is reported within a running session

- **GIVEN** a genuine dedicated session reported as running with a user-wait indication
- **WHEN** Desktop refreshes
- **THEN** the session remains accessible with its waiting state, including after disabling the project

#### Scenario: Refresh invalidates an ineligible selection

- **GIVEN** an open overview backed by a record that is absent or ineligible in the next snapshot
- **WHEN** refreshed data is rendered or restored
- **THEN** the obsolete selection exposes no console or stop action for an ordinary worker

### Requirement: Ordinary execution behavior and capacity are preserved

The correction SHALL preserve ordinary executions in the normal project list and queue, including their existing lifecycle, ordering, cancellation and project-wide capacity accounting. Capacity information SHALL continue to describe project-wide work even when no orchestration sessions are visible.

#### Scenario: Worker remains available and counted

- **GIVEN** an ordinary ticket worker launched manually or by an orchestrator
- **WHEN** it is excluded from Orchestration
- **THEN** its normal execution access, queue position, lifecycle controls and contribution to capacity remain unchanged

### Requirement: Users receive a changelog entry

Implementation SHALL add an English user-facing Fixed entry under CHANGELOG.md [Unreleased] describing the corrected Orchestration view membership.

#### Scenario: Fix is delivered

- **GIVEN** the membership correction is implemented
- **WHEN** release notes are prepared
- **THEN** Unreleased contains one Fixed entry explaining that ordinary ticket executions no longer appear as orchestration agents
