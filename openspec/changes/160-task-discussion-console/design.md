# Design

## A reserved skill id rather than a new endpoint
`discuss` joins `custom` as a reserved skill id handled inside `dispatchCommand` (`cmd/agent/agent_config.go`) before the configured-skill lookup. The whole existing chain then works unchanged: `runSkill` in the web UI, `POST /api/tasks/{id}/run-skill`, the activity and remote run records, `agentDispatcher.DispatchAndWait`, `prepareDispatch` (worktree and branch), the `SECTILE_*` environment, the PTY session and the stop/replay controls.

Rejected alternatives:
- A dedicated `POST /api/tasks/{id}/discuss` endpoint. It would duplicate the dispatch, activity and run bookkeeping of `run-skill` for no behavioural gain.
- Reusing `action: "open_terminal"` with an empty skill id, which `dispatchCommand` already maps to `InteractiveAgentLaunch`. That path belongs to the external-terminal feature (`launchTaskExternalTerminal`), which opens a native host terminal window and is reached from a different UI control. Borrowing it would tie the discussion to the external-terminal configuration.
- A project-level free console scoped to a task. The free console is deliberately taskless (`openspec/specs/free-console`); reusing it would mean re-adding the task, worktree and branch resolution that the task dispatch already performs.

## Prompt-free launch
`dispatchCommand` returns `runner.InteractiveAgentLaunch(...)` for `discuss`, which resolves the provider binary with no arguments and no permission-bypass flag — a human is watching the session and answers the prompts. Any `prompt` carried by the dispatch is ignored, including the `runId` sentence the agent appends to skill prompts, because a discussion has no skill contract to fulfil.

This matches the free console's "Prompt-free launch" requirement. The task is still reachable: the session runs in the task worktree with `SECTILE_TASK_ID`, `SECTILE_TASK_KEY`, `SECTILE_TASK_BRANCH`, `SECTILE_TASK_WORKTREE` and `SECTILE_PROJECT_ID` set and Sectile MCP registered, so the user can ask the agent to read the task. Injecting a task summary as an opening prompt was rejected: it makes the agent start talking before the user has said anything, which is the behaviour the ticket asks to avoid.

## No workflow effect
A discussion performs no stage transition, writes no skill result and posts nothing to the tracker. It still creates the usual activity and remote run so the console is listed, selectable and stoppable; that run ends when the PTY exits, through the existing `finishDesktopRun` path.

## Admission
`cmd/agent/agent_desktop.go` validates a launch request against the project skills plus the reserved `custom` id, which requires a non-empty prompt. `discuss` is admitted with no prompt, since a discussion has nothing to carry.

## Open points
None.
