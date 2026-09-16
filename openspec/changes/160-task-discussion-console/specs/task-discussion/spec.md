## ADDED Requirements

### Requirement: Task discussion action
Sectile SHALL offer, on a task, an action that opens an agent session for discussion instead of running a workflow skill. The action SHALL be available in the web task menu, in the task detail skills area and in the desktop launch and relaunch selectors, alongside the existing skill actions.

#### Scenario: Starting a discussion from the task menu
- **GIVEN** a task in a project with a connected local agent
- **WHEN** the user triggers the discussion action
- **THEN** an agent session opens for that task
- **AND** no skill command is invoked.

#### Scenario: Discussion offered whatever the stage
- **GIVEN** a task at any workflow stage that is not finished
- **WHEN** its actions are displayed
- **THEN** the discussion action is offered
- **AND** its availability does not depend on the next workflow step.

#### Scenario: No local agent
- **GIVEN** a task whose project has no connected local agent
- **WHEN** the user triggers the discussion action
- **THEN** the request is rejected with the same explanation as a skill launch
- **AND** no session is opened.

### Requirement: Prompt-free discussion launch
A discussion SHALL invoke only the configured provider executable, with no skill command, no workflow instructions and no generated prompt, and SHALL preserve that CLI's normal interactive permission defaults.

#### Scenario: The command line carries no prompt
- **GIVEN** a discussion is dispatched for a task
- **WHEN** the command line is built
- **THEN** it contains the provider executable alone
- **AND** no skill command, task summary or run instruction contributes arguments.

#### Scenario: A carried prompt is ignored
- **GIVEN** a discussion dispatch carrying prompt text
- **WHEN** the command line is built
- **THEN** that text does not appear in the command line.

#### Scenario: Unresolvable provider
- **GIVEN** a project whose configured provider executable cannot be resolved
- **WHEN** a discussion is dispatched
- **THEN** the launch fails with the provider resolution error
- **AND** no session is started.

### Requirement: Discussion runs in the task context
A discussion SHALL run in the task's own checkout, on its branch, with the task execution environment, so the agent can read the task on request through Sectile MCP.

#### Scenario: Session location
- **GIVEN** a task with a worktree and a branch prepared by the local agent
- **WHEN** a discussion opens
- **THEN** the session's working directory is that checkout
- **AND** its environment carries the task identifier, key, branch, worktree and project identifier.

#### Scenario: No extra scaffolding
- **GIVEN** a discussion is dispatched
- **WHEN** it is prepared
- **THEN** it reuses the checkout and branch the task dispatch already resolves
- **AND** no specification artifact is created.

### Requirement: A discussion has no workflow effect
A discussion SHALL NOT transition the task stage, record a skill result, or report a workflow outcome to the tracker. It SHALL be tracked as an ordinary execution so that it is listed, selectable and stoppable.

#### Scenario: Stage is untouched
- **GIVEN** a task at a given workflow stage
- **WHEN** a discussion opens and later ends
- **THEN** the task stage, labels and status are unchanged
- **AND** no pull request is created or linked.

#### Scenario: The console is controllable
- **GIVEN** a running discussion
- **WHEN** the user views the executions list
- **THEN** the discussion appears as an execution for that task
- **AND** the user can stop it and replay its terminal history.

### Requirement: Discussion admission
The local agent SHALL admit a discussion launch request without instructions, and SHALL keep rejecting unknown skill identifiers.

#### Scenario: Admitted without instructions
- **GIVEN** a launch request naming the discussion action with an empty prompt
- **WHEN** the local agent validates it
- **THEN** the request is admitted.

#### Scenario: Unknown identifier still rejected
- **GIVEN** a launch request naming an identifier that is neither a project skill nor a reserved one
- **WHEN** the local agent validates it
- **THEN** the request is rejected.
