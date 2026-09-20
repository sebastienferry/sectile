# The Sectile MCP catalog gains a task update tool

## Why
The Sectile MCP catalog currently provides tools for reading tasks (`get_task`, `list_tasks`), creating tasks (`create_task`), and posting comments (`add_comment`), but lacks a tool to update an existing task's title, description, or descriptive metadata fields. When autonomous skills (such as `rewrite-story`) produce an improved specification or when agents refine task definitions, they cannot programmatically update the task via MCP. Agents are forced to either leave the task description stale or resort to direct unauthenticated or out-of-band HTTP calls to the local web server, bypassing the caller attribution, session ownership, and consistency guarantees that the MCP server provides.

## What Changes
Add `update_task` to the canonical MCP catalog:
- Accept `taskKey` as a required parameter (matching by full task ID or task key).
- Accept optional mutable descriptive fields: `title`, `description`, `priority`, `issueType`, and `labels`.
- Enforce validation: require at least one mutable field in each call. Reject empty or blank `title`. Allow `description` to be updated or explicitly cleared via an empty string.
- Maintain workflow invariants: disallow changing workflow stage or status via `update_task` (status transitions remain strictly reserved for `transition_stage`).
- Protect workflow labels: when custom `labels` are provided, filter out workflow stage labels (`#new`, `#clarified`, `#specified`, etc.) and preserve the task's existing workflow stage label so that task stage tracking is never corrupted or advanced.
- Delegate updates to `db.UpdateTaskBy` using the authenticated caller principal (`callerOf(resolve, req)`), automatically queueing asynchronous tracker synchronization (GitHub/Jira) under the acting user's identity.
- Expand the canonical MCP catalog from nine tools to ten tools: update `internal/agentmcp/mcp.go`, `internal/mcptest/contract.go`, and all catalog tests to recognize `update_task`.

## Capabilities
### Added Capabilities
- `mcp-task-update`: updating mutable descriptive fields of a task through the MCP interface with caller attribution and tracker synchronization.

### Modified Capabilities
- `sectile-mcp-naming`: the canonical catalog becomes ten tools (`get_task`, `transition_stage`, `add_comment`, `list_tasks`, `get_project_context`, `list_projects`, `start_run`, `finish_run`, `create_task`, and `update_task`).

## Impact
- `internal/taskmcp/server.go`: new `update_task` tool registration, argument schema, validation, label sanitization, and execution.
- `internal/agentmcp/mcp.go`: stdio proxy whitelist and expected tool count extended from nine to ten.
- `internal/mcptest/contract.go`: contract test suite updated to test `update_task` and assert a ten-tool catalog.
- `internal/agent/agent_config_test.go` and `internal/handlers/mcp_test.go`: tool count assertions updated from nine to ten.
- `internal/taskmcp/server_test.go`: unit test suite covering `update_task` operations, field validation, workflow label preservation, and error cases.
- Documentation: `README.md`, `docs/ARCHITECTURE.md`, `docs/contracts/server-agent-v1.md`.
