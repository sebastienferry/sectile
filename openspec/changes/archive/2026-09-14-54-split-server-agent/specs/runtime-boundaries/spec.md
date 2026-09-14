## ADDED Requirements

### Requirement: Independent runtime artifacts
The system SHALL distribute separate `taskflow-server` and `taskflow-agent` binaries without a unified executable shim.

#### Scenario: Independent agent build
- **GIVEN** a Go checkout without installed frontend dependencies
- **WHEN** an operator builds the agent target
- **THEN** the agent executable is produced without building or embedding the web UI

#### Scenario: Desktop packaging
- **GIVEN** a packaged desktop application
- **WHEN** it starts local execution
- **THEN** it launches the bundled agent directly without starting a server or opening a database

### Requirement: Headless control plane
The server SHALL own persisted task management, board/roadmap/sprint configuration, tracker synchronization, upstream HTTP MCP and authenticated agent dispatch, without running Git, CLI, terminal or LLM subprocesses or accessing workstation repository contents.

#### Scenario: Server without a workstation
- **GIVEN** a server with explicit tracker configuration and no local repository or CLI tools
- **WHEN** users read or update tracker-backed tasks
- **THEN** the server performs the supported tracker operations and persists their actual results
- **AND** no agent connection is required for tracker synchronization

#### Scenario: Tracker failure
- **GIVEN** invalid tracker credentials or an unavailable tracker
- **WHEN** synchronization or a queued mutation executes
- **THEN** the failure remains visible and the operation is not reported as successful

### Requirement: Agent-owned local execution
The agent SHALL own Git, worktree, pull-request, editor, terminal and LLM process operations for its connected projects.

#### Scenario: Disconnected execution request
- **GIVEN** no matching connected agent
- **WHEN** a local workspace or LLM operation is requested
- **THEN** the system reports agent unavailability
- **AND** the server does not attempt local execution

#### Scenario: Workflow completion
- **GIVEN** an agent executing a task in its assigned workspace
- **WHEN** the execution finishes
- **THEN** the server records verified workflow results separately from process-launch acknowledgement
- **AND** cancellation and failures cannot falsely advance the task

### Requirement: Three-party interface contract
The project SHALL document Server API, Agent Loopback and MCP ownership, authentication, addresses, request/response and error behavior, and migration from unified executable invocations.

#### Scenario: Native client configuration
- **GIVEN** a native coding client configured for the agent MCP bridge
- **WHEN** it reads a task, records a transition or starts and finishes a run
- **THEN** it uses the existing supported MCP catalog and server-owned state
- **AND** the agent does not open a local task database
