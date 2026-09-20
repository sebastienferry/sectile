## ADDED Requirements

### Requirement: Update a task through the MCP interface
The MCP catalog SHALL expose an `update_task` tool that updates mutable descriptive fields of an existing task and returns the updated task under a `task` key. It SHALL accept `taskKey` as a required argument, and optional `title`, `description`, `priority`, `issueType`, and `labels`. It SHALL NOT accept workflow stage, status, branch, or PR linkage arguments.

#### Scenario: Update title and description of an existing task
- **GIVEN** an existing task on any project
- **WHEN** an authenticated session calls `update_task` with the task's key, a new title, and a new description
- **THEN** the task's title and description are updated in the store
- **AND** the response contains the updated task
- **AND** an asynchronous tracker update is enqueued.

#### Scenario: Clear a task description
- **GIVEN** an existing task with a non-empty description
- **WHEN** `update_task` is called with the task's key and `description: ""`
- **THEN** the task's description is cleared to an empty string
- **AND** the response reflects the cleared description.

#### Scenario: Update priority and issue type
- **GIVEN** an existing task
- **WHEN** `update_task` is called with a valid `priority` ("low", "medium", "high", or "urgent") and a valid `issueType`
- **THEN** the task's priority and issue type are updated accordingly.

#### Scenario: Update custom labels while preserving workflow stage
- **GIVEN** an existing task in workflow stage `clarified` carrying `#clarified`
- **WHEN** `update_task` is called with `labels: ["frontend", "urgent", "#specified"]`
- **THEN** the foreign stage label `#specified` is stripped
- **AND** the task retains its current workflow stage label `#clarified`
- **AND** the custom labels `frontend` and `urgent` are saved on the task.

### Requirement: Validation of update inputs
`update_task` SHALL validate that `taskKey` is present and non-blank, that at least one mutable field is supplied, that `title` is not blank if provided, and that the targeted task exists.

#### Scenario: Reject missing or blank taskKey
- **GIVEN** an `update_task` call where `taskKey` is omitted, empty, or whitespace only
- **WHEN** the call is dispatched
- **THEN** it fails with an error indicating `taskKey is required`
- **AND** no task is modified.

#### Scenario: Reject update with no mutable fields
- **GIVEN** an `update_task` call with a valid `taskKey` but no mutable fields (`title`, `description`, `priority`, `issueType`, `labels` are all omitted)
- **WHEN** the call is dispatched
- **THEN** it fails with an error indicating that at least one mutable field must be provided
- **AND** no task is modified.

#### Scenario: Reject empty or blank title
- **GIVEN** an `update_task` call where `title` is provided as an empty string or whitespace only
- **WHEN** the call is dispatched
- **THEN** it fails with an error indicating that title cannot be empty
- **AND** the task's title remains unchanged.

#### Scenario: Reject non-existent task
- **GIVEN** an `update_task` call with a `taskKey` that does not match any existing task ID or key
- **WHEN** the call is dispatched
- **THEN** it fails with a task not found error naming the missing task key.

### Requirement: Workflow invariants preservation
`update_task` SHALL NOT alter the task's workflow stage or status, and SHALL NOT accept workflow status arguments in its schema.

#### Scenario: Stage progression remains invariant under update_task
- **GIVEN** an existing task in stage `clarified` with status `clarified`
- **WHEN** `update_task` updates the task's title and description
- **THEN** the task's stage remains `clarified`
- **AND** the task's status remains unchanged.

### Requirement: Caller attribution and tracker synchronization
`update_task` SHALL attribute the modification to the authenticated MCP caller and enqueue tracker synchronization under that caller's identity.

#### Scenario: Enqueue tracker update attributed to acting user
- **GIVEN** an authenticated MCP session associated with a user
- **WHEN** `update_task` successfully modifies a task on a tracker-backed project
- **THEN** a tracker update operation is queued carrying the acting user's ID
- **AND** tracker credentials resolve for that acting user.
