## ADDED Requirements

### Requirement: Create a task through the MCP interface
The MCP catalog SHALL expose a `create_task` tool that creates a Sectile task and
returns it under a `task` key, including its allocated `key` and its `externalUrl`
when the tracker provides one. It SHALL accept `projectId` and `title` as required
arguments, and `description`, `issueType`, `priority`, `labels` and `parentKey` as
optional ones. It SHALL NOT accept a status, a source or an external URL.

#### Scenario: Create a task on a tracker-backed project
- **GIVEN** a project whose tracker supports issue creation
- **WHEN** a session calls `create_task` with that project's identifier and a title
- **THEN** the task is created once, on that project
- **AND** the result carries the task with its key and its external URL
- **AND** no equivalent task is created through any other endpoint.

#### Scenario: Supply the optional descriptive arguments
- **GIVEN** a valid project identifier and title
- **WHEN** `create_task` also supplies a description, issue type, priority, labels or a parent key
- **THEN** the created task carries each supplied value
- **AND** omitting any of them leaves the corresponding project or board default in place.

### Requirement: Creation names its project explicitly
`create_task` SHALL require a non-empty `projectId` and SHALL NOT infer the project
from ambient session context, from a task key or from a default project.

#### Scenario: Reject a missing or blank project
- **GIVEN** a `create_task` call whose `projectId` is absent, empty or whitespace
- **WHEN** the call is dispatched
- **THEN** it fails with an error naming the missing project identifier
- **AND** no task is created on any project.

#### Scenario: Reject a blank title
- **GIVEN** a `create_task` call whose `title` is absent, empty or whitespace
- **WHEN** the call is dispatched
- **THEN** it fails with an error naming the missing title
- **AND** no task is created.

### Requirement: Agent creation never degrades to a local-only ticket
`create_task` SHALL require remote creation, so that a project whose tracker cannot
create issues remotely fails the call instead of producing a task that exists only
in the local board. Projects whose tracker is local SHALL remain creatable.

#### Scenario: Tracker cannot create remotely
- **GIVEN** a project backed by a tracker that does not support remote creation
- **WHEN** `create_task` is called for that project
- **THEN** the call fails and names the unsupported tracker
- **AND** no task is persisted locally.

#### Scenario: Local project
- **GIVEN** a project whose tracker is local
- **WHEN** `create_task` is called for that project
- **THEN** the task is created on the local board.

### Requirement: Created tasks enter the workflow at its first stage
A task created through `create_task` SHALL receive the same status and workflow
label as any other newly created task, and the tool SHALL NOT offer a way to place
it at a later stage.

#### Scenario: Read back a task created through MCP
- **GIVEN** a successful `create_task` call
- **WHEN** the created task is read back
- **THEN** its status is `to_clarify`
- **AND** `new` is its only workflow label
- **AND** any supplied custom labels are retained.
