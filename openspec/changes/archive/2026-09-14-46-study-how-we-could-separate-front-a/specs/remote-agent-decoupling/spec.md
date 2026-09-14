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

### Requirement: Typed MCP workflow operations

TaskFlow SHALL expose task lookup, stage transition, comment creation, task listing,
and project context tools over Streamable HTTP and a database-free stdio bridge.

#### Scenario: Invalid stage input
- **WHEN** a client supplies an unknown stage or omits the report note
- **THEN** the tool rejects the call without changing task state.

#### Scenario: Report persistence failure
- **WHEN** a stage transition cannot persist its tracker activity
- **THEN** the local stage update is rolled back and no tracker job is dispatched.

#### Scenario: Stdio client discovery
- **WHEN** a client initializes `taskflow mcp` against an authenticated server or gateway
- **THEN** it discovers the same five tool schemas and can invoke them over stdio.

### Requirement: API-only execution configuration

The local agent SHALL consume a versioned configuration API without opening a
local database or interpreting server filesystem paths as workstation paths.

#### Scenario: Refresh and local overrides
- **WHEN** a workflow dispatch arrives
- **THEN** the agent refreshes the actual task project's configuration, applies
  workstation overrides, validates or creates the local worktree, and scaffolds skills.

#### Scenario: Existing custom skill
- **WHEN** configuration refresh encounters a manually modified generated skill
- **THEN** the local edit is backed up and its backup path reported in the agent log
- **AND** the effective server-managed content replaces the installed skill.

#### Scenario: Unknown contract or unavailable API
- **WHEN** the server returns an unsupported schema version or cannot be reached
- **THEN** execution fails before launching an AI process.

#### Scenario: Machine API authentication
- **WHEN** a request presents an invalid bearer credential while TASKFLOW_SERVER_TOKEN is configured
- **THEN** the MCP/configuration endpoint rejects the request.

### Requirement: Automatic LLM MCP bootstrap

Before launching the targeted supported LLM CLI, the local agent SHALL register
TaskFlow's stdio bridge in that CLI's project configuration using the active gateway.

#### Scenario: Existing client configuration
- **WHEN** a supported client already has settings and other MCP servers
- **THEN** bootstrap preserves their values and updates only TaskFlow's connection entry.

#### Scenario: Malformed client configuration
- **WHEN** existing MCP configuration cannot be parsed
- **THEN** the agent reports a launch failure without overwriting the file.

### Requirement: Confirmed external terminal launch

External terminal requests SHALL accept an omitted skill ID and report local
launch failures to the initiating browser request.

#### Scenario: Open the configured interactive agent
- **WHEN** the user selects External terminal without choosing a skill
- **THEN** the local agent bootstraps MCP and launches its configured interactive CLI.

#### Scenario: Native launcher failure
- **WHEN** the native terminal launcher exits immediately with an error
- **THEN** the request fails with the launcher error instead of reporting success
  or silently starting a hidden PTY.

### Requirement: Native client pickup without a chat companion

The local agent SHALL support native coding clients without a separate TaskFlow
chat application. On connection for an explicit project, it SHALL scaffold the
project skills and register the local MCP bridge in the selected repository.

#### Scenario: User starts pickup in a native client
- **WHEN** the local agent connects for an explicitly configured project
- **THEN** the selected repository contains the provider MCP configuration and project skills
- **AND** the user can invoke pickup in the native coding client using server MCP tools
- **AND** the native coding client owns its conversation and approval interface

#### Scenario: User starts pickup from the web
- **WHEN** a pickup skill is dispatched to the connected local agent
- **THEN** the agent prepares the task worktree and its MCP configuration
- **AND** launches the configured native coding CLI with the pickup skill and task key

### Requirement: Validated fresh configuration and explicit launch intent

The server and agent SHALL use the documented v1 configuration and dispatch
contract in `docs/contracts/server-agent-v1.md`. Configuration SHALL be downloaded
for every launch, with no fallback to a stored snapshot. The launch message SHALL
carry task/skill intent rather than a duplicate of effective execution settings.

#### Scenario: Invalid project initialization
- **WHEN** an explicitly configured project cannot be fetched or validated
- **THEN** the agent does not register its execution WebSocket
- **AND** reports the configuration error including the server diagnostic when available

#### Scenario: Conflicting local and remote terminal settings
- **WHEN** a launch has effective server settings and explicit local overrides
- **THEN** the agent follows the documented precedence and uses the effective terminal mode
- **AND** reports terminal launch failure without silently switching to a hidden PTY

#### Scenario: Server retires a managed skill
- **WHEN** a previously installed skill is absent from the new configuration
- **THEN** unchanged managed files are backed up and removed
- **AND** customized retired files and unrelated personal skills remain untouched

#### Scenario: Invalid skill destination
- **WHEN** any skill has an invalid or duplicate installation destination
- **THEN** the agent rejects the complete skill list before installing any file

### Requirement: Local desktop console ownership
The web MUST delegate skill execution to the local agent and MUST NOT expose
embedded PTY consoles or local Git diff, branch and worktree controls.
The desktop MUST present native terminals and stop controls without implementing
a provider-specific chatbot.

#### Scenario: Reopen the console host
- **WHEN** the user closes and reopens the desktop while its agent remains running
- **THEN** the same executions and bounded terminal replay remain available

#### Scenario: Missing local agent
- **WHEN** a web user launches a skill without a connected local agent
- **THEN** the server rejects the launch rather than running it server-side
