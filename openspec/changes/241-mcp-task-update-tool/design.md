# Design

## Decisions

### The tool delegates to `database.UpdateTaskBy`, preserving single-path updates and tracker sync
`database.UpdateTaskBy` (`internal/db/db.go:2325`) already handles updating task fields in SQLite, records the updated timestamp, re-indexes pinned status if changed, and automatically calls `d.enqueueTrackerUpdateAsUnsafe` under the acting user's ID (`actor.ID`) for any modified title, description, priority, or labels. Implementing `update_task` as a typed MCP wrapper around `UpdateTaskBy` guarantees that updates made via MCP follow the exact same persistence, validation, and tracker synchronization paths as the web/HTTP API, avoiding code duplication or synchronization drift.

### `taskKey` is required and resolves by key or primary ID
`taskKey` is marked required with `minLength: 1`. It matches either the task's local primary ID (e.g. `gh-ef5a2777-920f-4744-a7e8-a58a4c257a23-241`) or its human-readable key (e.g. `#241`), delegating to `database.GetTaskByID` / `getTaskByIDUnsafe`. If no task exists matching `taskKey`, the call fails with a clear error: `task not found: <taskKey>`.

### Explicit validation: at least one mutable field must be provided
Calling `update_task` with only `taskKey` is a no-op that signifies a malformed agent request. The tool requires at least one of `title`, `description`, `priority`, `issueType`, or `labels` to be non-nil. If none are provided, the handler returns an error: `at least one mutable field (title, description, priority, issueType, labels) must be provided`.

### Empty strings vs omitted fields
- **`title`**: A title cannot be empty or whitespace only. If `title` is provided as non-nil but `strings.TrimSpace(*title) == ""`, the handler returns `title cannot be empty`.
- **`description`**: A description can be updated to new markdown content, or explicitly cleared by providing an empty string `""`. Omitted `description` (`nil`) leaves the existing description unchanged.
- **`priority`**: Optional enum restricted to `"low"`, `"medium"`, `"high"`, or `"urgent"`.
- **`issueType`**: Optional string updating the task's issue type (e.g. "Task", "Bug").
- **`labels`**: Optional list of strings.

### Workflow stage invariants and label sanitization
`update_task` intentionally omits `status`, `stage`, `trackerStatus`, `branchName`, `prUrl`, and `prLinks` from its arguments:
1. **Workflow transitions are exclusive to `transition_stage`**: Advancing or regressing workflow stages requires stage transition notes and audit trail activities.
2. **Workflow labels are protected**: In Sectile, workflow stage labels (`#new`, `#clarified`, `#specified`, `#implemented`, `#reviewed`, `#finished`, `#closed`, case-insensitive, with or without `#`) reflect the stage of the task. If an agent passes custom labels via `labels`, all workflow stage labels are stripped from the incoming list, and the task's current workflow stage label (derived from `existing.Status` via `GetStageLabelForStatus` or `existing.Labels`) is preserved via `db.SetWorkflowLabel`. This prevents an agent from inadvertently clearing or falsifying the task's stage through a label update.

### Caller attribution
The MCP server uses `caller := callerOf(resolve, req)` to identify the authenticated caller. The actor `db.Actor{ID: caller.UserID, Name: caller.Name}` is passed to `database.UpdateTaskBy`, ensuring queued tracker updates (such as GitHub issue title/body edits or Jira field updates) carry the user's ID and resolve the user's personal tracker token.

### Catalog expansion from nine to ten canonical tools
`cmd/agent/mcp.go` (and `internal/agentmcp/mcp.go`) runs a stdio proxy that validates the catalog against a whitelist and enforces the exact expected tool count. Relaxing the count check would permit skewed installations; instead, both the proxy and the test harnesses (`internal/mcptest/contract.go`, `internal/handlers/mcp_test.go`, `internal/agent/agent_config_test.go`) explicitly step from 9 to 10 tools.

## Rejected Alternatives

- **A dedicated `edit_task_description` tool**:
  While `rewrite-story` primarily updates the description, other workflows need to update task titles, labels, or priorities. Adding individual single-field tools would bloat the MCP catalog. A single `update_task` tool with optional fields cleanly serves all descriptive mutation needs.
- **Allowing `status` in `update_task`**:
  Allowing arbitrary status updates in `update_task` would bypass stage precondition checks, required review notes, and PR validation enforced by `transition_stage`. Status must remain exclusive to `transition_stage`.
- **Allowing agents to overwrite stage labels via `labels`**:
  Allowing arbitrary strings in `labels` without filtering would let an agent set `#implemented` or remove `#clarified` without going through `transition_stage`, desynchronizing board columns and tracker statuses. Filtering workflow stage labels protects board state integrity.
- **Requiring all fields or full task replacement**:
  Requiring callers to pass all fields would force agents to read every field first, risking race conditions and clobbering fields they did not intend to touch. Partial updates with pointer-based optional fields (`models.UpdateTaskRequest`) provide safe, targeted mutations.
