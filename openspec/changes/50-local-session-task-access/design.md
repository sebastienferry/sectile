# Design

## Context
`internal/taskmcp/server.go` composes `get_task` from `db.GetTaskByID` (local SQLite) and `db.GetTaskComments` (`internal/db/comments.go`), which reads the tracker for tracker-backed tasks. A single `err` return collapses the two, so a tracker failure hides a task that was already read successfully. `get_project_context` forwards `db.AgentConfig` unchanged; that structure carries every skill's `content` and `commandContent`, which the project-context file writer needs but a skill session does not.

## Decisions

### Degrade the task read, do not fail it
`get_task` returns `{task, comments, commentsError?}`. Comments stay absent on failure and the reason is stated, so a session can never mistake a fetch failure for an empty discussion. Rejected: retrying the tracker inside the tool (the credential is missing, not flaky) and silently returning an empty list (indistinguishable from a ticket with no comments, and the ticket asks for an actionable error).

### Shrink the context at the MCP boundary, not in `AgentConfig`
`AgentConfig` keeps its full shape: `internal/agentconfig` writes `.taskflow/config.json` and the `AGENTS.md` block from it, and the agent ships skill bodies to disk from the same structure. Only the MCP tool projects it down. Rejected: making the payload opt-in through a parameter — the default is what sessions actually call, and a default that overflows the budget is the defect.

### Keep skill references resolvable
Each entry keeps `id`, `directory` and `command`, and the payload carries the directories the agent writes skill files into. The server is headless and does not know the checkout, so it names the location convention rather than absolute paths; a session needing a skill body opens the file rather than receiving ten of them inline.

## Risks
A consumer reading `skills[].content` from `get_project_context` loses it. The only known consumers are skill sessions, which receive their own skill text in the invocation; the file writer uses `AgentConfig` directly, not the MCP tool.
