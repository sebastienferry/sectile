## Purpose

Defines functional requirements and acceptance criteria for `remote-agent-decoupling`, allowing TaskFlow's Web UI and Database to run on a remote server while dispatching LLM workflow actions, Git worktree operations, and interactive PTY sessions to a connected local background agent daemon over a secure WebSocket relay.

## ADDED Requirements

### Requirement: Outbound Agent WebSocket Registration
The TaskFlow server SHALL accept outbound WebSocket connection requests at `/ws/agent-connect` and register connected local agents using valid authentication tokens.

#### Scenario: Successful agent connection registration
- **GIVEN** a local agent daemon running with a valid user authentication token
- **WHEN** the agent connects outward to `wss://<remote-server>/ws/agent-connect`
- **THEN** the server validates the token, registers the agent connection under the user's ID, and marks the user's agent status as `Connected`.

#### Scenario: Rebinding existing agent session
- **GIVEN** an active agent connection already registered for a user and project mapping
- **WHEN** a new agent daemon connects for the same user and project
- **THEN** the server terminates the older connection with close code `4001 Session Rebound` and registers the new agent daemon.

### Requirement: User Identity Matching for Command Dispatch
The TaskFlow backend SHALL dispatch task workflow actions to a local agent ONLY IF the requesting Web UI user session matches the connected agent's authenticated user ID.

#### Scenario: Dispatching command with matching user identity
- **GIVEN** a Web UI session authenticated as User `A` and an active local agent registered to User `A`
- **WHEN** User `A` triggers a workflow action (e.g. `/clarify-issue`, `/code-issue`, or terminal launch) on a task card
- **THEN** the remote server serializes the command and sends the payload to User `A`'s connected local agent over WebSocket.

#### Scenario: Attempting command dispatch when agent is disconnected
- **GIVEN** a Web UI session authenticated as User `A` with no active local agent connected
- **WHEN** User `A` attempts to trigger a workflow action on a task card
- **THEN** the remote server rejects the request with HTTP 428 Precondition Required and prompts the user to start `taskflow agent`.

### Requirement: Local LLM Workflow Step Execution
The local agent daemon SHALL receive workflow commands from the remote server and execute them locally within isolated Git worktrees.

#### Scenario: Executing workflow step locally
- **GIVEN** a local agent daemon connected to the remote server
- **WHEN** the local agent receives a `dispatch_step` message for task `#46`
- **THEN** the agent creates or checks out the local Git worktree `.tasks/worktrees/#46`, executes the specified LLM skill, and streams activity updates back to the remote server.

### Requirement: Bi-Directional PTY Terminal Streaming
The TaskFlow system SHALL stream interactive terminal keystrokes and ANSI terminal output bi-directionally between Xterm.js on the remote Web UI and the local agent's PTY master.

#### Scenario: Real-time terminal interaction
- **GIVEN** an active terminal session open in the remote Web UI for task `#46`
- **WHEN** the user types keystrokes in Xterm.js
- **THEN** keystrokes are transmitted via WebSocket to the remote server, relayed to the local agent, written to the local PTY master, and stdout ANSI output streams back to the browser.
