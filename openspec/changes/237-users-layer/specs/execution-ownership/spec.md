## ADDED Requirements

### Requirement: Executions record their owner
Every execution SHALL record the user who started it: the signed-in user for a launch
from the interface, the user bound to the API key for a run reported over MCP, and the
agent's user for a run the agent owns. Comments SHALL record the acting user. Records
created before this change SHALL keep an empty owner and SHALL be treated as belonging
to no one.

#### Scenario: Run reported over MCP
- **GIVEN** an MCP client authenticated with the API key of a user
- **WHEN** it reports the start of a run
- **THEN** the run lists that user as its owner

#### Scenario: Run launched from the interface
- **GIVEN** a signed-in user
- **WHEN** they launch a skill on a task
- **THEN** the resulting execution lists them as its owner

### Requirement: Everyone sees every execution
Every signed-in user SHALL see all running executions on the tickets they display,
including executions owned by other users, each with its owner's name.

#### Scenario: Colleague's run is visible
- **GIVEN** two users, one of whom has a running execution on a task
- **WHEN** the other opens that task
- **THEN** the execution is shown as running, with the first user's name

### Requirement: Only the owner or an admin stops an execution
Stopping an execution SHALL be allowed to its owner and to admins only. A member's
attempt on another user's execution SHALL be refused as not allowed and SHALL leave
the execution running. The stop SHALL be delivered to the owner's agent, and an
execution SHALL be closed as orphaned only when the owner's agent reports that it does
not have it. An execution without a recorded owner SHALL be stoppable by admins only.

#### Scenario: Member stops a colleague's run
- **GIVEN** a running execution owned by one user
- **WHEN** another user who is a member asks to stop it
- **THEN** the request is refused as not allowed
- **AND** the execution is still running, on the server and on the owner's agent

#### Scenario: Owner stops their run
- **GIVEN** a running execution
- **WHEN** its owner asks to stop it
- **THEN** the stop reaches their agent and the execution ends as canceled

#### Scenario: Admin stops a colleague's run
- **GIVEN** a running execution owned by a member whose agent is connected
- **WHEN** an admin asks to stop it
- **THEN** the stop reaches the member's agent and the execution ends as canceled

#### Scenario: A client reports the end of a run it does not own
- **GIVEN** a running execution owned by one user
- **WHEN** another user's client reports that run finished
- **THEN** the report is refused and the execution stays running
- **AND** the workflow is not handed back

#### Scenario: Admin stops a run whose owner's agent is gone
- **GIVEN** a running execution whose owner has no connected agent
- **WHEN** an admin asks to stop it without forcing
- **THEN** the request reports that the agent cannot be reached and the execution stays open

### Requirement: A member only triggers executions on their own agent
Launching a skill, dispatching a step or sending console input SHALL reach the agent of
the signed-in user only, whatever target user a request names. An admin MAY name
another user's agent. A launch on a project where the user has no connected agent
SHALL be refused as it is today.

#### Scenario: Member names a colleague's agent
- **GIVEN** a signed-in member and a connected agent belonging to another user
- **WHEN** the member sends a dispatch naming the other user
- **THEN** the dispatch is routed to the member's own agent, or refused when they have none
- **AND** nothing reaches the other user's agent

#### Scenario: Admin dispatches to another agent
- **GIVEN** a signed-in admin and a connected agent belonging to another user
- **WHEN** the admin sends a dispatch naming that user
- **THEN** the dispatch reaches that user's agent
