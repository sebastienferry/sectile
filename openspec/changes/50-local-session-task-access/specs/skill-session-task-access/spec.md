## ADDED Requirements

### Requirement: Task reads survive tracker unavailability
The task-management interface SHALL return a task that is known locally even when its tracker comments cannot be retrieved, and SHALL state the retrieval failure explicitly.

#### Scenario: Tracker credential missing
- **GIVEN** a tracker-backed task stored locally and a server with no usable tracker credential
- **WHEN** a launched skill session reads the task
- **THEN** the task is returned with its identity, status, branch and description
- **AND** the comment retrieval failure is reported as an explicit error field
- **AND** no empty comment list is presented as the ticket's discussion

#### Scenario: Comments available
- **GIVEN** a task whose tracker comments can be retrieved
- **WHEN** a session reads the task
- **THEN** the comments are returned and no retrieval error is reported

#### Scenario: Unknown task
- **GIVEN** a task key that matches no stored task
- **WHEN** a session reads it
- **THEN** the read fails with a not-found error

### Requirement: Project context stays consumable by a session
The project-context interface SHALL return project identity, execution settings, specification framework, pull-request creation stage and per-skill references without inlining skill or command bodies.

#### Scenario: Context read by a launched session
- **GIVEN** a project configured with several generated skills
- **WHEN** a launched skill session reads the project context
- **THEN** the payload carries the project id, repository identity, tracker, specification framework and pull-request creation stage
- **AND** each skill is represented by its id, directory, command name and resolved file paths
- **AND** no skill body or command body is included in the payload
